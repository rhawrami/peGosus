package mem

// DataSegment wraps Segment, adding an offset ID.
type DataSegment struct {
	seg   *Segment
	offID uint64
}

// Seg returns the underlying segment.
func (d *DataSegment) Seg() *Segment { return d.seg }

// ID returns the segment's offset ID.
func (d *DataSegment) ID() uint64 { return d.offID }

// MakeData takes in a slice of segments, copies those segments to
// their own slice, and returns a Data object.
func MakeData(s []*Segment) *Data {
	segments := make([]*DataSegment, len(s))
	var length, capacity uint64

	for i := 0; i < len(s); i++ {
		segments[i] = &DataSegment{seg: s[i], offID: uint64(i)}
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
		segments: []*DataSegment{
			{seg: g, offID: 0},
		},
	}
}

// Data represents a set of segments.
type Data struct {
	length   uint64         // length in bytes
	capacity uint64         // max byte capacity
	segments []*DataSegment // set of data segments
}

// Len returns the total length of `d`.
func (d *Data) Len() uint64 { return d.length }

// Cap returns the total capacity of `d`.
func (d *Data) Cap() uint64 { return d.capacity }

// SegmentAt returns the segment at offset `o`.
func (d *Data) SegmentAt(o uint64) *Segment { return d.segments[o].seg }

// updateOffsetIDs updates the offset ID of each segment, ensuring
// IDs correspond to slice positions.
func (d *Data) updateOffsetIDs() {
	for i, v := range d.segments {
		v.offID = uint64(i)
	}
}

// LenProfile returns the length profile of `d`.
func (d *Data) LenProfile() []uint64 {
	sp := make([]uint64, len(d.segments))
	for i := 0; i < len(sp); i++ {
		sp[i] = d.segments[i].seg.length
	}
	return sp
}

// CapProfile returns the capacity profile of `d`.
func (d *Data) CapProfile() []uint64 {
	sp := make([]uint64, len(d.segments))
	for i := 0; i < len(sp); i++ {
		sp[i] = d.segments[i].seg.capacity
	}
	return sp
}

// LenAndCapProfile returns (LenProfile, CapProfile) of `d`.
func (d *Data) LenAndCapProfile() ([]uint64, []uint64) {
	l := len(d.segments)
	lp := make([]uint64, l)
	cp := make([]uint64, l)
	for i := 0; i < l; i++ {
		lp[i] = d.segments[i].seg.length
		cp[i] = d.segments[i].seg.capacity
	}
	return lp, cp
}

// AddSegment adds a segment to `d`; if inc is true, increments the added
// segment by one.
func (d *Data) AddSegment(s *Segment, inc bool) {
	incBy := int64(0)
	if inc {
		incBy = 1
	}
	s.refCount.Add(incBy)

	ds := &DataSegment{seg: s, offID: uint64(len(d.segments))}

	d.length += s.length
	d.capacity += s.capacity
	d.segments = append(d.segments, ds)
}

// DropSegment drops the segment at position `o` from
// `d`; if dec is true, decrements by one.
func (d *Data) DropSegment(o uint64, dec bool) {
	var decBy int64 = 0
	if dec {
		decBy = -1
	}
	g := d.segments[o].seg
	d.length -= g.length
	d.capacity -= g.capacity

	for i := int(o); i < len(d.segments)-1; i++ {
		d.segments[i] = d.segments[i+1]
		d.segments[i].offID -= 1
	}

	d.segments = d.segments[:len(d.segments)-1]
	g.refCount.Add(decBy)
}

// IncAll increments the reference count for each segment.
func (d *Data) IncAll() {
	for _, v := range d.segments {
		v.seg.Inc()
	}
}

// Inc increments the reference count of the segment at
// offset `o`.
func (d *Data) Inc(o uint64) {
	d.segments[o].seg.Inc()
}

// PutAll returns all segments to their corresponding slabs, also resetting
// `d`'s state.
func (d *Data) PutAll() {
	for i := 0; i < len(d.segments); i++ {
		d.segments[i].seg.Put()
	}

	d.capacity = 0
	d.length = 0
	d.segments = d.segments[:0]
}

// DecAll decrements the reference count of all segments; returns false if
// at least one segment now has reference count 0; drops all segments with
// reference count of 0.
func (d *Data) DecAll() bool {
	var yay bool = true

	l := len(d.segments)
	for i := 0; i < l; i++ {
		v := d.segments[i]

		oldLen := v.seg.length
		oldCap := v.seg.capacity
		if safe := v.seg.Dec(); !safe {
			d.length -= oldLen
			d.capacity -= oldCap

			copy(d.segments[i:], d.segments[i+1:])

			d.segments = d.segments[:l-1]
			l -= 1
			yay = false
		}
	}

	d.updateOffsetIDs()
	return yay
}

// Dec decrements the reference count of the segment at
// offset `o`; if the reference count of the segment hits zero,
// the segment is dropped, and returns false.
func (d *Data) Dec(o uint64) bool {
	var yay bool = true

	oldLen := d.segments[o].seg.length
	oldCap := d.segments[o].seg.capacity
	if !d.segments[o].seg.Dec() {
		d.length -= oldLen
		d.capacity -= oldCap

		if int(o) < len(d.segments)-1 {
			copy(d.segments[o:], d.segments[o+1:])
		}

		d.segments = d.segments[:len(d.segments)-1]
		yay = false
	}

	d.updateOffsetIDs()
	return yay
}

// AddLength adds at most `l` bytes of length to segment `o`.
func (d *Data) AddLength(l, o uint64) {
	d.length -= d.segments[o].seg.length
	d.segments[o].seg.AddLength(int(l))
	d.length += d.segments[o].seg.length
}

// SubLength subtracts at most `l` bytes of length to segment `o`.
func (d *Data) SubLength(l, o uint64) {
	d.length -= d.segments[o].seg.length
	d.segments[o].seg.SubLength(int(l))
	d.length += d.segments[o].seg.length
}

// SetLength sets segment `o` to at most `l` bytes of length.
func (d *Data) SetLength(l, o uint64) {
	d.length -= d.segments[o].seg.length
	d.segments[o].seg.SetLength(int(l))
	d.length += d.segments[o].seg.length
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
		v.seg.refCount.Add(a)
		d.segments = append(d.segments, v)
	}

	d.updateOffsetIDs()
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
		source := v.Seg().AsBytes()
		x := copy(target[on:], source)
		on += x
	}
}

// MemSetU8 sets every byte, for each segment, with `x`,
// up to the segment's length; for setting one segment only,
// or applying an offset, call MemSetU8/MemSetU8Detailed on that segment.
func (d *Data) MemSetU8(x uint8) {
	for _, v := range d.segments {
		v.seg.MemSetU8(x)
	}
}

// MemSetU32 sets every four bytes, for each segment, with `x`,
// up to the segment's length; for setting one segment only,
// or applying an offset, call MemSetU32/MemSetU32Detailed on that segment.
func (d *Data) MemSetU32(x uint32) {
	for _, v := range d.segments {
		v.seg.MemSetU32(x)
	}
}

// MemSetU64 sets every eight bytes, for each segment, with `x`,
// up to the segment's length; for setting one segment only,
// or applying an offset, call MemSetU64/MemSetU64Detailed on that segment.
func (d *Data) MemSetU64(x uint64) {
	for _, v := range d.segments {
		v.seg.MemSetU64(x)
	}
}
