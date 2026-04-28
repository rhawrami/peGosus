package mem

import "sync/atomic"

// Segment represents one piece of a slab.
type Segment struct {
	base     *byte        // base address
	length   uint64       // byte length
	capacity uint64       // maximum byte capacity
	refCount atomic.Int64 // reference count
	slab     *Slab        // slab that segment belongs to
}

// CanSupport returns true if `s` has space for `l` elements, each of size `t`.
func (s *Segment) CanSupport(l, t int) bool {
	return s.capacity >= uint64(l*t)
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

// AsBytes casts `s` as a slice of bytes with length `l`.
func (s *Segment) AsBytes(l int) []byte { return asBT(s.base, l) }

// AsI64T casts `s` as a slice of 64-bit signed integers with length `l`.
func (s *Segment) AsI64T(l int) []int64 { return asI64T(s.base, l) }

// AsI32T casts `s` as a slice of 32-bit signed integers with length `l`.
func (s *Segment) AsI32T(l int) []int32 { return asI32T(s.base, l) }

// AsF64T casts `s` as a slice of 64-bit floating point values with length `l`.
func (s *Segment) AsF64T(l int) []float64 { return asF64T(s.base, l) }

// AsF32T casts `s` as a slice of 32-bit floating point values with length `l`.
func (s *Segment) AsF32T(l int) []float32 { return asF32T(s.base, l) }

// AsU32T casts `s` as a slice of 32-bit unsigned integers with length `l`.
func (s *Segment) AsU32T(l int) []uint32 { return asU32T(s.base, l) }
