package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParallelDistinctManyAndGrouped(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	for _, groups := range []int{0, 7, 10000} {
		t.Run(fmt.Sprint(groups), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			batches := make([]*store.Batch, 8)
			rng := rand.New(rand.NewPCG(27, 98))
			for chunk := range batches {
				const rows = 2049
				key := store.MakeVector(a, rows, dtype.Int64T(), true)
				value := store.MakeVector(a, rows, dtype.Int64T(), true)
				floats := store.MakeVector(a, rows, dtype.Float64T(), true)
				texts := make([][]byte, rows)
				offsets := []uint32{}
				for row := range rows {
					id := chunk*rows + row
					key.I64s()[row] = int64(id % max(1, groups))
					value.I64s()[row] = int64(rng.IntN(103) - 51)
					floats.F64s()[row] = float64(value.I64s()[row])
					if id%29 == 0 {
						floats.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(id%256))
					}
					if id%23 == 0 {
						floats.F64s()[row] = math.Copysign(0, -1)
					}
					if id%31 == 0 {
						floats.F64s()[row] = math.Inf(1)
					}
					if id%17 == 0 {
						value.Validity().Clear(row)
						floats.Validity().Clear(row)
					}
					if id%19 == 0 {
						key.Validity().Clear(row)
					}
					texts[row] = []byte(fmt.Sprintf("binary\x00 long distinct value %03d", id%137))
					if id%11 != 0 {
						offsets = append(offsets, uint32(row))
					}
				}
				batches[chunk] = store.MakeBatch([]store.Vector{key, value, floats, store.MakeStringVector(a, texts, nil)})
				batches[chunk].SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, rows, offsets)))
			}
			table := store.MakeTable(batches)
			for _, batch := range batches {
				batch.Release()
			}
			schema := MakeSchemaWithNullability([]string{"g", "x", "f", "s"}, []dtype.Type{dtype.Int64T(), dtype.Int64T(), dtype.Float64T(), dtype.StringT()}, []bool{true, true, true, false})
			aggregates := []Aggregate{MakeCountDistinct(MakeColumn("x")), MakeSum(MakeColumn("x")).Distinct(), MakeAvg(MakeColumn("x")).Distinct(), MakeMin(MakeColumn("x")).Distinct(), MakeMax(MakeColumn("x")).Distinct(), MakeCountDistinct(MakeColumn("x")), MakeCountDistinct(MakeColumn("f")), MakeMin(MakeColumn("f")).Distinct(), MakeMax(MakeColumn("f")).Distinct(), MakeCountDistinct(MakeColumn("s")), MakeMin(MakeColumn("s")).Distinct(), MakeMax(MakeColumn("s")).Distinct(), MakeCountStar(), MakeSum(MakeColumn("x")), MakeCountDistinct(MakeColumn("x").Add(MakeLiteral(MakeI64Scalar(1))))}
			if groups == 10000 {
				aggregates = []Aggregate{MakeCountDistinct(MakeColumn("x")), MakeSum(MakeColumn("x")).Distinct(), MakeCountStar()}
			}
			logical := MakeScan(table, schema).Aggregate(aggregates...)
			if groups != 0 {
				logical = MakeScan(table, schema).GroupBy([]Expr{MakeColumn("g")}, aggregates...)
			}
			p, err := MakePhysicalPlan(logical)
			if err != nil {
				t.Fatal(err)
			}
			originalSchema := append([]Field(nil), p.steps[len(p.steps)-1].schema.fields...)
			expected := map[string][]Scalar{}
			take := func(batch *store.Batch, target map[string][]Scalar) bool {
				for row := range batch.Len() {
					key := "global"
					start := 0
					if groups != 0 {
						v := scalarAt(batch.VectorAt(0), row)
						key = fmt.Sprint(v.bits, v.null)
						start = 1
					}
					values := make([]Scalar, batch.NVectors()-start)
					for col := range values {
						values[col] = scalarAt(batch.VectorAt(col+start), row)
						if values[col].Type().ID() == dtype.STRT {
							values[col].text = string([]byte(values[col].text))
						}
					}
					if _, exists := target[key]; exists {
						t.Fatalf("duplicate group %q", key)
					}
					target[key] = values
				}
				return true
			}
			if result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 512 << 20, Workers: 1}, func(batch *store.Batch) bool { return take(batch, expected) }); result.Code() != ExecutionCompleted {
				t.Fatal(result.Code(), result.Err())
			}
			actual := map[string][]Scalar{}
			result, parallel := p.executeParallelDistinct(context.Background(), a, ExecutionOptions{MemoryBudget: 512 << 20, Workers: 4}, func(batch *store.Batch) bool { return take(batch, actual) })
			if !parallel || result.Code() != ExecutionCompleted {
				t.Fatalf("parallel=%t outcome=%v error=%v", parallel, result.Code(), result.Err())
			}
			if len(actual) != len(expected) {
				t.Fatalf("groups got=%d want=%d", len(actual), len(expected))
			}
			for key, want := range expected {
				got, exists := actual[key]
				if !exists {
					t.Fatalf("missing group %q", key)
				}
				for i := range want {
					if got[i].dType != want[i].dType || got[i].null != want[i].null || got[i].text != want[i].text || got[i].bits != want[i].bits {
						t.Fatalf("group=%s aggregate=%d got=%+v want=%+v", key, i, got[i], want[i])
					}
				}
			}
			for i, field := range originalSchema {
				if p.steps[len(p.steps)-1].schema.FieldAt(i) != field {
					t.Fatal("mutated field")
				}
			}
			state := makeGroupState(p.steps[len(p.steps)-1], nil)
			if state.uniqueSources[1] != 0 || groups != 10000 && (state.uniqueSources[5] != 0 || state.uniqueSources[10] != 9) {
				t.Fatal("identical expressions not sharing distinct sets", state.uniqueSources)
			}
			state.release()
			p.Release()
			table.Release()
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatal("leaked payload", usage)
			}
		})
	}
}

