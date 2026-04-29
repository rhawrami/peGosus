package mem

// DataSegment wraps Segment, adding an offset ID.
type DataSegment struct {
	seg   *Segment
	offID uint64
}

// Seg returns the underlying segment.
func (d *DataSegment) Seg() *Segment { return d.seg }

// ID returns the segments offset ID.
func (d *DataSegment) ID() uint64 { return d.offID }

// MakeData takes in a slice of segments, copies those segments to
// its own slice, and returns a Data object.
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

// SizeProfile represents either a set of data segment
// lengths or capacities.
type SizeProfile struct {
	p []uint64
}

// AsSlice returns the profile as a slice.
func (sp SizeProfile) AsSlice() []uint64 { return sp.p }

// Data represents a set of segments.
type Data struct {
	length   uint64         // length in bytes
	capacity uint64         // max byte capacity
	segments []*DataSegment // set of data segments
}

// updateOffsetIDs updates the offset ID of each segment, ensuring
// IDs correspond to slice positions.
func (d *Data) updateOffsetIDs() {
	for i, v := range d.segments {
		v.offID = uint64(i)
	}
}

// LenProfile returns the length profile of `d`.
func (d *Data) LenProfile() SizeProfile {
	sp := make([]uint64, len(d.segments))
	for i := 0; i < len(d.segments); i++ {
		sp[i] = d.segments[i].seg.length
	}
	return SizeProfile{p: sp}
}

// CapProfile returns the length profile of `d`.
func (d *Data) CapProfile() SizeProfile {
	sp := make([]uint64, len(d.segments))
	for i := 0; i < len(d.segments); i++ {
		sp[i] = d.segments[i].seg.capacity
	}
	return SizeProfile{p: sp}
}

// Profiles returns (LenProfile, CapProfile) of `d`.
func (d *Data) Profiles() (SizeProfile, SizeProfile) {
	lp := make([]uint64, len(d.segments))
	cp := make([]uint64, len(d.segments))
	for i := 0; i < len(d.segments); i++ {
		lp[i] = d.segments[i].seg.length
		cp[i] = d.segments[i].seg.capacity
	}
	return SizeProfile{p: lp}, SizeProfile{p: cp}
}

// AddSegment adds a segment to `d`.
func (d *Data) AddSegment(s *Segment) {
	ds := &DataSegment{seg: s, offID: uint64(len(d.segments))}

	d.length += s.length
	d.capacity += s.capacity
	d.segments = append(d.segments, ds)
}

// IncAll increments the reference count on all segments.
func (d *Data) IncAll() {
	for i := 0; i < len(d.segments); i++ {
		d.segments[i].seg.Inc()
	}
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
// at least one segment now has reference count 0.
func (d *Data) DecAll() bool {
	var yay bool = true

	var l int
	for i := 0; i < l; i++ {
		v := d.segments[i]
		if safe := v.seg.Dec(); !safe {
			d.length -= v.seg.length
			d.capacity -= v.seg.capacity
			if i < (l - 1) {
				for j := i; j < l; j++ {
					d.segments[j] = d.segments[j+1]
				}
			}
			d.segments = d.segments[:len(d.segments)-1]
			l -= 1
			yay = false
		}
	}

	d.updateOffsetIDs()
	return yay
}

// Dec decrements the reference count of the segment at
// offset `o`; returns false if the reference count of that
// segment hits zero.
func (d *Data) Dec(o uint64) bool {
	var yay bool = true
	if !d.segments[o].seg.Dec() {
		d.length -= d.segments[o].seg.length
		d.capacity -= d.segments[o].seg.capacity

		if int(o) < len(d.segments)-1 {
			for i := int(o); i < len(d.segments); i++ {
				d.segments[i] = d.segments[i+1]
			}
		}
		d.segments = d.segments[:len(d.segments)-1]
		yay = false
	}

	d.updateOffsetIDs()
	return yay
}

// AddLength adds `l` bytes of length to segment `o`.
func (d *Data) AddLength(l, o uint64) {
	d.segments[o].seg.AddLength(int(l))
	d.length += l
}

// SubLength subtracts `l` bytes of length to segment `o`.
func (d *Data) SubLength(l, o uint64) {
	d.segments[o].seg.SubLength(int(l))
	d.length -= l
}
