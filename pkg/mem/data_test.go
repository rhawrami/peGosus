package mem

import (
	"math/rand/v2"
	"testing"
)

func TestMakeData(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 10)
	g2, _ := s.MakeSegment(slabSize / 10)
	g3, _ := s.MakeSegment(slabSize / 10)
	g4, _ := s.MakeSegment(slabSize / 10)
	g5, _ := s.MakeSegment(slabSize / 10)
	segs := []*Segment{g1, g2, g3, g4, g5}

	var c, l int
	for _, v := range segs {
		l += v.length
		c += v.capacity
	}

	d := MakeData(segs)

	if len(d.segments) != len(segs) {
		t.Errorf("got len %d, expected %d", len(d.segments), len(segs))
	}
	for i := 0; i < len(d.segments); i++ {
		if d.segments[i] != segs[i] {
			t.Errorf("on %d: got %p, expected %p", i, d.segments[i], segs[i])
		}
	}
	if d.Cap() != c {
		t.Errorf("got cap %d, expected %d", d.Cap(), c)
	}
	if d.Len() != l {
		t.Errorf("got len %d, expected %d", d.Len(), l)
	}

}

func TestMakeDataFromSingleSegment(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g, _ := s.MakeSegment(slabSize / 5)

	d := MakeDataFromSingleSegment(g)

	if len(d.segments) != 1 {
		t.Errorf("got len %d, expected 1", len(d.segments))
	}
	if d.segments[0] != g {
		t.Errorf("got seg %p, expected %p", d.segments[0], g)
	}
	if c, l := d.segments[0].capacity, d.segments[0].length; c != g.Cap() || l != g.Len() {
		t.Errorf("got cap %d and len %d, expected %d and %d", c, l, g.Cap(), g.Len())
	}
}

func TestLenProfile(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 5)
	g2, _ := s.MakeSegment(slabSize / 10)
	g3, _ := s.MakeSegment(slabSize / 10)
	g4, _ := s.MakeSegment(slabSize / 10)
	g5, _ := s.MakeSegment(slabSize / 10)

	d1 := MakeDataFromSingleSegment(g1)

	lp := d1.LenProfile()
	for i := 0; i < len(d1.segments); i++ {
		if lp[i] != d1.segments[i].Len() {
			t.Errorf("d1: on %d: got len %d, expected %d", i, lp[i], d1.segments[i].Len())
		}
	}

	segs2 := []*Segment{g2, g3, g4, g5}
	d2 := MakeData(segs2)

	lp = d2.LenProfile()
	for i := 0; i < len(d2.segments); i++ {
		if lp[i] != d2.segments[i].Len() {
			t.Errorf("d2: on %d: got len %d, expected %d", i, lp[i], d2.segments[i].Len())
		}
	}

}

func TestCapProfile(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 5)
	g2, _ := s.MakeSegment(slabSize / 10)
	g3, _ := s.MakeSegment(slabSize / 10)
	g4, _ := s.MakeSegment(slabSize / 10)
	g5, _ := s.MakeSegment(slabSize / 10)

	d1 := MakeDataFromSingleSegment(g1)

	cp := d1.CapProfile()
	for i := 0; i < len(d1.segments); i++ {
		if cp[i] != d1.segments[i].Cap() {
			t.Errorf("d1: on %d: got cap %d, expected %d", i, cp[i], d1.segments[i].Cap())
		}
	}

	segs2 := []*Segment{g2, g3, g4, g5}
	d2 := MakeData(segs2)

	cp = d2.CapProfile()
	for i := 0; i < len(d2.segments); i++ {
		if cp[i] != d2.segments[i].Cap() {
			t.Errorf("d2: on %d: got cap %d, expected %d", i, cp[i], d2.segments[i].Cap())
		}
	}
}

