package store

import (
	"fmt"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

// VSVecConfig configures the creation of a variable-size vector.
type VSVecConfig struct {
	// element length
	Length int
	// buffer byte length
	BuffLength int
	// data type
	Type dtype.Type
	// make a validity bitmap
	MakeValidity bool
	// zero out bytes
	SetZero bool
	// allocator
	A *mem.Allocator
}

// MakeVariableSizeVec returns a variable-size vector based on a variable-size vector config.
func MakeVariableSizeVec(cfg VSVecConfig) *VariableSizeVec {
	var data, offsets *mem.Segment
	var vbm *BitMap

	data = cfg.A.AllocSeg(cfg.Length * cfg.BuffLength)
	offsets = cfg.A.AllocSeg(cfg.Length * dtype.VariableOffsetByteSize)
	if cfg.MakeValidity {
		b := cfg.A.AllocSeg((cfg.Length + 7) >> 3)
		if cfg.SetZero {
			b.MemSetU8(0)
		}
		vbm = MakeBitMapWithKnownNiN(cfg.Length, 0, b)
	}
	if cfg.SetZero {
		data.MemSetU8(0)
		offsets.MemSetU8(0)
	}

	return &VariableSizeVec{
		dType:   cfg.Type,
		length:  cfg.Length,
		nin:     0,
		data:    data,
		offsets: offsets,
		vbm:     vbm,
	}
}

// MakeEmptyVariableSizeVec returns an empty variable-size vector with
// type `t`.
func MakeEmptyVariableSizeVec(t dtype.Type) *VariableSizeVec {
	return &VariableSizeVec{dType: t}
}

// VariableSizeVec represents a vector of variable-size data.
type VariableSizeVec struct {
	dType   dtype.Type   // data type
	length  int          // element length
	nin     int          // null count
	data    *mem.Segment // main data
	offsets *mem.Segment // offsets
	vbm     *BitMap      // validity bitmap
}

// String returns the string representation of the vector.
func (v *VariableSizeVec) String() string {
	return fmt.Sprintf("[%d]<%s>{}", v.length, v.dType.String())
}

// Type returns the vector's type.
func (v *VariableSizeVec) Type() dtype.Type { return v.dType }

// TypeID returns the vector's type ID.
func (v *VariableSizeVec) TypeID() dtype.TID { return v.dType.ID() }

// Len returns the vector's element length
func (v *VariableSizeVec) Len() int { return v.length }

// NiN returns the vector's "nulls-in-number."
func (v *VariableSizeVec) NiN() int { return v.nin }

// RecalcNiN updates the NiN of the vector, returning the new count.
func (v *VariableSizeVec) RecalcNiN() int {
	if v.vbm == nil {
		v.nin = 0
		return 0
	}
	nin := v.vbm.RecalcNiN()
	v.nin = nin
	return nin
}

// Data returns the segment with ID `id`, if defined; returns nil otherwise.
func (v *VariableSizeVec) Data(id SegTypeID) *mem.Segment {
	switch id {
	case ELEMENTS:
		return v.data
	case OFFSETS:
		return v.offsets
	default:
		return nil
	}
}

// Put returns the data and validity bitmap, if non-nil, to their slabs;
// makes the vector undefined.
func (v *VariableSizeVec) Put() {
	// if data isn't nil, then offsets won't be nil either
	if v.data != nil {
		v.data.Put()
		v.data = nil
		v.offsets.Put()
		v.offsets = nil
	}
	if v.vbm != nil {
		v.vbm.Put()
		v.data = nil
	}
	v.length = 0
	v.nin = 0
}
