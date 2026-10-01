package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParallelCompactGroupedCount(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 8)
	want := make(map[string][2]int64)
	for chunk := range batches {
		key := store.MakeVector(a, 257, dtype.Float64T(), true)
		value := store.MakeVector(a, 257, dtype.Int32T(), true)
		selection := store.MakeBitMap(a, 257)
		for row := range 257 {
			id := chunk*257 + row
			name := "0"
			switch {
			case id%19 == 0:
				key.Validity().Clear(row)
				name = "null"
			case id%31 == 0:
				key.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(id&15))
				name = "NaN"
			case id%3 == 0:
				key.F64s()[row] = math.Copysign(0, -1)
			default:
				key.F64s()[row] = float64(id%13 - 6)
				name = strconv.FormatFloat(key.F64s()[row], 'g', -1, 64)
			}
			if id%11 == 0 {
				value.Validity().Clear(row)
			}
			if id%7 == 0 {
				continue
			}
			selection.Set(row)
			counts := want[name]
			counts[0]++
			if id%11 != 0 {
				counts[1]++
			}
			want[name] = counts
		}
		batch := store.MakeBatch([]store.Vector{key, value})
		batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
		batches[chunk] = batch
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	schema := MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{dtype.Float64T(), dtype.Int32T()}, []bool{true, true})
	p, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeCount(MakeColumn("value"))))
	if err != nil {
		t.Fatal(err)
	}
	if p.steps[0].operation != physicalAggregate || makeCompactGroupState(p.steps[0]) == nil {
		t.Fatal("COUNT grouping did not select compact path")
	}
	for _, workers := range []int{1, 4} {
		seen := make(map[string]bool)
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: workers}, func(batch *store.Batch) bool {
			for row := range batch.Len() {
				name := "null"
				key := batch.VectorAt(0)
				if key.Validity() == nil || key.Validity().IsSet(row) {
					v := key.F64s()[row]
					if math.IsNaN(v) {
						name = "NaN"
					} else if v == 0 {
						name = "0"
					} else {
						name = strconv.FormatFloat(v, 'g', -1, 64)
					}
				}
				counts, ok := want[name]
				if !ok || seen[name] || batch.VectorAt(1).I64s()[row] != counts[0] || batch.VectorAt(2).I64s()[row] != counts[1] {
					t.Errorf("workers=%d incorrect group %q", workers, name)
				}
				seen[name] = true
			}
			return true
		})
		if result.Code() != ExecutionCompleted || len(seen) != len(want) {
			t.Fatalf("workers=%d result %v/%v groups %d/%d", workers, result.Code(), result.Err(), len(seen), len(want))
		}
	}
	p.Release()
	table.Release()
}

