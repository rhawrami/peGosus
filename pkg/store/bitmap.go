package store

import (
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/op/bitop"
)

// MakeBitMap returns an all-clear bitmap allocated from general storage.
func MakeBitMap(a *mem.Allocator, l int) *BitMap {
	return makeBitMap(a, l, false)
}

// MakeBitMapTemp returns an all-clear bitmap allocated from temporary storage.
func MakeBitMapTemp(a *mem.Allocator, l int) *BitMap {
	return makeBitMap(a, l, true)
}

// MakeBitMapWithKnownNiN returns a bitmap that takes ownership of `data` and
// trusts the supplied null count. The logical length is limited by the data.
func MakeBitMapWithKnownNiN(l, nin int, data *mem.Segment) *BitMap {
	l = cleanBitMapLength(l, data)
	if nin < 0 {
		nin = 0
	} else if nin > l {
		nin = l
	}
	m := &BitMap{length: l, nin: nin, data: data}
	m.maskTail()
	return m
}

// MakeBitMapWithUnknownNiN returns a bitmap that takes ownership of `data`
// and calculates its null count. The logical length is limited by the data.
func MakeBitMapWithUnknownNiN(l int, data *mem.Segment) *BitMap {
	m := &BitMap{length: cleanBitMapLength(l, data), data: data}
	m.RecalcNiN()
	return m
}

// BitMap represents an LSB-first validity bitmap; set bits are valid.
type BitMap struct {
	length int
	nin    int
	data   *mem.Segment
}

// Len returns the bitmap's logical element length.
func (m *BitMap) Len() int { return m.length }

// ViN returns the number of set bits in the logical range.
func (m *BitMap) ViN() int { return m.length - m.nin }

// NiN returns the number of clear bits in the logical range.
func (m *BitMap) NiN() int { return m.nin }

// Data returns the borrowed backing segment.
func (m *BitMap) Data() *mem.Segment { return m.data }

// Bytes returns a borrowed view containing only logical bitmap bytes.
func (m *BitMap) Bytes() []byte { return m.bytes() }

// IsSet returns whether the bit at `o` is set.
func (m *BitMap) IsSet(o int) bool {
	return o >= 0 && o < m.length && (m.bytes()[o>>3]>>(o&7))&1 == 1
}

// Set sets the bit at `o` and updates the null count.
func (m *BitMap) Set(o int) {
	if o < 0 || o >= m.length || m.IsSet(o) {
		return
	}
	m.bytes()[o>>3] |= 1 << (o & 7)
	m.nin--
}

// Clear clears the bit at `o` and updates the null count.
func (m *BitMap) Clear(o int) {
	if o < 0 || o >= m.length || !m.IsSet(o) {
		return
	}
	m.bytes()[o>>3] &^= 1 << (o & 7)
	m.nin++
}

// Retain returns an independently releasable bitmap over the same data.
// Payload mutation requires sole ownership because null counts are per wrapper.
func (m *BitMap) Retain() *BitMap {
	if m == nil || m.data == nil {
		return nil
	}
	m.data.Inc()
	return &BitMap{length: m.length, nin: m.nin, data: m.data}
}

// Release releases the bitmap's segment reference and clears the wrapper.
func (m *BitMap) Release() {
	if m == nil || m.data == nil {
		return
	}
	m.data.Dec()
	m.length = 0
	m.nin = 0
	m.data = nil
}

// Put releases the bitmap.
func (m *BitMap) Put() { m.Release() }

// RecalcNiN recalculates and returns the number of clear logical bits.
func (m *BitMap) RecalcNiN() int {
	m.maskTail()
	if m.length == 0 {
		m.nin = 0
		return 0
	}
	m.nin = m.length - int(bitop.PopCount(m.bytes()))
	return m.nin
}

// RangeViN returns the number of set bits over the cleaned range [start, stop).
func (m *BitMap) RangeViN(start, stop int) int {
	start, stop = m.cleanRange(start, stop)
	var count int
	data := m.bytes()
	for i := start; i < stop; i++ {
		count += int((data[i>>3] >> (i & 7)) & 1)
	}
	return count
}

// RangeNiN returns the number of clear bits over the cleaned range [start, stop).
func (m *BitMap) RangeNiN(start, stop int) int {
	start, stop = m.cleanRange(start, stop)
	return stop - start - m.RangeViN(start, stop)
}

// ClearAll clears every logical bit.
func (m *BitMap) ClearAll() {
	data := m.bytes()
	for i := range data {
		data[i] = 0
	}
	m.nin = m.length
}

// SetAll sets every logical bit and clears all tail padding.
func (m *BitMap) SetAll() {
	data := m.bytes()
	for i := range data {
		data[i] = 0xff
	}
	m.maskTail()
	m.nin = 0
}