func TestDistinctManyZeroNullEmptyOwnershipAndTermination(t *testing.T) {
	for _, input := range []string{"zeros", "nulls", "empty"} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
		batches := make([]*store.Batch, 4)
		for chunk := range batches {
			n := 2049
			if input == "empty" {
				n = 0
			}
			v := store.MakeVector(a, n, dtype.Float64T(), true)
			for row := range n {
				if row%3 == 0 {
					v.F64s()[row] = math.Copysign(0, -1)
				} else if row%3 == 1 {
					v.F64s()[row] = 0
				} else {
					v.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(chunk+1))
				}
				if input == "nulls" {
					v.Validity().Clear(row)
				}
			}
			batches[chunk] = store.MakeBatch([]store.Vector{v})
		}
		table := store.MakeTable(batches)
		for _, batch := range batches {
			batch.Release()
		}
		p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Float64T()})).Aggregate(MakeCountDistinct(MakeColumn("x")), MakeMin(MakeColumn("x")).Distinct(), MakeMax(MakeColumn("x")).Distinct(), MakeSum(MakeColumn("x")).Distinct(), MakeAvg(MakeColumn("x")).Distinct(), MakeCountStar()))
		if err != nil {
			t.Fatal(err)
		}
		baseline := a.Usage()
		for _, workers := range []int{1, 4} {
			var output *store.Batch
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: workers}, func(batch *store.Batch) bool { output = batch.Retain(); return true })
			if result.Code() != ExecutionCompleted || output == nil || output.Len() != 1 {
				t.Fatal(input, workers, result.Code())
			}
			wantCount := int64(0)
			if input == "zeros" {
				wantCount = 2
			}
			if output.VectorAt(0).I64s()[0] != wantCount {
				t.Fatal("distinct NaN/zero count", input, workers)
			}
			for col := 1; col <= 4; col++ {
				value := scalarAt(output.VectorAt(col), 0)
				if input != "zeros" {
					if !value.IsNull() {
						t.Fatal("empty reduction must be null", input, col, value)
					}
					continue
				}
				want := uint64(0)
				if col == 1 {
					want = uint64(1) << 63
				}
				if value.IsNull() || value.bits != want {
					t.Fatalf("signed zero input=%s workers=%d col=%d got=%+v wantbits=%x", input, workers, col, value, want)
				}
			}
			output.Release()
			if usage := a.Usage(); usage.GeneralUsed != baseline.GeneralUsed || usage.ScratchUsed != baseline.ScratchUsed {
				t.Fatal("query leaked", usage, baseline)
			}
		}
		if input == "zeros" {
			calls := 0
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(*store.Batch) bool { calls++; return false })
			if result.Code() != ExecutionStopped || calls != 1 {
				t.Fatal("early sink", result.Code(), calls)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			result = p.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("cancelled query reached sink"); return true })
			if result.Code() != ExecutionCancelled {
				t.Fatal("cancel", result.Code())
			}
			result = p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32, Workers: 4}, func(*store.Batch) bool { t.Error("exhausted query reached sink"); return true })
			if result.Code() != ExecutionResourceExhausted {
				t.Fatal("budget", result.Code())
			}
			ctx, cancel = context.WithCancel(context.Background())
			result = p.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(*store.Batch) bool { cancel(); return true })
			if result.Code() != ExecutionCancelled {
				t.Fatal("sink cancellation", result.Code())
			}
		}
		var retained *store.Batch
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(batch *store.Batch) bool { retained = batch.Retain(); return true })
		if result.Code() != ExecutionCompleted {
			t.Fatal(result.Code())
		}
		p.Release()
		table.Release()
		if retained.Len() != 1 {
			t.Fatal("retained output")
		}
		retained.Release()
		if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
			t.Fatal("leaked", usage)
		}
	}
}
