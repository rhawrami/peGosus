package plan

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestSpilledGroupAndDistinctLanes(t *testing.T) {
	for _, shape := range []string{"group", "global", "distinct", "hot", "float_order", "empty", "empty_global", "null"} {
		t.Run(shape, func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			batches := make([]*store.Batch, 5)
			for chunk := range batches {
				n := 257
				if shape == "empty" || shape == "empty_global" {
					n = 0
				}
				keys := store.MakeVector(a, n, dtype.Float64T(), true)
				nums := store.MakeVector(a, n, dtype.Float64T(), true)
				ints := store.MakeVector(a, n, dtype.Int64T(), true)
				texts := make([][]byte, n)
				flags := store.MakeVector(a, n, dtype.BoolT(), true)
				mask := store.MakeBitMap(a, n)
				for row := range n {
					id := chunk*n + row
					key := id % 173
					if shape == "hot" || shape == "float_order" {
						key = 0
					}
					keys.F64s()[row] = float64(key)
					if id%17 == 0 {
						keys.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(id%255))
					}
					if key == 0 && id%2 == 0 {
						keys.F64s()[row] = math.Copysign(0, -1)
					}
					nums.F64s()[row] = float64(id%139 - 67)
					if shape == "float_order" {
						nums.F64s()[row] = []float64{1e20, -1e20, 3, 1e20, -1e20, 2, 1, -1}[id%8]
					}
					ints.I64s()[row] = int64(id%191 - 95)
					texts[row] = []byte(fmt.Sprintf("shared\x00 long binary payload %04d", id%283))
					flags.Bools()[row/8] |= byte(id%2) << uint(row%8)
					if id%19 == 0 || shape == "null" {
						nums.Validity().Clear(row)
						ints.Validity().Clear(row)
						flags.Validity().Clear(row)
					}
					if id%23 == 0 || shape == "null" {
						keys.Validity().Clear(row)
					}
					if id%7 != 0 {
						mask.Set(row)
					}
				}
				batches[chunk] = store.MakeBatch([]store.Vector{keys, nums, ints, store.MakeStringVector(a, texts, nil), flags})
				batches[chunk].SetSelection(store.MakeRowSelectionFromBitMap(mask))
			}
			table := store.MakeTable(batches)
			for _, b := range batches {
				b.Release()
			}
			schema := MakeSchemaWithNullability([]string{"g", "f", "x", "s", "b"}, []dtype.Type{dtype.Float64T(), dtype.Float64T(), dtype.Int64T(), dtype.StringT(), dtype.BoolT()}, []bool{true, true, true, false, true})
			aggregates := []Aggregate{MakeCountStar(), MakeCountDistinct(MakeColumn("x")), MakeSum(MakeColumn("x")).Distinct(), MakeAvg(MakeColumn("x")).Distinct(), MakeSum(MakeColumn("f")), MakeSum(MakeColumn("f")).Distinct(), MakeAvg(MakeColumn("f")).Distinct(), MakeMin(MakeColumn("f")).Distinct(), MakeMax(MakeColumn("f")).Distinct(), MakeCountDistinct(MakeColumn("s")), MakeMin(MakeColumn("s")), MakeMax(MakeColumn("s")).Distinct(), MakeCountDistinct(MakeColumn("b"))}
			logical := MakeScan(table, schema).GroupBy([]Expr{MakeColumn("g")}, aggregates...)
			if shape == "global" || shape == "empty_global" || shape == "float_order" {
				logical = MakeScan(table, schema).Aggregate(aggregates...)
			}
			if shape == "distinct" {
				logical = MakeScan(table, schema).Project(MakeColumn("g"), MakeColumn("x"), MakeColumn("b")).Distinct()
			}
			p, err := MakePhysicalPlan(logical)
			if err != nil {
				t.Fatal(err)
			}
			expected := map[string][]Scalar{}
			keyCols := 1
			if shape == "global" || shape == "empty_global" || shape == "float_order" {
				keyCols = 0
			}
			if shape == "distinct" {
				keyCols = 3
			}
			take := func(b *store.Batch, target map[string][]Scalar) bool {
				for row := range b.Len() {
					key := ""
					values := make([]Scalar, b.NVectors())
					for col := range values {
						values[col] = scalarAt(b.VectorAt(col), row)
						if values[col].Type().ID() == dtype.STRT {
							values[col].text = string([]byte(values[col].text))
						}
						if col < keyCols {
							value := values[col]
							name := fmt.Sprint(value.bits, value.null)
							if value.Type().IsFloating() {
								if value.null {
									name = "null"
								} else if math.IsNaN(value.f64()) {
									name = "nan"
								} else if value.f64() == 0 {
									name = "zero"
								}
							}
							key += name + "/"
						}
					}
					if _, exists := target[key]; exists {
						t.Fatalf("duplicate group %q", key)
					}
					target[key] = values
				}
				return true
			}
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 1}, func(b *store.Batch) bool { return take(b, expected) })
			if result.Code() != ExecutionCompleted {
				t.Fatal("reference", result.Code(), result.Err())
			}
			baseline := a.Usage()
			directory := t.TempDir()
			actual := map[string][]Scalar{}
			options := ExecutionOptions{MemoryBudget: 192 << 10, Workers: 1, SpillDirectory: directory}
			result, handled := p.executeSpilledGroup(context.Background(), a, options, func(b *store.Batch) bool { return take(b, actual) }, nil)
			if !handled || result.Code() != ExecutionCompleted {
				t.Fatal("spilled", handled, result.Code(), result.Err())
			}
			if len(actual) != len(expected) {
				t.Fatalf("group count got=%d want=%d", len(actual), len(expected))
			}
			for key, want := range expected {
				got, exists := actual[key]
				if !exists {
					t.Fatalf("missing %q", key)
				}
				for col := range want {
					if got[col] != want[col] {
						t.Fatalf("group %s col=%d got=%+v want=%+v", key, col, got[col], want[col])
					}
				}
			}
			if entries, err := os.ReadDir(directory); err != nil || len(entries) != 0 {
				t.Fatal("spill files leaked", entries, err)
			}
			if usage := a.Usage(); usage.GeneralUsed != baseline.GeneralUsed || usage.ScratchUsed != baseline.ScratchUsed {
				t.Fatal("payload leaked", usage, baseline)
			}
			p.Release()
			table.Release()
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatal("source leaked", usage)
			}
		})
	}
}