func TestParallelGeneralGroupedAggregation(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	type result struct {
		count, nonNull, sum int64
		min, max            string
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	rng := rand.New(rand.NewPCG(97, 113))
	want := make(map[string]result)
	batches := make([]*store.Batch, 12)
	for chunk := range batches {
		key := store.MakeVector(a, 129, dtype.Int32T(), true)
		floatKey := store.MakeVector(a, 129, dtype.Float64T(), true)
		payload := store.MakeVector(a, 129, dtype.Int32T(), true)
		texts := make([][]byte, 129)
		selection := store.MakeBitMap(a, 129)
		var offsets []uint32
		for row := range 129 {
			id := chunk*129 + row
			key.I32s()[row] = int32(rng.IntN(29) - 14)
			if id%17 == 0 {
				key.Validity().Clear(row)
			}
			if id%19 == 0 {
				floatKey.Validity().Clear(row)
			}
			switch {
			case id%23 == 0:
				floatKey.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(id&15))
			case id%3 == 0:
				floatKey.F64s()[row] = math.Copysign(0, -1)
			default:
				floatKey.F64s()[row] = float64(rng.IntN(5) - 2)
			}
			payload.I32s()[row] = int32(id%63 - 31)
			if id%11 == 0 {
				payload.Validity().Clear(row)
			}
			texts[row] = []byte(fmt.Sprintf("long retained payload value %06d", id))
			if id%5 == 2 {
				continue
			}
			selection.Set(row)
			offsets = append(offsets, uint32(row))
			name := "null"
			if key.Validity().IsSet(row) {
				name = strconv.FormatInt(int64(key.I32s()[row]), 10)
			}
			name += "/null"
			if floatKey.Validity().IsSet(row) {
				v := floatKey.F64s()[row]
				if math.IsNaN(v) {
					name = name[:len(name)-4] + "NaN"
				} else if v == 0 {
					name = name[:len(name)-4] + "0"
				} else {
					name = name[:len(name)-4] + strconv.FormatFloat(v, 'g', -1, 64)
				}
			}
			state := want[name]
			state.count++
			if payload.Validity().IsSet(row) {
				state.nonNull++
				state.sum += int64(payload.I32s()[row])
			}
			text := string(texts[row])
			if state.min == "" || text < state.min {
				state.min = text
			}
			if text > state.max {
				state.max = text
			}
			want[name] = state
		}
		text := store.MakeStringVector(a, texts, nil)
		batch := store.MakeBatch([]store.Vector{key, floatKey, payload, text})
		if chunk%2 == 0 {
			batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
		} else {
			selection.Release()
			batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 129, offsets)))
		}
		batches[chunk] = batch
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	schema := MakeSchemaWithNullability([]string{"key", "floating", "payload", "text"}, []dtype.Type{dtype.Int32T(), dtype.Float64T(), dtype.Int32T(), dtype.StringT()}, []bool{true, true, true, false})
	p, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy([]Expr{MakeColumn("key"), MakeColumn("floating")}, MakeCountStar(), MakeCount(MakeColumn("payload")), MakeSum(MakeColumn("payload")), MakeMin(MakeColumn("text")), MakeMax(MakeColumn("text"))))
	if err != nil {
		t.Fatal(err)
	}
	if makeCompactGroupState(p.steps[0]) != nil {
		t.Fatal("unsupported grouping selected compact path")
	}
	for _, workers := range []int{1, 4} {
		seen := make(map[string]bool)
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: workers}, func(batch *store.Batch) bool {
			for row := range batch.Len() {
				name := "null"
				if v := batch.VectorAt(0); v.Validity() == nil || v.Validity().IsSet(row) {
					name = strconv.FormatInt(int64(v.I32s()[row]), 10)
				}
				name += "/null"
				if v := batch.VectorAt(1); v.Validity() == nil || v.Validity().IsSet(row) {
					f := v.F64s()[row]
					if math.IsNaN(f) {
						name = name[:len(name)-4] + "NaN"
					} else if f == 0 {
						name = name[:len(name)-4] + "0"
					} else {
						name = name[:len(name)-4] + strconv.FormatFloat(f, 'g', -1, 64)
					}
				}
				state, ok := want[name]
				sumVector := batch.VectorAt(4)
				sumValid := sumVector.Validity() == nil || sumVector.Validity().IsSet(row)
				if !ok || seen[name] || batch.VectorAt(2).I64s()[row] != state.count || batch.VectorAt(3).I64s()[row] != state.nonNull || sumValid != (state.nonNull != 0) || sumValid && sumVector.I64s()[row] != state.sum || batch.VectorAt(5).Strings()[row].View() != state.min || batch.VectorAt(6).Strings()[row].View() != state.max {
					t.Errorf("workers=%d incorrect group %q found=%t duplicate=%t got count=%d nonNull=%d sum=%d min=%q max=%q want=%+v", workers, name, ok, seen[name], batch.VectorAt(2).I64s()[row], batch.VectorAt(3).I64s()[row], batch.VectorAt(4).I64s()[row], batch.VectorAt(5).Strings()[row].View(), batch.VectorAt(6).Strings()[row].View(), state)
				}
				seen[name] = true
			}
			return true
		})
		if result.Code() != ExecutionCompleted || len(seen) != len(want) {
			t.Fatalf("workers=%d result %v/%v groups %d/%d", workers, result.Code(), result.Err(), len(seen), len(want))
		}
	}
	p.Release()
	table.Release()
}

func TestParallelGroupedStringKeyOwnership(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 8)
	want := make(map[string]int64)
	for chunk := range batches {
		values := make([][]byte, 257)
		valid := make([]bool, 257)
		for row := range values {
			id := chunk*257 + row
			if id%17 == 0 {
				want["<null>"]++
				continue
			}
			name := fmt.Sprintf("long shared German string key %03d", id%19)
			values[row] = []byte(name)
			valid[row] = true
			want[name]++
		}
		key := store.MakeStringVector(a, values, valid)
		batches[chunk] = store.MakeBatch([]store.Vector{key})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	schema := MakeSchemaWithNullability([]string{"key"}, []dtype.Type{dtype.StringT()}, []bool{true})
	p, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	var retained *store.Batch
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(batch *store.Batch) bool { retained = batch.Retain(); return true })
	p.Release()
	table.Release()
	if !parallel || result.Code() != ExecutionCompleted || retained == nil {
		t.Fatalf("string grouping %t %v/%v", parallel, result.Code(), result.Err())
	}
	defer retained.Release()
	if retained.Len() != len(want) {
		t.Fatalf("got %d groups, want %d", retained.Len(), len(want))
	}
	for row := range retained.Len() {
		name := "<null>"
		if retained.VectorAt(0).Validity().IsSet(row) {
			name = retained.VectorAt(0).Strings()[row].View()
		}
		if got := retained.VectorAt(1).I64s()[row]; got != want[name] {
			t.Fatalf("string key %q count %d, want %d", name, got, want[name])
		}
	}
}

