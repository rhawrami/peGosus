package mem

// MakeData takes in a slice of segments, copies those segments to
// their own slice, and returns a Data object.
func MakeData(s []*Segment) *Data {
	segments := make([]*Segment, len(s))
	var length, capacity int

	for i := 0; i < len(s); i++ {
		segments[i] = s[i]
		length += s[i].length
		capacity += s[i].capacity
	}

	return &Data{
		length:   length,
		capacity: capacity,
		segments: segments,
	}
}

// MakeDataFromSingleSegment returns a Data object with a single segment.
func MakeDataFromSingleSegment(g *Segment) *Data {
	return &Data{
		length:   g.length,
		capacity: g.capacity,
		segments: []*Segment{g},
	}
}

// Data represents a set of segments.
type Data struct {
	length   int        // length in bytes
	capacity int        // max byte capacity
	segments []*Segment // set of segments
}

// Clear sets `d`'s length and capacity to zero, and clears
// the underlying segment set; does not decrement or return
// the segments to their slabs (use PutAll or DecAll for those purposes).
func (d *Data) Clear() {
	d.length = 0
	d.capacity = 0
	d.segments = d.segments[:0]
}

// Len returns the total length of `d`.
func (d *Data) Len() int { return d.length }

// Cap returns the total capacity of `d`.
func (d *Data) Cap() int { return d.capacity }

// SegmentAt returns the segment at offset `o`.
func (d *Data) SegmentAt(o int) *Segment { return d.segments[o] }

// Segments returns the segment set.
func (d *Data) Segments() []*Segment { return d.segments }

// NSegments returns the number of segments that `d` holds.
func (d *Data) NSegments() int { return len(d.segments) }

// LenProfile returns the length profile of `d`.
func (d *Data) LenProfile() []int {
	sp := make([]int, len(d.segments))
	for i := 0; i < len(sp); i++ {
		sp[i] = d.segments[i].Len()
	}
	return sp
}

// CapProfile returns the capacity profile of `d`.
func (d *Data) CapProfile() []int {
	sp := make([]int, len(d.segments))
	for i := 0; i < len(sp); i++ {
		sp[i] = d.segments[i].Cap()
	}
	return sp
}

// LenAndCapProfile returns (LenProfile, CapProfile) of `d`.
func (d *Data) LenAndCapProfile() ([]int, []int) {
	l := len(d.segments)
	lp := make([]int, l)
	cp := make([]int, l)
	for i := 0; i < l; i++ {
		lp[i] = d.segments[i].Len()
		cp[i] = d.segments[i].Cap()
	}
	return lp, cp
}

// AddSegment adds a segment to `d`; if inc is true, increments the added
// segment by one.
func (d *Data) AddSegment(s *Segment, inc bool) {
	d.length += s.length
	d.capacity += s.capacity
	d.segments = append(d.segments, s)

	if inc {
		s.Inc()
	}
}

// DropSegment drops the segment at position `o` from
// `d`; if dec is true, decrements by one; returns false if
// reference count hit zero.
func (d *Data) DropSegment(o int, dec bool) bool {
	g := d.segments[o]
	d.length -= g.length
	d.capacity -= g.capacity

	if o != len(d.segments)-1 {
		copy(d.segments[o:], d.segments[o+1:])
	}
	d.segments = d.segments[:len(d.segments)-1]

	if dec {
		return g.Dec()
	}
	return true
}

// IncAll increments the reference count for each segment.
func (d *Data) IncAll() {
	for _, v := range d.segments {
		v.Inc()
	}
}

// Inc increments the reference count of the segment at
// offset `o`.
func (d *Data) Inc(o int) {
	d.segments[o].Inc()
}

// PutAll returns all segments to their corresponding slabs, also resetting
// `d`'s state.
func (d *Data) PutAll() {
	for i := 0; i < len(d.segments); i++ {
		d.segments[i].Put()
	}

	d.capacity = 0
	d.length = 0
	d.segments = d.segments[:0]
}