func TestLenAndCapProfile(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 5)
	g2, _ := s.MakeSegment(slabSize / 10)
	g3, _ := s.MakeSegment(slabSize / 10)
	g4, _ := s.MakeSegment(slabSize / 10)
	g5, _ := s.MakeSegment(slabSize / 10)

	d1 := MakeDataFromSingleSegment(g1)

	lp, cp := d1.LenAndCapProfile()
	for i := 0; i < len(d1.segments); i++ {
		if lp[i] != d1.segments[i].Len() {
			t.Errorf("d1: on %d: got len %d, expected %d", i, lp[i], d1.segments[i].Len())
		}
		if cp[i] != d1.segments[i].Cap() {
			t.Errorf("d1: on %d: got cap %d, expected %d", i, cp[i], d1.segments[i].Cap())
		}
	}

	segs2 := []*Segment{g2, g3, g4, g5}
	d2 := MakeData(segs2)

	lp, cp = d2.LenAndCapProfile()
	for i := 0; i < len(d2.segments); i++ {
		if lp[i] != d2.segments[i].Len() {
			t.Errorf("d2: on %d: got len %d, expected %d", i, lp[i], d2.segments[i].Len())
		}
		if cp[i] != d2.segments[i].Cap() {
			t.Errorf("d2: on %d: got cap %d, expected %d", i, cp[i], d2.segments[i].Cap())
		}
	}
}

func TestAddSegment(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 5)
	g2, _ := s.MakeSegment(slabSize / 10)
	d := MakeData([]*Segment{g1, g2})

	g3, _ := s.MakeSegment(slabSize / 10)
	g4, _ := s.MakeSegment(slabSize / 10)
	g5, _ := s.MakeSegment(slabSize / 5)
	for i, v := range []*Segment{g3, g4, g5} {
		cBefore, lBefore := d.Cap(), d.Len()
		rfcBefore := v.RefCount()

		d.AddSegment(v, true)
		if final := d.segments[len(d.segments)-1]; final != v {
			t.Errorf("on %d: got seg %p, expected %p", i, final, v)
		}
		if d.Len() != lBefore+v.Len() {
			t.Errorf("on %d: got len %d, expected %d", i, d.Len(), lBefore+v.Len())
		}
		if d.Cap() != cBefore+v.Cap() {
			t.Errorf("on %d: got cap %d, expected %d", i, d.Cap(), cBefore+v.Cap())
		}
		if v.RefCount() != rfcBefore+1 {
			t.Errorf("on %d: got rfc %d, expected %d", i, v.RefCount(), rfcBefore+1)
		}
	}
}

func TestDropSegment(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 5)
	g2, _ := s.MakeSegment(slabSize / 7)
	g3, _ := s.MakeSegment(slabSize / 10)
	g4, _ := s.MakeSegment(slabSize / 12)
	g5, _ := s.MakeSegment(slabSize / 15)

	d := MakeData([]*Segment{g1, g2, g3, g4, g5})

	// drop g3, don't decrement g3
	lBefore, cBefore := d.Len(), d.Cap()
	ok := d.DropSegment(2, false)
	if !ok {
		t.Error("dropped g3 without dec, but got back false")
	}
	if d.Len() != lBefore-g3.Len() {
		t.Errorf("dropped g3: got len %d, expected %d", d.Len(), lBefore-g3.Len())
	}
	if d.Cap() != cBefore-g3.Cap() {
		t.Errorf("dropped g3: got cap %d, expected %d", d.Cap(), cBefore-g3.Cap())
	}
	if d.SegmentAt(2) != g4 || d.SegmentAt(3) != g5 {
		t.Errorf("dropped g3: did not shift correctly: at 2, got %p, expected %p; at 3, got %p, expected %p",
			d.SegmentAt(2), g4, d.SegmentAt(3), g5,
		)
	}

	// drop g1, decrement g1
	lBefore, cBefore = d.Len(), d.Cap()
	glBefore, gcBefore := g1.Len(), g1.Cap()
	ok = d.DropSegment(0, true)
	if ok {
		t.Error("dropped g1 without dec, but got back true")
	}
	if d.Len() != lBefore-glBefore {
		t.Errorf("dropped g1: got len %d, expected %d", d.Len(), lBefore-glBefore)
	}
	if d.Cap() != cBefore-g1.Cap() {
		t.Errorf("dropped g1: got cap %d, expected %d", d.Cap(), cBefore-gcBefore)
	}
	if d.SegmentAt(0) != g2 || d.SegmentAt(1) != g4 || d.SegmentAt(2) != g5 {
		t.Errorf("dropped g1: did not shift correctly: at 0, got %p, expected %p; at 1, got %p, expected %p; at 2, got %p, expected %p",
			d.SegmentAt(0), g2, d.SegmentAt(1), g4, d.SegmentAt(2), g4,
		)
	}
	if s.holes != 1 || g1.RefCount() != 0 {
		t.Errorf("dropped g1, and decremented: got %d holes, rfc %d, expected %d holes and %d rfc",
			s.holes, 1, g1.RefCount(), 0)
	}
}

