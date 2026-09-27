package plan

import (
	"context"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalGlobalAggregation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	var batches []*store.Batch
	for _, numbers := range [][]float64{{math.NaN(), 4, -0.0}, {math.Copysign(0, -1), 9, math.NaN()}, {100}} {
		v := store.MakeVector(a, len(numbers), dtype.Float64T(), true)
		copy(v.F64s(), numbers)
		if len(numbers) == 1 {
			v.Validity().Clear(0)
		}
		b := store.MakeBatch([]store.Vector{v})
		if len(numbers) == 3 && numbers[0] == 0 {
			b.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 3, []uint32{0, 2})))
		}
		batches = append(batches, b)
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Float64T()}))
	physical, err := MakePhysicalPlan(scan.Aggregate(
		MakeCountStar().Alias("rows"), MakeCount(MakeColumn("x")),
		MakeSum(MakeColumn("x")), MakeAvg(MakeColumn("x")),
		MakeMin(MakeColumn("x")), MakeMax(MakeColumn("x")),
	).Project(MakeColumn("rows"), MakeColumn("aggregate_2"), MakeColumn("aggregate_3"), MakeColumn("aggregate_4"), MakeColumn("aggregate_5"), MakeColumn("aggregate_6")))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	var output *store.Batch
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(b *store.Batch) bool {
		if output != nil {
			t.Fatal("global aggregation emitted multiple rows")
		}
		output = b.Retain()
		return true
	})
	physical.Release()
	if result.Code() != ExecutionCompleted || output == nil {
		t.Fatalf("aggregate failed: %v", result.Code())
	}
	defer output.Release()
	if output.Len() != 1 || output.VectorAt(0).I64s()[0] != 6 || output.VectorAt(1).I64s()[0] != 5 {
		t.Fatalf("incorrect counts: %d/%d", output.VectorAt(0).I64s()[0], output.VectorAt(1).I64s()[0])
	}
	if !math.IsNaN(output.VectorAt(2).F64s()[0]) || !math.IsNaN(output.VectorAt(3).F64s()[0]) {
		t.Fatal("NaN must propagate through SUM and AVG")
	}
	if min, max := output.VectorAt(4).F64s()[0], output.VectorAt(5).F64s()[0]; min != 0 || !math.Signbit(min) || max != 4 {
		t.Fatalf("min/max: got %v/%v", min, max)
	}
}

func TestPhysicalAggregateEmptyAndAllNull(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	for _, empty := range []bool{true, false} {
		var table *store.Table
		if empty {
			table = store.MakeEmptyTable([]dtype.Type{dtype.Int32T()})
		} else {
			v := store.MakeVector(a, 3, dtype.Int32T(), true)
			v.Validity().ClearAll()
			b := store.MakeBatch([]store.Vector{v})
			table = store.MakeTable([]*store.Batch{b})
			b.Release()
		}
		physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int32T()})).Aggregate(
			MakeCountStar(), MakeCount(MakeColumn("x")), MakeSum(MakeColumn("x")), MakeAvg(MakeColumn("x")),
		))
		if err != nil {
			t.Fatal(err)
		}
		table.Release()
		result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096}, func(output *store.Batch) bool {
			want := int64(3)
			if empty {
				want = 0
			}
			if output.Len() != 1 || output.VectorAt(0).I64s()[0] != want || output.VectorAt(1).I64s()[0] != 0 || output.VectorAt(2).Validity().IsSet(0) || output.VectorAt(3).Validity().IsSet(0) {
				t.Fatalf("empty=%t: incorrect global aggregate", empty)
			}
			return true
		})
		physical.Release()
		if result.Code() != ExecutionCompleted {
			t.Fatalf("empty=%t: result %v", empty, result.Code())
		}
	}
}

