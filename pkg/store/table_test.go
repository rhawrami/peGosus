package store

import (
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestTableRetainsBatches(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	v := MakeVector(a, 3, dtype.Int64T(), false)
	copy(v.I64s(), []int64{10, 20, 30})
	batch := MakeBatch([]Vector{v})
	table := MakeTable([]*Batch{batch})
	if table == nil || table.NColumns() != 1 || table.NBatches() != 1 || !table.TypeAt(0).Equal(dtype.Int64T()) {
		t.Fatal("unexpected table metadata")
	}
	if table.NullableAt(0) {
		t.Fatal("all-valid table column is unexpectedly nullable")
	}

	batch.Release()
	if got := table.BatchAt(0).VectorAt(0).I64s()[1]; got != 20 {
		t.Fatalf("retained table value: got %d, expected 20", got)
	}
	table.Release()
}

func TestTableRejectsTypeMismatch(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	x := MakeBatch([]Vector{MakeVector(a, 2, dtype.Int32T(), false)})
	y := MakeBatch([]Vector{MakeVector(a, 2, dtype.Int64T(), false)})
	if table := MakeTable([]*Batch{x, y}); table != nil {
		t.Fatal("accepted batches with different types")
	}
	x.Release()
	y.Release()
}

func TestEmptyTableRetainsTypes(t *testing.T) {
	table := MakeEmptyTable([]dtype.Type{dtype.Int64T(), dtype.StringT()})
	if !table.Valid() || table.NColumns() != 2 || table.NBatches() != 0 || !table.TypeAt(1).Equal(dtype.StringT()) {
		t.Fatal("unexpected empty table metadata")
	}
	table.Release()
	if table.Valid() || table.Retain() != nil {
		t.Fatal("released table remained valid")
	}
	if invalid := MakeEmptyTable([]dtype.Type{dtype.NullT()}); invalid != nil {
		t.Fatal("accepted NULL as a physical empty-table type")
	}
}

func TestTableTracksSourceNullability(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	vector := MakeVector(a, 2, dtype.Int32T(), true)
	vector.Validity().Clear(1)
	batch := MakeBatch([]Vector{vector})
	table := MakeTable([]*Batch{batch})
	batch.Release()
	if !table.NullableAt(0) {
		t.Fatal("table did not retain source nullability")
	}
	retained := table.Retain()
	if !retained.NullableAt(0) {
		t.Fatal("retained table lost source nullability")
	}
	retained.Release()
	table.Release()
}
