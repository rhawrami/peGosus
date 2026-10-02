package plan

import (
	"bytes"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestFilterReusesOwnedPredicateWithoutMutatingBorrowedInput(t *testing.T) {
	for _, derived := range []bool{false, true} {
		for _, n := range []int{0, 1, 7, 9, 65} {
			a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
			value := store.MakeVector(a, n, dtype.BoolT(), true)
			selected := store.MakeBitMap(a, n)
			for row := range n {
				if row%3 == 0 {
					value.Bools()[row>>3] |= 1 << (row & 7)
				}
				if row%5 == 0 {
					value.Validity().Clear(row)
				}
				if row%7 != 0 {
					selected.Set(row)
				}
			}
			batch := store.MakeBatch([]store.Vector{value})
			batch.SetSelection(store.MakeRowSelectionFromBitMap(selected))
			table := store.MakeTable([]*store.Batch{batch})
			expr := MakeColumn("keep")
			if derived {
				expr = expr.Not()
			}
			p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"keep"}, []dtype.Type{dtype.BoolT()})).Filter(expr))
			if err != nil {
				t.Fatal(err)
			}
			data, validity, selection := bytes.Clone(value.Bools()), bytes.Clone(value.Validity().Bytes()), bytes.Clone(selected.Bytes())
			owned := batch.Retain()
			before := a.Usage()
			if !executePhysicalFilter(a, owned, p.source.filters[0]) {
				t.Fatal("filter failed")
			}
			if derived && a.Usage().GeneralRequests != before.GeneralRequests {
				t.Fatal("owned predicate still copied into general selection storage")
			}
			mask, _ := owned.Selection().AsBitMap()
			for row := range n {
				keep := row%3 == 0
				if derived {
					keep = !keep
				}
				if mask.IsSet(row) != (keep && row%5 != 0 && row%7 != 0) {
					t.Fatalf("row %d changed predicate semantics", row)
				}
			}
			if !bytes.Equal(data, value.Bools()) || !bytes.Equal(validity, value.Validity().Bytes()) || !bytes.Equal(selection, selected.Bytes()) {
				t.Fatal("filter changed retained input storage")
			}
			owned.Release()
			p.Release()
			table.Release()
			batch.Release()
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("filter leaked %+v", usage)
			}
		}
	}
}
