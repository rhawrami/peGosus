package store

import (
	"fmt"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

// FSVecConfig configures the creation of a flat vector.
type FSVecConfig struct {
	// element length
	Length int
	// data type
	Type dtype.Type
	// make a validity bitmap
	MakeValidity bool
	// zero out bytes
	SetZero bool
	// allocator
	A *mem.Allocator
}

// MakeFlatVec returns a flat vector based on a flat vector config.
func MakeFlatVec(cfg FSVecConfig) *FlatVec {
	var data *mem.Segment
	var vbm *BitMap

	// use bit-size to accomodate boolean type.
	l := (cfg.Length*cfg.Type.Size1() + 7) >> 3
	data = cfg.A.AllocSeg(l)
	if cfg.MakeValidity {
		b := cfg.A.AllocSeg((cfg.Length + 7) >> 3)
		if cfg.SetZero {
			b.MemSetU8(0)
		}
		vbm = MakeBitMapWithKnownNiN(l, 0, b)
	}
	if cfg.SetZero {
		data.MemSetU8(0)
	}

	return &FlatVec{
		dType:  cfg.Type,
		length: cfg.Length,
		nin:    0,
		data:   data,
		vbm:    vbm,
	}
}

// MakeEmptyFlatVec returns an empty flat vector with
// type `t`.
func MakeEmptyFlatVec(t dtype.Type) *FlatVec {
	return &FlatVec{dType: t}
}

// FlatVec represents a vector of fixed-size data.
type FlatVec struct {
	dType  dtype.Type   // data type
	length int          // element length
	nin    int          // null count
	data   *mem.Segment // main data
	vbm    *BitMap      // validity bitmap
}

// String returns the string representation of the vector.
func (v *FlatVec) String() string {
	return fmt.Sprintf("[%d]<%s>{}", v.length, v.dType.String())
}

// Type returns the vector's type.
func (v *FlatVec) Type() dtype.Type { return v.dType }

// TypeID returns the vector's type ID.
func (v *FlatVec) TypeID() dtype.TID { return v.dType.ID() }

// Len returns the vector's element length
func (v *FlatVec) Len() int { return v.length }

// NiN returns the vector's "nulls-in-number."
func (v *FlatVec) NiN() int { return v.nin }

// RecalcNiN updates the NiN of the vector, returning the new count.
func (v *FlatVec) RecalcNiN() int {
	if v.vbm == nil {
		v.nin = 0
		return 0
	}
	nin := v.vbm.RecalcNiN()
	v.nin = nin
	return nin
}

// Data returns the segment with ID `id`, if defined; returns nil otherwise.
func (v *FlatVec) Data(id SegTypeID) *mem.Segment {
	switch id {
	case ELEMENTS:
		return v.data
	default:
		return nil
	}
}

// Put returns the data and vbm bitmap, if non-nil, to their slabs;
// makes the vector undefined.
func (v *FlatVec) Put() {
	if v.data != nil {
		v.data.Put()
		v.data = nil
	}
	if v.vbm != nil {
		v.vbm.Put()
		v.data = nil
	}
	v.length = 0
	v.nin = 0
}
