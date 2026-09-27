package plan

import (
	"context"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalOrderByAndLimit(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	var batches []*store.Batch
	for _, input := range [][]float64{{4, math.NaN(), -3}, {math.Inf(1), 1, math.Inf(-1)}, {99}} {
		v := store.MakeVector(a, len(input), dtype.Float64T(), true)
		copy(v.F64s(), input)
		if len(input) == 1 {
			v.Validity().Clear(0)
		}
		batches = append(batches, store.MakeBatch([]store.Vector{v}))
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Float64T()}))
	for _, test := range []struct {
		key           OrderKey
		limit, offset int64
		want          []float64
		nullAt        int
	}{
		{MakeOrderKey(MakeColumn("x")), 7, 0, []float64{math.Inf(-1), -3, 1, 4, math.Inf(1), math.NaN(), 0}, 6},
		{MakeOrderKey(MakeColumn("x")).Desc().NullsFirst(), 3, 1, []float64{math.NaN(), math.Inf(1), 4}, -1},
	} {
		physical, err := MakePhysicalPlan(scan.OrderBy(test.key).Limit(test.limit, test.offset))
		if err != nil {
			t.Fatal(err)
		}
		var output *store.Batch
		result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(b *store.Batch) bool { output = b.Retain(); return true })
		physical.Release()
		if result.Code() != ExecutionCompleted || output == nil {
			t.Fatalf("sort result: %v", result.Code())
		}
		mask := output.Selection().MakeBitMapTemp(a)
		var got []float64
		for row := range output.Len() {
			if mask != nil && !mask.IsSet(row) {
				continue
			}
			if output.VectorAt(0).Validity() != nil && !output.VectorAt(0).Validity().IsSet(row) {
				got = append(got, 0)
			} else {
				got = append(got, output.VectorAt(0).F64s()[row])
			}
		}
		if mask != nil {
			mask.Release()
		}
		output.Release()
		if len(got) != len(test.want) {
			t.Fatalf("got %v, want %v", got, test.want)
		}
		for i, expected := range test.want {
			if got[i] != expected && !(math.IsNaN(got[i]) && math.IsNaN(expected)) {
				t.Fatalf("row %d: got %v, want %v", i, got[i], expected)
			}
		}
	}
	table.Release()
}

func TestPhysicalTopNBoundsMemory(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	batches := make([]*store.Batch, 20)
	for i := range batches {
		v := store.MakeVector(a, 7, dtype.Int64T(), false)
		for j := range v.I64s() {
			v.I64s()[j] = int64(140 - i*7 - j)
		}
		batches[i] = store.MakeBatch([]store.Vector{v})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()}))
	physical, err := MakePhysicalPlan(scan.OrderBy(MakeOrderKey(MakeColumn("x"))).Limit(2, 1))
	if err != nil {
		t.Fatal(err)
	}
	if !physical.steps[0].hasTopN || physical.steps[0].topN != 3 {
		t.Fatal("ordered limit did not lower to TopN")
	}
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1024}, func(output *store.Batch) bool {
		if output.ActiveLen() != 2 || output.VectorAt(0).I64s()[1] != 2 || output.VectorAt(0).I64s()[2] != 3 {
			t.Fatalf("wrong TopN output: %v", output.VectorAt(0).I64s())
		}
		return true
	})
	physical.Release()
	table.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("TopN result: %v", result.Code())
	}
}
