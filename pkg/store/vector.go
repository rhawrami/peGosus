package store

import (
	"math"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

// VectorKind identifies a physical vector representation.
type VectorKind uint8

const (
	VectorInvalid VectorKind = iota
	VectorFlat
)

// MakeVector returns a general-storage flat vector.
func MakeVector(a *mem.Allocator, length int, t dtype.Type, nullable bool) Vector {
	return makeVector(a, length, t, nullable, false)
}

// MakeVectorTemp returns a temporary-storage flat vector.
func MakeVectorTemp(a *mem.Allocator, length int, t dtype.Type, nullable bool) Vector {
	return makeVector(a, length, t, nullable, true)
}

// MakeVectorFromOwnedSegments returns a flat vector that takes ownership of
// `data`, `validity`, and `backing`.
func MakeVectorFromOwnedSegments(t dtype.Type, length int, data *mem.Segment, validity *BitMap, backing *mem.Segment) Vector {
	if length < 0 || !t.Valid() || t.ID() == dtype.NULLT || data == nil || t.SlotBits() == 0 {
		releaseVectorSegments(data, validity, backing)
		return Vector{}
	}
	required, ok := vectorByteLength(length, t)
	if !ok || data.Len() < required || (t.ID() != dtype.BOOLT && data.Len()%(t.SlotBits()>>3) != 0) {
		releaseVectorSegments(data, validity, backing)
		return Vector{}
	}
	if validity != nil && validity.Len() != length {
		releaseVectorSegments(data, validity, backing)
		return Vector{}
	}
	if t.ID() != dtype.STRT && backing != nil {
		releaseVectorSegments(data, validity, backing)
		return Vector{}
	}
	if t.ID() == dtype.STRT && !germanStringsUseBacking(data, length, backing) {
		releaseVectorSegments(data, validity, backing)
		return Vector{}
	}
	return Vector{kind: VectorFlat, dType: t, length: length, data: data, validity: validity, backing: backing}
}

// MakeStringVector copies German-string descriptors and long payloads into
// general storage. A nil validity slice means every value is valid.
func MakeStringVector(a *mem.Allocator, values [][]byte, valid []bool) Vector {
	return makeStringVector(a, values, valid, false)
}

// MakeStringVectorTemp copies German-string descriptors and long payloads into
// temporary storage. A nil validity slice means every value is valid.
func MakeStringVectorTemp(a *mem.Allocator, values [][]byte, valid []bool) Vector {
	return makeStringVector(a, values, valid, true)
}

// Vector is an owner-confined physical column. Copying a Vector does not
// retain its storage; use Retain when creating another owner. Payload and
// validity mutation require sole ownership because retained vectors share
// storage.
type Vector struct {
	kind     VectorKind
	dType    dtype.Type
	length   int
	data     *mem.Segment
	validity *BitMap
	backing  *mem.Segment
}

// Kind returns the physical vector representation.
func (v *Vector) Kind() VectorKind { return v.kind }

// Type returns the logical data type.
func (v *Vector) Type() dtype.Type { return v.dType }

// TypeID returns the logical type ID.
func (v *Vector) TypeID() dtype.TID { return v.dType.ID() }

// Len returns the logical element count.
func (v *Vector) Len() int { return v.length }

// Data returns the borrowed primary data segment.
func (v *Vector) Data() *mem.Segment { return v.data }

// Backing returns the borrowed auxiliary payload segment.
func (v *Vector) Backing() *mem.Segment { return v.backing }

// Validity returns the borrowed column validity bitmap; nil means all valid.
func (v *Vector) Validity() *BitMap { return v.validity }

// NiN returns the number of null values.
func (v *Vector) NiN() int {
	if v.validity == nil {
		return 0
	}
	return v.validity.NiN()
}

// Retain returns an independently releasable vector over the same storage.
func (v *Vector) Retain() Vector {
	if v == nil || v.data == nil {
		return Vector{}
	}
	v.data.Inc()
	retained := Vector{kind: v.kind, dType: v.dType, length: v.length, data: v.data}
	retained.validity = v.validity.Retain()
	if v.backing != nil {
		v.backing.Inc()
		retained.backing = v.backing
	}
	return retained
}

// Release releases every owned segment reference and clears the vector.
func (v *Vector) Release() {
	if v == nil || v.data == nil {
		return
	}
	v.data.Dec()
	if v.validity != nil {
		v.validity.Release()
	}
	if v.backing != nil {
		v.backing.Dec()
	}
	*v = Vector{}
}

// I32s returns the borrowed int32 values for INT32 or DATE vectors.
func (v *Vector) I32s() []int32 {
	if v.dType.ID() != dtype.INT32T && v.dType.ID() != dtype.DATET {
		return nil
	}
	return v.data.AsI32T()[:v.length]
}

// I64s returns the borrowed int64 values for INT64 or TIMESTAMPTZ vectors.
func (v *Vector) I64s() []int64 {
	if v.dType.ID() != dtype.INT64T && v.dType.ID() != dtype.TIMESTAMPTZT {
		return nil
	}
	return v.data.AsI64T()[:v.length]
}

// F32s returns the borrowed float32 values.
func (v *Vector) F32s() []float32 {
	if v.dType.ID() != dtype.FLOAT32T {
		return nil
	}
	return v.data.AsF32T()[:v.length]
}

// F64s returns the borrowed float64 values.
func (v *Vector) F64s() []float64 {
	if v.dType.ID() != dtype.FLOAT64T {
		return nil
	}
	return v.data.AsF64T()[:v.length]
}

// Bools returns the borrowed bit-packed boolean bytes.
func (v *Vector) Bools() []byte {
	if v.dType.ID() != dtype.BOOLT {
		return nil
	}
	return v.data.AsBytes()[:(v.length+7)>>3]
}

// Strings returns the borrowed German-string descriptors. Assigned long-string
// descriptors must point into Backing; use MakeStringVector to copy arbitrary
// values safely. Descriptor mutation requires sole vector ownership.
func (v *Vector) Strings() []dtype.String {
	if v.dType.ID() != dtype.STRT || v.length == 0 {
		return nil
	}
	return unsafe.Slice((*dtype.String)(unsafe.Pointer(unsafe.SliceData(v.data.AsBytes()))), v.length)
}

func makeVector(a *mem.Allocator, length int, t dtype.Type, nullable, temporary bool) Vector {
	if length < 0 {
		length = 0
	}
	if a == nil || !t.Valid() || t.ID() == dtype.NULLT || t.SlotBits() == 0 {
		return Vector{}
	}
	byteLength, ok := vectorByteLength(length, t)
	if !ok {
		return Vector{}
	}
	var data *mem.Segment
	if temporary {
		data = a.AllocSegTemp(byteLength)
	} else {
		data = a.AllocSeg(byteLength)
	}
	var validity *BitMap
	if nullable {
		if temporary {
			validity = MakeBitMapTemp(a, length)
		} else {
			validity = MakeBitMap(a, length)
		}
		validity.SetAll()
	}
	if t.ID() == dtype.BOOLT || t.ID() == dtype.STRT {
		values := data.AsBytes()
		for i := range values {
			values[i] = 0
		}
	}
	return Vector{kind: VectorFlat, dType: t, length: length, data: data, validity: validity}
}

func makeStringVector(a *mem.Allocator, values [][]byte, valid []bool, temporary bool) Vector {
	v := makeVector(a, len(values), dtype.StringT(), valid != nil, temporary)
	if v.kind == VectorInvalid {
		return v
	}
	var payloadLength int
	maxInt := int(^uint(0) >> 1)
	for i, value := range values {
		if valid != nil && (i >= len(valid) || !valid[i]) {
			v.validity.Clear(i)
			continue
		}
		if len(value) > dtype.StringInlineSize {
			if uint64(len(value)) > math.MaxUint32 || len(value) > maxInt-payloadLength {
				v.Release()
				return Vector{}
			}
			payloadLength += len(value)
		}
	}

	if payloadLength != 0 {
		if temporary {
			v.backing = a.AllocSegTemp(payloadLength)
		} else {
			v.backing = a.AllocSeg(payloadLength)
		}
	}

	descriptors := v.Strings()
	var on int
	for i, value := range values {
		if valid != nil && (i >= len(valid) || !valid[i]) {
			continue
		}
		if len(value) <= dtype.StringInlineSize {
			descriptors[i] = dtype.MakeString(value)
			continue
		}
		payload := v.backing.AsBytes()[on : on+len(value)]
		copy(payload, value)
		descriptors[i] = dtype.MakeString(payload)
		on += len(value)
	}
	return v
}

func vectorByteLength(length int, t dtype.Type) (int, bool) {
	if length < 0 || t.SlotBits() <= 0 {
		return 0, false
	}
	maxInt := int(^uint(0) >> 1)
	if length > (maxInt-7)/t.SlotBits() {
		return 0, false
	}
	return (length*t.SlotBits() + 7) >> 3, true
}

func releaseVectorSegments(data *mem.Segment, validity *BitMap, backing *mem.Segment) {
	if data != nil {
		data.Dec()
	}
	if validity != nil {
		validity.Release()
	}
	if backing != nil {
		backing.Dec()
	}
}

func germanStringsUseBacking(data *mem.Segment, length int, backing *mem.Segment) bool {
	type stringLayout struct {
		length  uint32
		prefix  [4]byte
		address uint64
	}
	if length == 0 {
		return true
	}
	descriptors := unsafe.Slice((*stringLayout)(unsafe.Pointer(unsafe.SliceData(data.AsBytes()))), length)
	var backingStart, backingEnd uintptr
	if backing != nil && backing.Len() != 0 {
		bytes := backing.AsBytes()
		backingStart = uintptr(unsafe.Pointer(unsafe.SliceData(bytes)))
		backingEnd = backingStart + uintptr(len(bytes))
	}
	for _, descriptor := range descriptors {
		if descriptor.length <= dtype.StringInlineSize {
			continue
		}
		start := uintptr(descriptor.address)
		end := start + uintptr(descriptor.length)
		if backing == nil || end < start || start < backingStart || end > backingEnd {
			return false
		}
	}
	return true
}
