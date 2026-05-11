package mem

import "testing"

func TestMakeData(t *testing.T) {
	slabSize := 10_000
	s := MakeSlab(slabSize)

	g1, _ := s.MakeSegment(slabSize / 10)
	g2, _ := s.MakeSegment(slabSize / 10)
	g3, _ := s.MakeSegment(slabSize / 10)
	g4, _ := s.MakeSegment(slabSize / 10)
	g5, _ := s.MakeSegment(slabSize / 10)
	segs := []*Segment{g1, g2, g3, g4, g5}

	var c, l uint64
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

func TestAddSegment(t *testing.T) {}

func TestDropSegment(t *testing.T) {}

func TestDataInc(t *testing.T) {}

func TestDataDec(t *testing.T) {}

func TestPutAll(t *testing.T) {}

func TestDataLength(t *testing.T) {}

func TestDataMerge(t *testing.T) {}

func TestRechunkCopy(t *testing.T) {}

func TestDataMemSet(t *testing.T) {}