func TestDataInc(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 5)
	g2, _ := s.MakeSegment(slabSize / 7)
	g3, _ := s.MakeSegment(slabSize / 10)
	g4, _ := s.MakeSegment(slabSize / 12)
	g5, _ := s.MakeSegment(slabSize / 15)

	d := MakeData([]*Segment{g1, g2, g3, g4, g5})

	// increment g4 (at pos 3)
	rBefore := d.SegmentAt(3).RefCount()
	d.Inc(3)
	if d.SegmentAt(3).RefCount() != rBefore+1 {
		t.Errorf("incremented g4, got rfc %d, expected %d", d.SegmentAt(3).RefCount(), rBefore+1)
	}

	// increment all
	before := make([]int64, len(d.segments))
	for i := 0; i < len(d.segments); i++ {
		before[i] = d.SegmentAt(i).RefCount()
	}
	d.IncAll()
	for i := 0; i < len(d.segments); i++ {
		if d.SegmentAt(i).RefCount() != before[i]+1 {
			t.Errorf("incremented all: on %d, got %d, expected %d", i, d.SegmentAt(i).RefCount(), before[i]+1)
		}
	}
}

func TestDataDec(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 5)
	g2, _ := s.MakeSegment(slabSize / 7)
	g3, _ := s.MakeSegment(slabSize / 10)
	g4, _ := s.MakeSegment(slabSize / 12)
	g5, _ := s.MakeSegment(slabSize / 15)

	d := MakeData([]*Segment{g1, g2, g3, g4, g5})

	// inc g3 first, then dec
	d.Inc(2)
	rBefore := d.SegmentAt(2).RefCount()
	ok := d.Dec(2)
	if !ok {
		t.Error("decd g3 (rfc expected of 2), but got back false")
	}
	if d.SegmentAt(2).RefCount() != rBefore-1 {
		t.Errorf("decd g3, got rfc %d, expected %d", d.SegmentAt(2).RefCount(), rBefore-1)
	}

	// dec g4 => should be dropped
	glBefore, gcBefore := d.SegmentAt(3).Len(), d.SegmentAt(3).Cap()
	lBefore, cBefore := d.Len(), d.Cap()
	ok = d.Dec(3)
	if ok {
		t.Error("decd g4 (rfc expected of 0), but got back true")
	}
	if s.holes != 1 || !s.segments[3].IsFree() {
		t.Errorf("decd g4, got %d holes, expected 1, got IsFree false, expected true", s.holes)
	}
	if d.Len() != lBefore-glBefore {
		t.Errorf("decd g4: got len %d, expected %d", d.Len(), lBefore-glBefore)
	}
	if d.Cap() != cBefore-gcBefore {
		t.Errorf("decd g4: got cap %d, expected %d", d.Cap(), cBefore-gcBefore)
	}
	if d.SegmentAt(3) != g5 {
		t.Errorf("decd g4: at 3, got %p, expected %p", d.SegmentAt(3), g5)
	}

	// data is now {g1, g2, g3, g5}
	// all at rfc 1
	// first: inc g2 and g5
	// then: dec all
	// data should result in {g2, g5}
	// holes should be at 3 now
	d.Inc(1)
	d.Inc(3)
	glBefore, gcBefore = d.SegmentAt(0).Len()+d.SegmentAt(2).Len(), d.SegmentAt(0).Cap()+d.SegmentAt(2).Cap()
	lBefore, cBefore = d.Len(), d.Cap()
	ok = d.DecAll()
	if ok {
		t.Error("decd all , but got back true")
	}
	if d.Len() != lBefore-glBefore {
		t.Errorf("decd all: got len %d, expected %d", d.Len(), lBefore-glBefore)
	}
	if d.Cap() != cBefore-gcBefore {
		t.Errorf("decd all: got cap %d, expected %d", d.Cap(), cBefore-gcBefore)
	}
	if s.holes != 3 {
		t.Errorf("decd all: got %d holes, expected 3", s.holes)
	}

	if d.SegmentAt(0) != g2 || d.SegmentAt(1) != g5 {
		t.Errorf("decd all: got %v, expected {%p, %p}", d.segments, g2, g5)
	}

	// final decrement should drop everything
	ok = d.DecAll()
	if ok {
		t.Error("decd all (final), but got back true")
	}
	if d.Len() != 0 || d.Cap() != 0 {
		t.Errorf("decd all (final): got len/cap %d/%d, expected 0, 0", d.Len(), d.Cap())
	}
	if len(d.segments) != 0 {
		t.Errorf("decd all (final): got len %d, expected 0", len(d.segments))
	}
	if s.used != 0 {
		t.Errorf("decd all (final): got slab used %d, expected 0", s.used)
	}
}

