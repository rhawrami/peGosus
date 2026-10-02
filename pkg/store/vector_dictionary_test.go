package store

import (
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestDictionaryStringVectorOwnershipAndValidation(t *testing.T) {
	for _, n := range []int{0, 1, 7, 9, 65} {
		a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
		scope := mem.MakeAllocationScope(a, 8192)
		view := mem.MakeAllocatorWithScope(a, scope)
		dictionary := MakeStringVector(view, [][]byte{nil, []byte("a\x00ø"), []byte("a long retained dictionary string"), []byte("a\x00ø")}, nil)
		indices, validity := view.AllocSeg(n*4), MakeBitMap(view, n)
		for row := range n {
			indices.AsU32T()[row] = uint32(row % 4)
			if row%5 != 0 {
				validity.Set(row)
			} else {
				indices.AsU32T()[row] = math.MaxUint32
			}
		}
		v := MakeDictionaryStringVectorFromOwnedSegments(dictionary, n, indices, validity)
		if v.Kind() != VectorDictionaryString || v.Strings() != nil || v.Dictionary() == nil {
			t.Fatal("missing encoded representation")
		}
		retained := v.Retain()
		v.Release()
		batch := MakeBatchRetained([]Vector{retained})
		retained.Release()
		copy := batch.Project([]int{0})
		batch.Release()
		v = copy.VectorAt(0).Retain()
		copy.Release()
		for row := range n {
			want := ""
			if row%5 != 0 {
				want = v.Dictionary().StringAt(row % 4).View()
			}
			if v.StringAt(row).View() != want || v.Validity().IsSet(row) != (row%5 != 0) {
				t.Fatalf("row %d lost its dictionary or null", row)
			}
		}
		v.Release()
		if scope.Live() != 0 {
			t.Fatalf("retained vector leaked %d bytes", scope.Live())
		}
		for _, invalid := range []uint32{1, math.MaxUint32} {
			dictionary = MakeStringVector(view, [][]byte{[]byte("only value")}, nil)
			indices = view.AllocSeg(4)
			indices.AsU32T()[0] = invalid
			if v = MakeDictionaryStringVectorFromOwnedSegments(dictionary, 1, indices, nil); v.Kind() != VectorInvalid || scope.Live() != 0 {
				t.Fatal("invalid row ID accepted or leaked")
			}
		}
	}
}

func TestRetainSelectionBitmapOwnership(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	for _, n := range []int{0, 1, 7, 9, 65} {
		bitmap := MakeBitMap(a, n)
		for row := 0; row < n; row += 3 {
			bitmap.Set(row)
		}
		selection := MakeRowSelectionFromBitMap(bitmap)
		before := a.Usage()
		retained := selection.RetainBitMap(a)
		if retained.Data() != bitmap.Data() || a.Usage().ScratchRequests != before.ScratchRequests {
			t.Fatal("read-only bitmap view allocated or copied")
		}
		selection.Release()
		for row := range n {
			if retained.IsSet(row) != (row%3 == 0) {
				t.Fatal("retained bitmap changed")
			}
		}
		retained.Release()
		selection = MakeRowSelectionFromSelVec(MakeSelVecFromOffsets(a, n, []uint32{0, 3, 6}))
		retained = selection.RetainBitMap(a)
		selection.Release()
		if retained.Len() != n {
			t.Fatal("selection-vector domain changed")
		}
		retained.Release()
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("selection leaked: %+v", usage)
	}
}