// DecAll decrements the reference count of all segments; returns false if
// at least one segment now has reference count 0; drops all segments with
// reference count of 0.
func (d *Data) DecAll() bool {
	yay := true

	l := len(d.segments)
	i := 0
	j := 0
	r := 0

	for j < l {
		incIBy := 1
		incJBy := 1

		v := d.segments[j]
		oldLen := v.length
		oldCap := v.capacity

		if safe := v.Dec(); !safe {
			d.length -= oldLen
			d.capacity -= oldCap
			incIBy = 0
			r += 1
			yay = false

		}
		d.segments[i] = v
		i += incIBy
		j += incJBy

	}

	d.segments = d.segments[:len(d.segments)-r]

	return yay
}

// Dec decrements the reference count of the segment at
// offset `o`; if the reference count of the segment hits zero,
// the segment is dropped, and returns false.
func (d *Data) Dec(o int) bool {
	var yay bool = true

	oldLen := d.segments[o].length
	oldCap := d.segments[o].capacity
	if !d.segments[o].Dec() {
		d.length -= oldLen
		d.capacity -= oldCap

		if o < len(d.segments)-1 {
			copy(d.segments[o:], d.segments[o+1:])
		}

		d.segments = d.segments[:len(d.segments)-1]
		yay = false
	}

	return yay
}

// AddLength adds at most `l` bytes of length to segment `o`.
func (d *Data) AddLength(l, o int) {
	d.length -= d.segments[o].length
	d.segments[o].AddLength(l)
	d.length += d.segments[o].length
}

// SubLength subtracts at most `l` bytes of length to segment `o`.
func (d *Data) SubLength(l, o int) {
	d.length -= d.segments[o].length
	d.segments[o].SubLength(l)
	d.length += d.segments[o].length
}

// SetLength sets segment `o` to at most `l` bytes of length.
func (d *Data) SetLength(l, o int) {
	d.length -= d.segments[o].length
	d.segments[o].SetLength(l)
	d.length += d.segments[o].length
}

// Merge merges `x` to `d`; if inc is true, all segments
// in `x` are incremented.
func (d *Data) Merge(x *Data, inc bool) {
	a := int64(0)
	if inc {
		a = 1
	}
	d.length += x.length
	d.capacity += x.capacity
	for _, v := range x.segments {
		v.refCount.Add(a)
		d.segments = append(d.segments, v)
	}
}

// RechunkCopy copies all underlying data (up to each segment length) from
// `d` to `dst`, where `dst` is data with only one underlying segment;
// panics if `dst` does not have one segment; panics if `dst` does not
// have enough capacity.
func (d *Data) RechunkCopy(dst *Data) {
	if d.length > dst.capacity {
		panic("Rechunk: dst does not have enough capacity")
	}
	if len(dst.segments) != 1 {
		panic("Rechunk: dst does not have only one segment.")
	}

	dst.SetLength(d.length, 0)

	on := 0
	target := dst.SegmentAt(0).AsBytes()
	for _, v := range d.segments {
		source := v.AsBytes()
		x := copy(target[on:], source)
		on += x
	}
}

// MemSetU8 sets every byte, for each segment, with `x`,
// up to the segment's length; for setting one segment only,
// or applying an offset, call MemSetU8/MemSetU8Detailed on that segment.
func (d *Data) MemSetU8(x uint8) {
	for _, v := range d.segments {
		v.MemSetU8(x)
	}
}

// MemSetU32 sets every four bytes, for each segment, with `x`,
// up to the segment's length; for setting one segment only,
// or applying an offset, call MemSetU32/MemSetU32Detailed on that segment.
func (d *Data) MemSetU32(x uint32) {
	for _, v := range d.segments {
		v.MemSetU32(x)
	}
}

// MemSetU64 sets every eight bytes, for each segment, with `x`,
// up to the segment's length; for setting one segment only,
// or applying an offset, call MemSetU64/MemSetU64Detailed on that segment.
func (d *Data) MemSetU64(x uint64) {
	for _, v := range d.segments {
		v.MemSetU64(x)
	}
}
