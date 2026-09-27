package plan

import (
	"context"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalDistinctAcrossBatches(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	values := [][]float64{{math.NaN(), 0, 42, 13}, {math.Float64frombits(0x7ff8000000000001), math.Copysign(0, -1), 0, 13}}
	var batches []*store.Batch
	for i, group := range values {
		v := store.MakeVector(a, len(group), dtype.Float64T(), true)
		copy(v.F64s(), group)
		v.Validity().Clear(3)
		strings := store.MakeStringVector(a, [][]byte{
			[]byte("same long string value"), []byte("same long string value"),
			[]byte("other long string value"), []byte("same long string value"),
		}, nil)
		b := store.MakeBatch([]store.Vector{v, strings})
		if i == 0 {
			b.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 4, []uint32{0, 1, 3})))
		}
		batches = append(batches, b)
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x", "s"}, []dtype.Type{dtype.Float64T(), dtype.StringT()})).Distinct())
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	var output *store.Batch
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(b *store.Batch) bool { output = b.Retain(); return true })
	physical.Release()
	if result.Code() != ExecutionCompleted || output == nil {
		t.Fatalf("DISTINCT failed: %v", result.Code())
	}
	defer output.Release()
	if output.Len() != 4 {
		t.Fatalf("got %d rows, want 4", output.Len())
	}
	if !math.IsNaN(output.VectorAt(0).F64s()[0]) || output.VectorAt(0).F64s()[1] != 0 || output.VectorAt(0).Validity().IsSet(2) || output.VectorAt(0).F64s()[3] != 0 || output.VectorAt(1).Strings()[0].View() != "same long string value" || output.VectorAt(1).Strings()[3].View() != "other long string value" {
		t.Fatal("DISTINCT canonicalization, nulls or string ownership failed")
	}
}
