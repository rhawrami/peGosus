package store

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

// SegTypeID identifies what type of data a
// segment is holding.
type SegTypeID int

const (
	// actual elements (defined on all types)
	ELEMENTS SegTypeID = iota
	// offsets (defined on variable size data)
	OFFSETS
	// validity (defined on vectors with nulls)
	VALIDITY
)

// Vec represents an array of same-type data.
type Vec interface {
	// implement Stringer.
	String() string
	// Type returns the vector's type.
	Type() dtype.Type
	// TypeID returns the vector's type ID.
	TypeID() dtype.TID
	// Len returns the element length.
	Len() int
	// NiN returns the "nulls-in-number."
	NiN() int
	// RecalcNiN recounts the NiN, updates the null count,
	// and returns the new null count.
	RecalcNiN() int
	// Data returns data with the type `id`; returns
	// nil if undefined on type, or not available.
	Data(id SegTypeID) *mem.Segment
	// TakeIn takes in a segment, and places it at location `t`;
	// does nothing if `t` is not defined on the vector.
	TakeIn(x *mem.Segment, t SegTypeID)
	// MakeVBM makes a validity bitmap for the vector, and sets all
	// bits to zero if setZero; does nothing if there's already a vbm.
	MakeVBM(a *mem.Allocator, setZero bool)
	// Put returns all segments to their slabs, making
	// the vector undefined.
	Put()
}
