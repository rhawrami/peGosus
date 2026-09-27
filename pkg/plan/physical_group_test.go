package plan

import (
	"context"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalGroupedAggregation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	groups := [][]float64{{0, math.NaN(), 3}, {math.Copysign(0, -1), math.Float64frombits(0x7ff8000000000001), 3}}
	var batches []*store.Batch
	for i, keys := range groups {
		key := store.MakeVector(a, 3, dtype.Float64T(), true)
		copy(key.F64s(), keys)
		key.Validity().Clear(2)
		value := store.MakeVector(a, 3, dtype.Int32T(), true)
		copy(value.I32s(), []int32{1, 2, 3})
		if i == 1 {
			value.Validity().Clear(2)
		}
		batches = append(batches, store.MakeBatch([]store.Vector{key, value}))
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "value"}, []dtype.Type{dtype.Float64T(), dtype.Int32T()})).GroupBy(
		[]Expr{MakeColumn("key").Alias("group")}, MakeCountStar().Alias("n"), MakeCount(MakeColumn("value")), MakeSum(MakeColumn("value")),
	).OrderBy(MakeOrderKey(MakeColumn("n")).Desc()))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	var output *store.Batch
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(b *store.Batch) bool { output = b.Retain(); return true })
	physical.Release()
	if result.Code() != ExecutionCompleted || output == nil {
		t.Fatalf("grouped aggregation failed: %v", result.Code())
	}
	defer output.Release()
	if output.Len() != 3 {
		t.Fatalf("group count: got %d", output.Len())
	}
	if output.VectorAt(0).F64s()[0] != 0 || !math.IsNaN(output.VectorAt(0).F64s()[1]) || output.VectorAt(0).Validity().IsSet(2) {
		t.Fatal("grouping did not canonicalize signed zero, NaN or NULL")
	}
	for i, counts := range [][3]int64{{2, 2, 2}, {2, 2, 4}, {2, 1, 3}} {
		if output.VectorAt(1).I64s()[i] != counts[0] || output.VectorAt(2).I64s()[i] != counts[1] || output.VectorAt(3).I64s()[i] != counts[2] {
			t.Fatalf("group %d: count/sum mismatch", i)
		}
	}
}

func TestPhysicalGroupedCountDistinct(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	key := store.MakeVector(a, 6, dtype.Int32T(), false)
	copy(key.I32s(), []int32{1, 1, 1, 2, 2, 2})
	value := store.MakeVector(a, 6, dtype.Int32T(), true)
	copy(value.I32s(), []int32{5, 5, 6, 5, 7, 9})
	value.Validity().Clear(5)
	batch := store.MakeBatch([]store.Vector{key, value})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "value"}, []dtype.Type{dtype.Int32T(), dtype.Int32T()})).GroupBy(
		[]Expr{MakeColumn("key")}, MakeCountDistinct(MakeColumn("value")), MakeSum(MakeColumn("value")).Distinct(),
	))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(b *store.Batch) bool {
		if b.Len() != 2 || b.VectorAt(1).I64s()[0] != 2 || b.VectorAt(2).I64s()[0] != 11 || b.VectorAt(1).I64s()[1] != 2 || b.VectorAt(2).I64s()[1] != 12 {
			t.Fatal("grouped DISTINCT results differ from scalar reference")
		}
		return true
	})
	physical.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("grouped DISTINCT result: %v", result.Code())
	}
}
