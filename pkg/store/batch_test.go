package store

import (
	"testing"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestRowSelectionCheckedRepresentations(t *testing.T) {
	if unsafe.Sizeof(BitMap{}) != unsafe.Sizeof(SelVec{}) ||
		unsafe.Offsetof(BitMap{}.length) != unsafe.Offsetof(SelVec{}.length) ||
		unsafe.Offsetof(BitMap{}.nin) != unsafe.Offsetof(SelVec{}.nin) ||
		unsafe.Offsetof(BitMap{}.data) != unsafe.Offsetof(SelVec{}.data) {
		t.Fatal("bitmap and selection-vector layouts differ")
	}

	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	selection := MakeRowSelectionFromSelVec(MakeSelVecFromOffsets(a, 8, []uint32{1, 3, 7}))
	if selection.Kind() != SelectionVector || selection.Len() != 8 || selection.ActiveLen() != 3 {
		t.Fatal("unexpected selection metadata")
	}
	if _, ok := selection.AsBitMap(); ok {
		t.Fatal("selection vector returned as bitmap")
	}
	selected, ok := selection.AsSelVec()
	if !ok || len(selected.Offsets()) != 3 || selected.Offsets()[1] != 3 {
		t.Fatal("selection vector access failed")
	}
	bitmap := selection.MakeBitMapTemp(a)
	assertBitMap(t, bitmap, []bool{false, true, false, true, false, false, false, true})
	bitmap.Release()
	selection.Release()

	cleaned := MakeSelVecFromOffsets(a, 8, []uint32{7, 3, 3, 9, 1})
	if got := cleaned.Offsets(); len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 7 {
		t.Fatalf("cleaned offsets: got %v", got)
	}
	cleaned.Release()

	invalidData := a.AllocSeg(5)
	if invalid := MakeSelVec(8, 1, invalidData); invalid != nil {
		t.Fatal("accepted an unaligned selection segment")
	}
}

func TestBatchSelectionSelfAssignmentAndEmptyProjection(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	batch := MakeBatch([]Vector{MakeVector(a, 5, dtype.Int32T(), false)})
	bitmap := MakeBitMap(a, 5)
	bitmap.Set(2)
	selection := MakeRowSelectionFromBitMap(bitmap)
	batch.SetSelection(selection)
	if !batch.SetSelection(batch.Selection()) || batch.ActiveLen() != 1 {
		t.Fatal("selection self-assignment cleared the selection")
	}

	projected := batch.Project(nil)
	if projected.Len() != 5 || projected.NVectors() != 0 || projected.ActiveLen() != 1 {
		t.Fatal("empty projection did not preserve the row domain")
	}
	batch.Release()
	projected.Release()
}

func TestBatchProjectOwnership(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	left := MakeVector(a, 4, dtype.Int64T(), false)
	right := MakeStringVector(a, [][]byte{[]byte("a"), []byte("longer than twelve"), []byte("c"), []byte("d")}, nil)
	batch := MakeBatch([]Vector{left, right})
	selection := MakeBitMap(a, 4)
	selection.Set(1)
	selection.Set(3)
	if !batch.SetSelection(MakeRowSelectionFromBitMap(selection)) {
		t.Fatal("failed to set matching selection")
	}

	projected := batch.Project([]int{1, 0, 1})
	if projected == nil || projected.NVectors() != 3 || projected.ActiveLen() != 2 {
		t.Fatal("unexpected projected batch")
	}
	if got := right.Data().RefCount(); got != 3 {
		t.Fatalf("projected string data references: got %d, expected 3", got)
	}
	if got := right.Backing().RefCount(); got != 3 {
		t.Fatalf("projected string backing references: got %d, expected 3", got)
	}

	batch.Release()
	if got := projected.VectorAt(0).Strings()[1].View(); got != "longer than twelve" {
		t.Fatalf("projected string after source release: got %q", got)
	}
	projected.Release()
}

func TestBatchRejectsInvalidShapes(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	x := MakeVector(a, 2, dtype.Int32T(), false)
	y := MakeVector(a, 3, dtype.Int32T(), false)
	if batch := MakeBatch([]Vector{x, y}); batch != nil {
		t.Fatal("accepted vectors with different lengths")
	}
	x.Release()
	y.Release()
}