func TestParallelGroupedResourceStopAndCancellation(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 8)
	for chunk := range batches {
		values := make([][]byte, 1024)
		for row := range values {
			values[row] = []byte(fmt.Sprintf("unique long group key for budget test %010d", chunk*1024+row))
		}
		key := store.MakeStringVector(a, values, nil)
		batches[chunk] = store.MakeBatch([]store.Vector{key})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	defer table.Release()
	schema := MakeSchemaWithNullability([]string{"key"}, []dtype.Type{dtype.StringT()}, []bool{false})
	p, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	if _, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20}, func(*store.Batch) bool { t.Fatal("general grouping unexpectedly used automatic workers"); return true }); parallel {
		t.Fatal("general grouping should use the measured serial auto path")
	}
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 2 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("over-budget grouping reached sink"); return true })
	if !parallel || result.Code() != ExecutionResourceExhausted {
		t.Fatalf("grouped resource result %t %v/%v", parallel, result.Code(), result.Err())
	}
	calls := 0
	result = p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(*store.Batch) bool { calls++; return false })
	if result.Code() != ExecutionStopped || calls != 1 {
		t.Fatalf("grouped early stop %v/%v, calls %d", result.Code(), result.Err(), calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = p.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("cancelled grouped query reached sink"); return true })
	if result.Code() != ExecutionCancelled {
		t.Fatalf("grouped cancellation %v/%v", result.Code(), result.Err())
	}
}

func TestParallelGeneralGroupAutoCardinality(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	for _, cardinality := range []int{16, 16384} {
		t.Run(fmt.Sprintf("groups=%d", cardinality), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			batches := make([]*store.Batch, 16)
			for chunk := range batches {
				key := store.MakeVector(a, 1024, dtype.Int32T(), false)
				stringsIn := make([][]byte, 1024)
				for row := range 1024 {
					value := (chunk*1024 + row) % cardinality
					key.I32s()[row] = int32(value)
					stringsIn[row] = []byte(fmt.Sprintf("long group sample key %06d", value))
				}
				text := store.MakeStringVector(a, stringsIn, nil)
				batches[chunk] = store.MakeBatch([]store.Vector{key, text})
			}
			table := store.MakeTable(batches)
			for _, batch := range batches {
				batch.Release()
			}
			defer table.Release()
			schema := MakeSchemaWithNullability([]string{"key", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{false, false})
			p, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy([]Expr{MakeColumn("key"), MakeColumn("text")}, MakeCountStar()))
			if err != nil {
				t.Fatal(err)
			}
			defer p.Release()
			count := int64(0)
			result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20}, func(batch *store.Batch) bool {
				for _, n := range batch.VectorAt(2).I64s() {
					count += n
				}
				return true
			})
			if cardinality == 16 {
				if !parallel || result.Code() != ExecutionCompleted || count != 16384 {
					t.Fatalf("low-cardinality auto %t %v/%v, %d rows", parallel, result.Code(), result.Err(), count)
				}
			} else if parallel {
				t.Fatalf("high-cardinality sample selected workers: %v", result.Code())
			}
		})
	}
}

func TestParallelShardedHighCardinalityGrouping(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 8)
	for chunk := range batches {
		key := store.MakeVector(a, 4096, dtype.Int32T(), false)
		stringsIn := make([][]byte, 4096)
		for row := range 4096 {
			id := chunk*4096 + row
			key.I32s()[row] = int32(id)
			stringsIn[row] = []byte(fmt.Sprintf("long shared high cardinality key %05d", id))
		}
		text := store.MakeStringVector(a, stringsIn, nil)
		batches[chunk] = store.MakeBatch([]store.Vector{key, text})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	schema := MakeSchemaWithNullability([]string{"key", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{false, false})
	p, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy([]Expr{MakeColumn("key"), MakeColumn("text")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	var retained []*store.Batch
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 512 << 20, Workers: min(4, runtime.GOMAXPROCS(0))}, func(batch *store.Batch) bool {
		retained = append(retained, batch.Retain())
		return true
	})
	p.Release()
	table.Release()
	if !parallel || result.Code() != ExecutionCompleted || len(retained) < 2 {
		t.Fatalf("partitioned merge %t %v/%v, batches %d", parallel, result.Code(), result.Err(), len(retained))
	}
	seen := make(map[int32]bool)
	for _, batch := range retained {
		for row, key := range batch.VectorAt(0).I32s() {
			if seen[key] || batch.VectorAt(1).Strings()[row].View() != fmt.Sprintf("long shared high cardinality key %05d", key) || batch.VectorAt(2).I64s()[row] != 1 {
				t.Errorf("incorrect merged group %d", key)
			}
			seen[key] = true
		}
		batch.Release()
	}
	if len(seen) != 32768 {
		t.Fatalf("got %d groups", len(seen))
	}
}

func TestParallelGroupedAllFilteredOut(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	batches := make([]*store.Batch, 4)
	for chunk := range batches {
		key := store.MakeVector(a, 257, dtype.Int32T(), false)
		for row := range 257 {
			key.I32s()[row] = int32(chunk*257 + row)
		}
		batches[chunk] = store.MakeBatch([]store.Vector{key})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	defer table.Release()
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key"}, []dtype.Type{dtype.Int32T()}, []bool{false})).Filter(MakeColumn("key").Lt(0)).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	called := false
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 8 << 20, Workers: 4}, func(*store.Batch) bool { called = true; return true })
	if !parallel || result.Code() != ExecutionCompleted || called {
		t.Fatalf("empty grouped output %t %v/%v sink called=%t", parallel, result.Code(), result.Err(), called)
	}
}
