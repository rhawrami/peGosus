package plan

import (
	"context"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalLimitAcrossBatchesAndSelections(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	batches := make([]*store.Batch, 4)
	for i := range batches {
		v := store.MakeVector(a, 4, dtype.Int32T(), false)
		for row := range 4 {
			v.I32s()[row] = int32(i*4 + row)
		}
		batches[i] = store.MakeBatch([]store.Vector{v})
	}
	batches[0].SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 4, []uint32{1, 3})))
	batches[2].SetSelection(store.MakeRowSelectionFromBitMap(store.MakeBitMap(a, 4)))
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int32T()}))
	for _, test := range []struct {
		count, offset int64
		want          []int32
	}{
		{3, 1, []int32{3, 4, 5}},
		{20, 100, nil},
		{0, 0, nil},
	} {
		physical, err := MakePhysicalPlan(scan.Limit(test.count, test.offset).Project(MakeColumn("x")))
		if err != nil {
			t.Fatal(err)
		}
		var got []int32
		result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096}, func(output *store.Batch) bool {
			mask := output.Selection().MakeBitMapTemp(a)
			for row := range output.Len() {
				if mask.IsSet(row) {
					got = append(got, output.VectorAt(0).I32s()[row])
				}
			}
			mask.Release()
			return true
		})
		physical.Release()
		if result.Code() != ExecutionCompleted || len(got) != len(test.want) {
			t.Fatalf("LIMIT %d OFFSET %d: result %v, rows %v, expected %v", test.count, test.offset, result.Code(), got, test.want)
		}
		for i, expected := range test.want {
			if got[i] != expected {
				t.Fatalf("row %d: got %d, expected %d", i, got[i], expected)
			}
		}
	}
	table.Release()
}
