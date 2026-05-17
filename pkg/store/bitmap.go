package store

import (
	"math/bits"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/op/bitop"
)

// MakeBitMapWithKnownNiN returns a bitmap with length `l`, a
// "nulls-in-number" of count `nin`, and data object of `data`.
func MakeBitMapWithKnownNiN(l, nin int, data *mem.Segment) *BitMap {
	return &BitMap{length: l, nin: nin, data: data}
}

// MakeBitMapWithUnknownNiN returns a bitmap with length `l`, and a
// "nulls-in-number" count based on `data`.
func MakeBitMapWithUnknownNiN(l int, data *mem.Segment) *BitMap {
	bm := &BitMap{length: l, data: data}
	_ = bm.RecalcNiN()
	return bm
}

// BitMap represents a byte array, with each bit representing
// a single element.
type BitMap struct {
	length int          // element length
	nin    int          // "nulls-in-number"
	data   *mem.Segment // data
}

// Len returns the bitmap's element length.
func (m *BitMap) Len() int { return m.length }

// Put returns the underlying data, making the bitmap undefined.
func (m *BitMap) Put() {
	m.length = 0
	m.nin = 0
	m.data.Put()
}

// ViN returns the bitmap's "valids-in-number."
func (m *BitMap) ViN() int { return m.length - m.nin }

// NiN returns the bitmap's "nulls-in-number."
func (m *BitMap) NiN() int { return m.nin }

// RecalcNiN recalculates the bitmap's "nulls-in-number," updates
// the count, and returns the new "nulls-in-number."
func (m *BitMap) RecalcNiN() int {
	pc := int(bitop.PopCount(m.data.AsBytes()))
	nin := m.length - pc
	m.nin = nin
	return nin
}

// RangeViN returns the "valids-in-number" over [start, stop).
func (m *BitMap) RangeViN(start, stop int) int {
	return rangePCFromBM(start, stop, m.data)
}

// RangeNiN returns the "nulls-in-number" over [start, stop).
func (m *BitMap) RangeNiN(start, stop int) int {
	return m.length - rangePCFromBM(start, stop, m.data)
}

// ClearAll sets all bits to zero, also updating "nulls-in-number."
// to zero.
func (m *BitMap) ClearAll() {
	m.data.MemSetU8(0)
	m.nin = m.length
}

// SetAll sets all bits to one, also updating "nulls-in-number."
// to be equal to length.
func (m *BitMap) SetAll() {
	m.data.MemSetU8(1)
	// set excess bits to zero
	if rem := m.length & 7; rem != 0 {
		m.data.AsBytes()[m.data.Len()-1] &= (255 >> rem)
	}
	m.nin = 0
}

// ANDInPlaceViN takes the bitwise AND of `m` and `x`, placing the
// result in `m`, and returns the new "valids-in-number".
func (m *BitMap) ANDInPlaceViN(x *BitMap) int {
	// all nulls in m
	if m.nin == 0 {
		return 0
	}
	// all nulls in x
	if x.nin == 0 {
		m.ClearAll()
		return 0
	}
	// alias dst
	pc := int(bitop.BitWiseAndWithPopCount(m.data.AsBytes(), x.data.AsBytes(), m.data.AsBytes()))
	m.nin = m.length - pc
	return pc
}

// ORInPlaceViN takes the bitwise ORR of `m` and `x`, placing the
// result in `m`, and returns the new "valids-in-number".
func (m *BitMap) ORInPlaceViN(x *BitMap) int {
	// all nulls in both
	if (m.nin == m.length) && (x.nin == m.length) {
		return 0
	}
	// alias dst
	pc := int(bitop.BitWiseOrWithPopCount(m.data.AsBytes(), x.data.AsBytes(), m.data.AsBytes()))
	m.nin = m.length - pc
	return pc
}

// XORInPlaceViN takes the bitwise XOR of `m` and `x`, placing the
// result in `m`, and returns the new "valids-in-number".
func (m *BitMap) XORInPlaceViN(x *BitMap) int {
	// all nulls in both
	if (m.nin == 0) && (x.nin == 0) {
		return 0
	}
	// alias dst
	pc := int(bitop.BitWiseXorWithPopCount(m.data.AsBytes(), x.data.AsBytes(), m.data.AsBytes()))
	m.nin = m.length - pc
	return pc
}