// ANDInPlaceViN intersects `m` with `x` and returns the set-bit count.
func (m *BitMap) ANDInPlaceViN(x *BitMap) int {
	return AndViNFromBMs(m, x, m)
}

// ORInPlaceViN unions `m` with `x` and returns the set-bit count.
func (m *BitMap) ORInPlaceViN(x *BitMap) int {
	return OrViNFromBMs(m, x, m)
}

// XORInPlaceViN applies XOR to `m` and `x` and returns the set-bit count.
func (m *BitMap) XORInPlaceViN(x *BitMap) int {
	return XorViNFromBMs(m, x, m)
}

// ANDNInPlace applies AND NOT to `m` and `x` and returns the set-bit count.
func (m *BitMap) ANDNInPlace(x *BitMap) int {
	return AndNViNFromBMs(m, x, m)
}

// AndViNFromBMs places `x & y` in `z` and returns its set-bit count.
func AndViNFromBMs(x, y, z *BitMap) int {
	if !bitMapShapesEqual(x, y, z) {
		if z != nil {
			z.ClearAll()
		}
		return 0
	}
	if x.length == 0 {
		z.nin = 0
		return 0
	}
	bitop.BitWiseAndWithPopCount(x.bytes(), y.bytes(), z.bytes())
	return z.recalcAfterBinary()
}

// OrViNFromBMs places `x | y` in `z` and returns its set-bit count.
func OrViNFromBMs(x, y, z *BitMap) int {
	if !bitMapShapesEqual(x, y, z) {
		if z != nil {
			z.ClearAll()
		}
		return 0
	}
	if x.length == 0 {
		z.nin = 0
		return 0
	}
	bitop.BitWiseOrWithPopCount(x.bytes(), y.bytes(), z.bytes())
	return z.recalcAfterBinary()
}

// XorViNFromBMs places `x ^ y` in `z` and returns its set-bit count.
func XorViNFromBMs(x, y, z *BitMap) int {
	if !bitMapShapesEqual(x, y, z) {
		if z != nil {
			z.ClearAll()
		}
		return 0
	}
	if x.length == 0 {
		z.nin = 0
		return 0
	}
	bitop.BitWiseXorWithPopCount(x.bytes(), y.bytes(), z.bytes())
	return z.recalcAfterBinary()
}

// AndNViNFromBMs places `x &^ y` in `z` and returns its set-bit count.
func AndNViNFromBMs(x, y, z *BitMap) int {
	if !bitMapShapesEqual(x, y, z) {
		if z != nil {
			z.ClearAll()
		}
		return 0
	}
	if x.length == 0 {
		z.nin = 0
		return 0
	}
	bitop.BitWiseAndNWithPopCount(x.bytes(), y.bytes(), z.bytes())
	return z.recalcAfterBinary()
}

func cleanBitMapLength(l int, data *mem.Segment) int {
	if l < 0 || data == nil {
		return 0
	}
	maxInt := int(^uint(0) >> 1)
	max := maxInt
	if data.Len() <= maxInt>>3 {
		max = data.Len() << 3
	}
	if l > max {
		return max
	}
	return l
}

func makeBitMap(a *mem.Allocator, l int, temporary bool) *BitMap {
	if l < 0 {
		l = 0
	}
	var data *mem.Segment
	if temporary {
		data = a.AllocSegTemp(bitMapByteLength(l))
	} else {
		data = a.AllocSeg(bitMapByteLength(l))
	}
	m := MakeBitMapWithKnownNiN(l, l, data)
	m.ClearAll()
	return m
}

func bitMapShapesEqual(x, y, z *BitMap) bool {
	return x != nil && y != nil && z != nil && x.length == y.length && x.length == z.length
}

func (m *BitMap) bytes() []byte {
	if m == nil || m.data == nil {
		return nil
	}
	return m.data.AsBytes()[:bitMapByteLength(m.length)]
}

func bitMapByteLength(length int) int {
	return (length >> 3) + min(length&7, 1)
}

func (m *BitMap) maskTail() {
	if m == nil || m.length == 0 || m.data == nil {
		return
	}
	if rem := m.length & 7; rem != 0 {
		data := m.bytes()
		data[len(data)-1] &= byte((1 << rem) - 1)
	}
}

func (m *BitMap) recalcAfterBinary() int {
	m.maskTail()
	count := int(bitop.PopCount(m.bytes()))
	m.nin = m.length - count
	return count
}

func (m *BitMap) cleanRange(start, stop int) (int, int) {
	if start < 0 {
		start = 0
	}
	if stop > m.length {
		stop = m.length
	}
	if stop < start || stop < 0 || start > m.length {
		return 0, 0
	}
	return start, stop
}