func TestPutAll(t *testing.T) {
	slabSize := 10_000
	s1 := MakeSlab(slabSize)
	s2 := MakeSlab(slabSize)

	// first data
	g1, _ := s1.MakeSegment(slabSize / 5)
	g2, _ := s2.MakeSegment(slabSize / 12)

	// second
	g3, _ := s1.MakeSegment(slabSize / 7)
	g4, _ := s2.MakeSegment(slabSize / 15)

	d1 := MakeData([]*Segment{g1, g2})
	d2 := MakeData([]*Segment{g3, g4})

	d1.PutAll()
	if len(d1.segments) != 0 {
		t.Errorf("put d1: got N seg %d, expected 0", len(d1.segments))
	}
	if d1.Cap() != 0 || d1.Len() != 0 {
		t.Errorf("put d1: got len/cap %d/%d, expected 0, 0", d1.Len(), d1.Cap())
	}
	if s1.holes != 1 || s2.holes != 1 {
		t.Errorf("put d1: got s1/s2 holes %d/%d, expected 1, 1", s1.holes, s2.holes)
	}

	// returning penultimate segments, should keep holes at 1
	d2.PutAll()
	if len(d2.segments) != 0 {
		t.Errorf("put d2: got N seg %d, expected 0", len(d2.segments))
	}
	if d2.Cap() != 0 || d2.Len() != 0 {
		t.Errorf("put d2: got len/cap %d/%d, expected 0, 0", d2.Len(), d2.Cap())
	}
	if s1.holes != 1 || s2.holes != 1 {
		t.Errorf("put d2: got s1/s2 holes %d/%d, expected 1/1", s1.holes, s2.holes)
	}
	if s1.used != 0 || s2.used != 0 {
		t.Errorf("put d2: got s1/s2 used %d/%d, expected 0/0", s1.used, s2.used)
	}
}

func TestDataLength(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 5)
	g2, _ := s.MakeSegment(slabSize / 7)
	g3, _ := s.MakeSegment(slabSize / 10)

	d := MakeData([]*Segment{g1, g2, g3})

	// decrease first segment length by 100
	sub := 10
	lBefore := d.Len()
	glBefore := d.SegmentAt(0).Len()
	d.SubLength(sub, 0)

	if d.Len() != lBefore-sub {
		t.Errorf("subbed %d: got d len %d, expected %d", sub, d.Len(), lBefore-sub)
	}

	if d.SegmentAt(0).Len() != glBefore-sub {
		t.Errorf("subbed %d: got s len %d, expected %d", sub, d.SegmentAt(0).Len(), glBefore-sub)
	}

	// increase first segment length by 50
	add := 50
	lBefore = d.Len()
	glBefore = d.SegmentAt(0).Len()
	d.AddLength(add, 0)

	if d.Len() != lBefore+add {
		t.Errorf("added %d: got d len %d, expected %d", add, d.Len(), lBefore+add)
	}

	if d.SegmentAt(0).Len() != glBefore+add {
		t.Errorf("add %d: got s len %d, expected %d", add, d.SegmentAt(0).Len(), glBefore+add)
	}

	// set second segment length to 250
	setTo := 250
	lBefore = d.Len()
	glBefore = d.SegmentAt(1).Len()
	d.SetLength(setTo, 1)

	if d.Len() != lBefore+(setTo-glBefore) {
		t.Errorf("setTo %d: got d len %d, expected %d", setTo, d.Len(), lBefore+(setTo-glBefore))
	}
	if d.SegmentAt(1).Len() != setTo {
		t.Errorf("set to %d: got s len %d, expected %d", setTo, d.SegmentAt(1).Len(), setTo)
	}

}