func TestSpilledGroupBudgetTailsCancellationAndRetainedOutput(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	const rows = 1025
	key := store.MakeVector(a, rows, dtype.Int64T(), false)
	texts := make([][]byte, rows)
	for row := range rows {
		key.I64s()[row] = int64(row % 513)
		texts[row] = []byte(fmt.Sprintf("long unique binary\x00 payload %05d", row))
	}
	batch := store.MakeBatch([]store.Vector{key, store.MakeStringVector(a, texts, nil)})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchema([]string{"g", "s"}, []dtype.Type{dtype.Int64T(), dtype.StringT()})
	base := MakeScan(table, schema).GroupBy([]Expr{MakeColumn("g").Alias("g")}, MakeCountDistinct(MakeColumn("s")).Alias("n"), MakeMax(MakeColumn("s")).Alias("last"))
	p, err := MakePhysicalPlan(base.OrderBy(MakeOrderKey(MakeColumn("n")).Desc(), MakeOrderKey(MakeColumn("g"))).Limit(7, 5))
	if err != nil {
		t.Fatal(err)
	}
	baseline := a.Usage()
	directory := t.TempDir()
	options := ExecutionOptions{MemoryBudget: 96 << 10, Workers: 1, SpillDirectory: directory}
	plain, err := MakePhysicalPlan(base)
	if err != nil {
		t.Fatal(err)
	}
	failed := plain.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 96 << 10, Workers: 1}, func(*store.Batch) bool { t.Error("in-memory budget exceeded but emitted"); return true })
	if failed.Code() != ExecutionResourceExhausted {
		t.Fatal("fixture does not exceed memory budget", failed.Code())
	}
	plain.Release()
	// The full group result exceeds the budget; sorted runs and lane merging remain bounded.
	got := 0
	result, handled := p.executeSpilledGroup(context.Background(), a, options, func(b *store.Batch) bool {
		for row := range b.Len() {
			if bitmap, ok := b.Selection().AsBitMap(); ok && !bitmap.IsSet(row) {
				continue
			}
			want := int64(got + 5)
			if b.VectorAt(0).I64s()[row] != want || b.VectorAt(1).I64s()[row] != 2 {
				t.Fatalf("sorted/offset group got=%d/%d want=%d/2", b.VectorAt(0).I64s()[row], b.VectorAt(1).I64s()[row], want)
			}
			got++
		}
		return true
	}, nil)
	if !handled || result.Code() != ExecutionCompleted || got != 7 {
		t.Fatal("spilled tail", handled, result.Code(), result.Err(), got)
	}
	check := func() {
		t.Helper()
		if entries, err := os.ReadDir(directory); err != nil || len(entries) != 0 {
			t.Fatal("temporary files leaked", entries, err)
		}
		if usage := a.Usage(); usage.GeneralUsed != baseline.GeneralUsed || usage.ScratchUsed != baseline.ScratchUsed {
			t.Fatal("temporary payload leaked", usage, baseline)
		}
	}
	check()
	calls := 0
	result, _ = p.executeSpilledGroup(context.Background(), a, options, func(*store.Batch) bool { calls++; return false }, nil)
	if result.Code() != ExecutionStopped || calls != 1 {
		t.Fatal("sink stop", result.Code(), calls)
	}
	check()
	ctx, cancel := context.WithCancel(context.Background())
	controlled := &parallelSortCancelContext{Context: ctx, cancel: cancel}
	result, _ = p.executeSpilledGroup(controlled, a, options, func(*store.Batch) bool { t.Error("cancelled query reached sink"); return true }, nil)
	if result.Code() != ExecutionCancelled {
		t.Fatal("mid-execution cancellation", result.Code(), result.Err())
	}
	cancel()
	check()
	ctx, cancel = context.WithCancel(context.Background())
	result, _ = p.executeSpilledGroup(ctx, a, options, func(*store.Batch) bool { cancel(); return true }, nil)
	if result.Code() != ExecutionCancelled {
		t.Fatal("sink cancellation", result.Code(), result.Err())
	}
	cancel()
	check()
	small := options
	small.MemoryBudget = 128
	result, _ = p.executeSpilledGroup(context.Background(), a, small, func(*store.Batch) bool { t.Error("exhausted spill query reached sink"); return true }, nil)
	if result.Code() != ExecutionResourceExhausted {
		t.Fatal("tiny spill budget", result.Code(), result.Err())
	}
	check()
	blocker := directory + "/file"
	if err := os.WriteFile(blocker, []byte("block"), 0600); err != nil {
		t.Fatal(err)
	}
	invalid := options
	invalid.SpillDirectory = blocker
	result, _ = p.executeSpilledGroup(context.Background(), a, invalid, func(*store.Batch) bool { t.Error("invalid spill path reached sink"); return true }, nil)
	if result.Code() != ExecutionFailed {
		t.Fatal("spill IO failure", result.Code(), result.Err())
	}
	os.Remove(blocker)
	check()
	outputs := []*store.Batch{}
	result, _ = p.executeSpilledGroup(context.Background(), a, options, func(b *store.Batch) bool { outputs = append(outputs, b.Retain()); return true }, nil)
	if result.Code() != ExecutionCompleted {
		t.Fatal(result.Code(), result.Err())
	}
	p.Release()
	table.Release()
	count := 0
	for _, output := range outputs {
		for row := range output.Len() {
			if bitmap, ok := output.Selection().AsBitMap(); ok && !bitmap.IsSet(row) {
				continue
			}
			if output.VectorAt(2).StringAt(row).Len() == 0 {
				t.Fatal("retained string lost after source release")
			}
			count++
		}
		output.Release()
	}
	if count != 7 {
		t.Fatal("retained count", count)
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatal("retained result leaked", usage)
	}
}
