package store

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestVectorAllocation(t *testing.T) {
	tests := []struct {
		typeValue  dtype.Type
		length     int
		byteLength int
	}{
		{dtype.Int32T(), 17, 68},
		{dtype.Int64T(), 17, 136},
		{dtype.Float32T(), 17, 68},
		{dtype.Float64T(), 17, 136},
		{dtype.DateT(), 17, 68},
		{dtype.TimestampTZT(), 17, 136},
		{dtype.StringT(), 17, 272},
		{dtype.BoolT(), 17, 3},
	}
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})

	for _, test := range tests {
		t.Run(test.typeValue.String(), func(t *testing.T) {
			v := MakeVector(a, test.length, test.typeValue, true)
			if v.Kind() != VectorFlat || !v.Type().Equal(test.typeValue) || v.Len() != test.length {
				t.Fatalf("unexpected vector metadata: %+v", v)
			}
			if got := v.Data().Len(); got != test.byteLength {
				t.Fatalf("data bytes: got %d, expected %d", got, test.byteLength)
			}
			if v.Validity() == nil || v.Validity().ViN() != test.length {
				t.Fatal("nullable vector did not start all-valid")
			}
			v.Release()
			v.Release()
		})
	}

	if v := MakeVector(a, 10, dtype.NullT(), false); v.Kind() != VectorInvalid {
		t.Fatal("allocated a physical NULL vector")
	}
	if v := MakeVector(a, 10, dtype.Type{}, false); v.Kind() != VectorInvalid {
		t.Fatal("allocated an invalid physical vector")
	}
}

func TestVectorTypedViews(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	i32 := MakeVector(a, 4, dtype.Int32T(), false)
	copy(i32.I32s(), []int32{1, 2, 3, 4})
	if i32.I64s() != nil || i32.F32s() != nil || i32.F64s() != nil || i32.Strings() != nil {
		t.Fatal("int32 vector exposed an incompatible typed view")
	}
	for i, value := range i32.I32s() {
		if value != int32(i+1) {
			t.Fatalf("value %d: got %d, expected %d", i, value, i+1)
		}
	}
	i32.Release()

	booleans := MakeVector(a, 9, dtype.BoolT(), false)
	if len(booleans.Bools()) != 2 || booleans.Bools()[0] != 0 || booleans.Bools()[1] != 0 {
		t.Fatal("boolean vector did not start clear")
	}
	booleans.Release()
}

func TestVectorOwnedSegmentValidation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	misaligned := a.AllocSeg(5)
	if v := MakeVectorFromOwnedSegments(dtype.Int32T(), 1, misaligned, nil, nil); v.Kind() != VectorInvalid {
		t.Fatal("accepted a misaligned fixed-width segment")
	}

	data := a.AllocSeg(8)
	validity := MakeBitMap(a, 1)
	if v := MakeVectorFromOwnedSegments(dtype.Int32T(), 2, data, validity, nil); v.Kind() != VectorInvalid {
		t.Fatal("accepted mismatched validity")
	}

	payload := a.AllocSeg(32)
	copy(payload.AsBytes(), "this value is longer than twelve")
	descriptors := a.AllocSeg(dtype.StringSize)
	view := unsafe.Slice((*dtype.String)(unsafe.Pointer(unsafe.SliceData(descriptors.AsBytes()))), 1)
	view[0] = dtype.MakeString(payload.AsBytes()[:32])
	owned := MakeVectorFromOwnedSegments(dtype.StringT(), 1, descriptors, nil, payload)
	if owned.Kind() != VectorFlat || owned.Strings()[0].View() != "this value is longer than twelve" {
		t.Fatal("rejected valid owned German-string storage")
	}
	owned.Release()

	external := []byte("this payload is not owned by backing")
	descriptors = a.AllocSeg(dtype.StringSize)
	view = unsafe.Slice((*dtype.String)(unsafe.Pointer(unsafe.SliceData(descriptors.AsBytes()))), 1)
	view[0] = dtype.MakeString(external)
	backing := a.AllocSeg(len(external))
	if v := MakeVectorFromOwnedSegments(dtype.StringT(), 1, descriptors, nil, backing); v.Kind() != VectorInvalid {
		t.Fatal("accepted a German-string pointer outside its backing segment")
	}
}

func TestGenericStringVectorStartsWithSafeDescriptors(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	v := MakeVector(a, 3, dtype.StringT(), false)
	for i := range v.Strings() {
		if v.Strings()[i].Len() != 0 || v.Strings()[i].View() != "" {
			t.Fatal("string vector contains a stale descriptor")
		}
	}
	v.Release()
}

func TestStringVectorOwnsLongPayload(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	values := [][]byte{
		[]byte("short"),
		[]byte("this value is longer than twelve bytes"),
		[]byte("also longer than twelve bytes"),
	}
	want := []string{string(values[0]), string(values[1]), string(values[2])}
	v := MakeStringVector(a, values, []bool{true, false, true})
	if v.Backing() == nil {
		t.Fatal("long strings have no owned backing segment")
	}
	for i := range values {
		for j := range values[i] {
			values[i][j] = 'x'
		}
	}
	values = nil
	runtime.GC()

	if got := v.Strings()[0].View(); got != want[0] {
		t.Fatalf("inline string: got %q, expected %q", got, want[0])
	}
	if v.Validity().IsSet(1) {
		t.Fatal("null string is marked valid")
	}
	if got := v.Strings()[2].View(); got != want[2] {
		t.Fatalf("long string: got %q, expected %q", got, want[2])
	}

	retained := v.Retain()
	v.Release()
	runtime.GC()
	if got := retained.Strings()[2].View(); got != want[2] {
		t.Fatalf("retained long string: got %q, expected %q", got, want[2])
	}
	retained.Release()
}