func TestPhysicalAggregateStringOwnershipAndIntegerSums(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	strings := store.MakeStringVector(a, [][]byte{[]byte("zebra long string value"), []byte("apple long string value"), []byte("middle long string value")}, nil)
	numbers := store.MakeVector(a, 3, dtype.Int32T(), true)
	copy(numbers.I32s(), []int32{1, 2, 3})
	numbers.Validity().Clear(2)
	b := store.MakeBatch([]store.Vector{strings, numbers})
	table := store.MakeTable([]*store.Batch{b})
	b.Release()
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"s", "n"}, []dtype.Type{dtype.StringT(), dtype.Int32T()})).Aggregate(
		MakeMin(MakeColumn("s")), MakeMax(MakeColumn("s")), MakeSum(MakeColumn("n")), MakeAvg(MakeColumn("n")),
	))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	var output *store.Batch
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(b *store.Batch) bool { output = b.Retain(); return true })
	physical.Release()
	if result.Code() != ExecutionCompleted || output == nil {
		t.Fatalf("aggregate failed: %v", result.Code())
	}
	defer output.Release()
	if output.VectorAt(0).Strings()[0].View() != "apple long string value" || output.VectorAt(1).Strings()[0].View() != "zebra long string value" || output.VectorAt(2).I64s()[0] != 3 || output.VectorAt(3).F64s()[0] != 1.5 {
		t.Fatal("incorrect string or numeric aggregate result")
	}
}

func TestPhysicalAggregateDistinctAcrossBatches(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	var batches []*store.Batch
	for _, values := range [][]float64{{0, math.NaN(), 2}, {math.Copysign(0, -1), math.Float64frombits(0x7ff8000000000001), 2}} {
		v := store.MakeVector(a, len(values), dtype.Float64T(), true)
		copy(v.F64s(), values)
		v.Validity().Clear(2)
		batches = append(batches, store.MakeBatch([]store.Vector{v}))
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Float64T()}))
	physical, err := MakePhysicalPlan(scan.Aggregate(MakeCount(MakeColumn("x")), MakeCountDistinct(MakeColumn("x"))))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(output *store.Batch) bool {
		if output.VectorAt(0).I64s()[0] != 4 || output.VectorAt(1).I64s()[0] != 2 {
			t.Fatalf("count distinct: got %d/%d", output.VectorAt(0).I64s()[0], output.VectorAt(1).I64s()[0])
		}
		return true
	})
	physical.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("aggregate DISTINCT result: %v", result.Code())
	}
}

func TestPhysicalFloatBoundsIgnoreNaNAndPreferSignedZero(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	v := store.MakeVector(a, 4, dtype.Float32T(), true)
	copy(v.F32s(), []float32{float32(math.NaN()), 0, float32(math.Copysign(0, -1)), float32(math.NaN())})
	b := store.MakeBatch([]store.Vector{v})
	table := store.MakeTable([]*store.Batch{b})
	b.Release()
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Float32T()}))
	physical, err := MakePhysicalPlan(scan.Aggregate(MakeMin(MakeColumn("x")), MakeMax(MakeColumn("x"))))
	if err != nil {
		t.Fatal(err)
	}
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096}, func(output *store.Batch) bool {
		if !math.Signbit(float64(output.VectorAt(0).F32s()[0])) || math.Signbit(float64(output.VectorAt(1).F32s()[0])) {
			t.Fatal("wrong Min/Max signed zero preference")
		}
		return true
	})
	physical.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("float bounds result: %v", result.Code())
	}
	physical, err = MakePhysicalPlan(scan.Filter(MakeColumn("x").IsNull()).Aggregate(MakeMin(MakeColumn("x"))))
	if err != nil {
		t.Fatal(err)
	}
	result = physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096}, func(output *store.Batch) bool {
		if output.VectorAt(0).Validity().IsSet(0) {
			t.Fatal("all-empty bound should be NULL")
		}
		return true
	})
	physical.Release()
	table.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("empty bound result: %v", result.Code())
	}
}