// ANDNInPlaceViN takes the bitwise ANDN of `m` and `x`, placing the
// result in `m`, and returns the new "valids-in-number".
func (m *BitMap) ANDNInPlace(x *BitMap) int {
	// all nulls in m
	if m.nin == 0 {
		return 0
	}
	// all valids in x
	if x.nin == x.length {
		m.ClearAll()
		return 0
	}
	// alias dst
	pc := int(bitop.BitWiseAndNWithPopCount(m.data.AsBytes(), x.data.AsBytes(), m.data.AsBytes()))
	m.nin = m.length - pc
	return pc
}

// AndViNFromBMs performs a bitwise AND on `x` and `y`, places the
// result in `z`, updates `z`'s state, and returns the "valids-in-number".
func AndViNFromBMs(x, y, z *BitMap) int {
	// all nulls
	if (x.nin == x.length) || (y.nin == y.length) {
		z.ClearAll()
		return 0
	}
	pc := bitop.BitWiseAndWithPopCount(
		x.data.AsBytes(), y.data.AsBytes(), z.data.AsBytes(),
	)
	z.nin = z.length - int(pc)
	return int(pc)
}

// OrViNFromBMs performs a bitwise ORR on `x` and `y`, places the
// result in `z`, updates `z`'s state, and returns the "valids-in-number".
func OrViNFromBMs(x, y, z *BitMap) int {
	// all nulls
	if (x.nin == x.length) && (y.nin == y.length) {
		z.ClearAll()
		return 0
	}
	// all valids
	if (x.nin == 0) || (y.nin == 0) {
		z.SetAll()
		return z.length
	}
	pc := bitop.BitWiseOrWithPopCount(
		x.data.AsBytes(), y.data.AsBytes(), z.data.AsBytes(),
	)
	z.nin = z.length - int(pc)
	return int(pc)
}

// XorViNFromBMs performs a bitwise XOR on `x` and `y`, places the
// result in `z`, updates `z`'s state, and returns the "valids-in-number".
func XorViNFromBMs(x, y, z *BitMap) int {
	// all nulls
	if (x.nin == x.length) && (y.nin == y.length) {
		z.ClearAll()
		return 0
	}
	// all valids
	if (x.nin == 0) && (y.nin == 0) {
		z.ClearAll()
		return 0
	}
	pc := bitop.BitWiseOrWithPopCount(
		x.data.AsBytes(), y.data.AsBytes(), z.data.AsBytes(),
	)
	z.nin = z.length - int(pc)
	return int(pc)
}

// AndNViNFromBMs performs a bitwise AND NOT on `x` and `y`, places the
// result in `z`, updates `z`'s state, and returns the "valids-in-number".
func AndNViNFromBMs(x, y, z *BitMap) int {
	// all nulls
	if (x.nin == x.length) || (y.nin == 0) {
		z.ClearAll()
		return 0
	}
	// all valids
	if (x.nin == 0) || (y.nin == y.length) {
		z.SetAll()
		return z.length
	}
	pc := bitop.BitWiseAndNWithPopCount(
		x.data.AsBytes(), y.data.AsBytes(), z.data.AsBytes(),
	)
	z.nin = z.length - int(pc)
	return int(pc)
}

// rangePCFromBM returns the population count over [start, stop) from
// a bitmap.
func rangePCFromBM(start, stop int, bm *mem.Segment) int {
	a := (start + 7) >> 3
	b := (stop - 1 + 7) >> 3
	buff := bm.AsBytes()[a : b+1]

	pc := int(bitop.PopCount(buff))

	// adjust junk data in remaining bits
	if rem := start & 7; rem != 0 {
		pc -= bits.OnesCount8(buff[0])
		pc += bits.OnesCount8(buff[0] << rem)
	}
	if rem := (stop - 1) & 7; rem != 0 {
		pc -= bits.OnesCount8(buff[len(buff)-1])
		pc += bits.OnesCount8(buff[len(buff)-1] << rem)
	}

	return pc
}