func TestDataMerge(t *testing.T) {
	slabSize := 10_000
	s1 := MakeSlab(slabSize)
	s2 := MakeSlab(slabSize)

	g1, _ := s1.MakeSegment(slabSize / 5)
	g2, _ := s2.MakeSegment(slabSize / 7)
	g3, _ := s1.MakeSegment(slabSize / 10)

	g4, _ := s2.MakeSegment(slabSize / 5)
	g5, _ := s1.MakeSegment(slabSize / 7)
	g6, _ := s2.MakeSegment(slabSize / 10)

	d1 := MakeData([]*Segment{g1, g2, g3})
	d2 := MakeData([]*Segment{g4, g5, g6})

	// merge d2 to d1, dont increment
	d1.Merge(d2, false)

	var expectedLen, expectedCap int = 0, 0
	for _, v := range d1.segments {
		expectedLen += v.Len()
		expectedCap += v.Cap()
	}

	if d1.Len() != expectedLen || d1.Cap() != expectedCap {
		t.Errorf("got len/cap %d/%d, expected %d/%d", d1.Len(), expectedLen, d1.Cap(), expectedCap)
	}
	for i, v := range d1.segments {
		if v.RefCount() != 1 {
			t.Errorf("on %d, got rfc %d, expected 1", i, v.RefCount())
		}
	}
	if d1.SegmentAt(3) != d2.SegmentAt(0) || d1.SegmentAt(4) != d2.SegmentAt(1) || d1.SegmentAt(5) != d2.SegmentAt(2) {
		t.Errorf("got seg[3:] %v, expected %v", d1.segments[3:], d2.segments)
	}

	g7, _ := s1.MakeSegment(slabSize / 20)
	d3 := MakeDataFromSingleSegment(g7)

	// merge with increment
	d1.Merge(d3, true)
	if d1.SegmentAt(6).RefCount() != 2 {
		t.Errorf("got rfc %d, expected 2", d1.SegmentAt(6).RefCount())
	}
}

func TestRechunkCopy(t *testing.T) {
	slabSize := 20_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 10)
	g2, _ := s.MakeSegment(slabSize / 15)
	g3, _ := s.MakeSegment(slabSize / 20)
	g4, _ := s.MakeSegment(int(g1.Len() + g2.Len() + g3.Len()))

	src := MakeData([]*Segment{g1, g2, g3})
	dst := MakeDataFromSingleSegment(g4)

	for _, v := range src.segments {
		b := v.AsBytes()
		for i := range b {
			b[i] = uint8(rand.Uint32N(256))
		}
	}

	src.RechunkCopy(dst)

	on := 0
	for i, v := range src.segments {
		bSrc := v.AsBytes()
		bDst := dst.SegmentAt(0).AsBytes()[on : on+v.Len()]
		for j := range bSrc {
			if bDst[j] != bSrc[j] {
				t.Errorf("on seg %d, o %d, got %d, expected %d", i, j, bDst[j], bSrc[j])
			}
		}
		on += v.Len()
	}

	// // this should panic
	g5, _ := s.MakeSegment(int(src.SegmentAt(0).Len() + src.SegmentAt(1).Len()))
	dst2 := MakeDataFromSingleSegment(g5)
	defer func() {
		if r := recover(); r == nil {
			t.Error("did not panic when len was insufficient")
		}
	}()
	src.RechunkCopy(dst2)
}

func TestDataMemSet(t *testing.T) {
	slabSize := 20_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(1024)
	g2, _ := s.MakeSegment(1024 * 2)
	g3, _ := s.MakeSegment(1024 * 3)
	g4, _ := s.MakeSegment(1024 * 4)

	d := MakeData([]*Segment{g1, g2, g3, g4})

	// u8
	a := uint8(rand.Uint32N(256))
	d.MemSetU8(a)
	for i := range d.segments {
		b := d.SegmentAt(i).AsBytes()
		for j := range b {
			if b[j] != a {
				t.Errorf("U8: on seg %d, o %d, got %d, expected %d", i, j, b[j], a)
			}
		}
	}

	// u32
	a2 := rand.Uint32()
	d.MemSetU32(a2)
	for i := range d.segments {
		b := d.SegmentAt(i).AsU32T()
		for j := range b {
			if b[j] != a2 {
				t.Errorf("U32: on seg %d, o %d, got %d, expected %d", i, j, b[j], a2)
			}
		}
	}

	// u64
	a3 := rand.Uint64()
	d.MemSetU64(a3)
	for i := range d.segments {
		b := d.SegmentAt(i).AsU64T()
		for j := range b {
			if b[j] != a3 {
				t.Errorf("U64: on seg %d, o %d, got %d, expected %d", i, j, b[j], a3)
			}
		}
	}
}
