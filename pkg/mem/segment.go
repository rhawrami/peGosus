package mem

import (
	"fmt"
	"sync/atomic"
)

// Segment represents one piece of a slab.
type Segment struct {
	base     *byte        // base address
	length   int          // byte length
	capacity int          // maximum byte capacity
	refCount atomic.Int64 // reference count
	slab     *Slab        // slab that segment belongs to
}

// String summarizes a segment's state.
func (s *Segment) String() string {
	return fmt.Sprintf(
		"%p Segm[%dB, %.0f%% used] {%d}",
		s,
		s.capacity,
		float64(s.length)/float64(s.capacity)*100,
		s.refCount.Load(),
	)
}

// Len returns the segment's current byte length.
func (s *Segment) Len() int { return s.length }

// Cap returns the segment's maximum byte capacity.
func (s *Segment) Cap() int { return s.capacity }

// RefCount returns the reference count of `s`.
func (s *Segment) RefCount() int64 { return s.refCount.Load() }

// IsFree returns true if the reference count is 0.
func (s *Segment) IsFree() bool { return s.refCount.Load() == 0 }

// CanSupport returns true if `s` has space for `l` elements, each of size `t`.
func (s *Segment) CanSupport(l, t int) bool {
	return s.capacity >= l*t
}

// Put returns `s` to its slab.
func (s *Segment) Put() {
	s.slab.TakeSegment(s)
}

// IsAligned checks if the base address for `s` is aligned to `x` bytes.
func (s *Segment) IsAligned(x int) bool {
	return isAligned(s.base, x)
}

// Dec decrements the reference count by 1; if updated count is 0, `s` is
// returned to its slab; returns false if `s` was returned to its slab.
func (s *Segment) Dec() bool {
	if s.refCount.Add(-1) == 0 {
		s.Put()
		return false
	}
	return true
}

// Inc increments the reference count by 1.
func (s *Segment) Inc() {
	s.refCount.Add(1)
}

// AddLength increases the length by `l`; sets length to the max
// length if `l` + current length > capacity.
func (s *Segment) AddLength(l int) {
	if l+s.length > s.capacity {
		s.length = s.capacity
	} else {
		s.length += l
	}
}

// SubLength decreases the length by `l`; sets length to 0
// if `l` > current length.
func (s *Segment) SubLength(l int) {
	if l > s.length {
		s.length = 0
	} else {
		s.length -= l
	}
}

// SetLength sets the length to `l` bytes; sets length to capacity
// if `l` > capacity.
func (s *Segment) SetLength(l int) {
	length := l
	if length > s.capacity {
		length = s.capacity
	}
	s.length = length
}

// SetLengthToCap sets the length to the capacity.
func (s *Segment) SetLengthToCap() { s.length = s.capacity }

// MemSetU8 sets every byte, from the base address to the current set
// length, with value `v`.
func (s *Segment) MemSetU8(v uint8) {
	setU8(s.base, s.length, v)
}

// MemSetU32 sets every four bytes, from the base address to the current set
// length, with value `v`; panics if s.length isn't divisible by four.
func (s *Segment) MemSetU32(v uint32) {
	if s.length&3 != 0 {
		panic("MemSetU32: segment length not divisible by 4")
	}
	setU32(s.base, s.length>>2, v)
}

// MemSetU64 sets every eight bytes, from the base address to the current set
// length, with value `v`; panics if s.length isn't divisible by eight.
func (s *Segment) MemSetU64(v uint64) {
	if s.length&7 != 0 {
		panic("MemSetU64: segment length not divisible by 8")
	}
	setU64(s.base, s.length>>3, v)
}

// MemSetU8Detailed sets every byte, from [base address + `o`] to `l` bytes,
// with the value `v`; panics if offsetted length is greater than base length.
func (s *Segment) MemSetU8Detailed(v uint8, l, o int) {
	if s.length < (o+l) || s.length < o {
		panic("MemSetU8Detailed: offseted length greated than segment length")
	}
	addr := incPtr(s.base, o)
	setU8(addr, l, v)
}

// MemSetU32Detailed sets every four bytes, from [base address + `o`] to `l` bytes,
// with the value `v`; panics if offsetted length is greater than base length, or
// if length is not divisible by four.
func (s *Segment) MemSetU32Detailed(v uint32, l, o int) {
	if s.length < (o+l) || s.length < o {
		panic("MemSetU32Detailed: offseted length greated than segment length")
	}
	addr := incPtr(s.base, o)
	if l&3 != 0 {
		panic("MemSetU32Detailed: address not divisible by 4")
	}

	setU32(addr, l>>2, v)
}

// MemSetU64Detailed sets every eight bytes, from [base address + `o`] to `l` bytes,
// with the value `v`; panics if offsetted length is greater than base length,
// or if length is not divisible by eight.
func (s *Segment) MemSetU64Detailed(v uint64, l, o int) {
	if s.length < (o+l) || s.length < o {
		panic("MemSetU64Detailed: offseted length greated than segment length")
	}
	addr := incPtr(s.base, o)
	if l&7 != 0 {
		panic("MemSetU64Detailed: length not divisible by 8")
	}
	setU64(addr, l>>3, v)
}

// AsBytes casts `s` as a slice of bytes with length equal to the segment length.
func (s *Segment) AsBytes() []byte { return asBT(s.base, s.length) }

// AsI64T casts `s` as a slice of 64-bit signed integers with length equal to the
// segment length divided by eight; panics if s.length is not divisible by eight
func (s *Segment) AsI64T() []int64 {
	if s.length&7 != 0 {
		panic("AsI64T: segment length not divisibly by 8")
	}
	return asI64T(s.base, s.length>>3)
}

// AsI32T casts `s` as a slice of 32-bit signed integers with length equal to the
// segment length divided by four; panics if s.length is not divisible by four.
func (s *Segment) AsI32T() []int32 {
	if s.length&3 != 0 {
		panic("AsI32T: segment length not divisibly by 4")
	}
	return asI32T(s.base, s.length>>2)
}

// AsF64T casts `s` as a slice of 64-bit floating point values with length equal to the
// segment length divided by eight; panics if s.length is not divisible by eight.
func (s *Segment) AsF64T() []float64 {
	if s.length&7 != 0 {
		panic("AsF64T: segment length not divisibly by 8")
	}
	return asF64T(s.base, s.length>>3)
}

// AsF32T casts `s` as a slice of 32-bit floating point values with length equal to the
// segment length divided by four; panics if s.length is not divisible by four.
func (s *Segment) AsF32T() []float32 {
	if s.length&3 != 0 {
		panic("AsF32T: segment length not divisibly by 4")
	}
	return asF32T(s.base, s.length>>2)
}

// AsU64T casts `s` as a slice of 64-bit unsigned integers with length equal to the
// segment length divided by eight; panics if s.length is not divisible by eight.
func (s *Segment) AsU64T() []uint64 {
	if s.length&7 != 0 {
		panic("AsU64T: segment length not divisibly by 8")
	}
	return asU64T(s.base, s.length>>3)
}

// AsU32T casts `s` as a slice of 32-bit unsigned integers with length equal to the
// segment length divided by four; panics if s.length is not divisible by four.
func (s *Segment) AsU32T() []uint32 {
	if s.length&3 != 0 {
		panic("AsU32T: segment length not divisibly by 4")
	}
	return asU32T(s.base, s.length>>2)
}
