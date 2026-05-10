package mem

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestMakeSlabSet(t *testing.T) {
	sizeProfile := []uint64{10_000, 20_000, 30_000, 40_000, 50_000}
	ss := MakeSlabSet(sizeProfile)

	if len(ss.slabs) != len(sizeProfile) {
		t.Errorf("got %d segments, expected %d", len(ss.slabs), len(sizeProfile))
	}

	if ss.on != 0 {
		t.Errorf("got on %d, expected 0", ss.on)
	}

	var trueCap uint64
	for i, v := range ss.slabs {
		if v.capacity < sizeProfile[i] {
			t.Errorf("on %d, got %d cap < %d cap", i, v.capacity, sizeProfile[i])
		}
		trueCap += v.capacity
	}

	if ss.capacity != trueCap {
		t.Errorf("got max cap %d, expected %d", ss.capacity, trueCap)
	}
}

func TestSlabSetClear(t *testing.T) {
	sizeProfile := []uint64{10_000, 20_000, 30_000, 40_000}
	ss := MakeSlabSet(sizeProfile)

	for _, v := range ss.slabs {
		_, _ = v.MakeSegment(1_000)
		_, _ = v.MakeSegment(3_000)
		g, _ := v.MakeSegment(1_000)
		_, _ = v.MakeSegment(1_000)
		g.Put()
	}

	ss.Clear()

	for i, v := range ss.slabs {
		if len(v.segments) != 1 {
			t.Errorf("on %d, got seg N %d, expected 1", i, len(v.segments))
		}
		if v.used != 0 {
			t.Errorf("on %d, got used %d, expected 0", i, v.used)
		}
		if v.base != v.on || v.base != v.segments[len(v.segments)-1].base {
			t.Errorf("on %d, got base %p, on %p, edge base %p", i, v.base, v.on, v.segments[len(v.segments)-1].base)
		}
		if v.holes != 0 {
			t.Errorf("on %d, got holes %d, expected 0", i, v.holes)
		}
	}
}

func TestSlabSetGrow(t *testing.T) {
	sizeProfile := []uint64{10_000, 20_000, 30_000, 40_000}
	ss := MakeSlabSet(sizeProfile)

	oldCap := ss.capacity
	slab0Cap := ss.slabs[0].capacity

	ss.Grow()

	if len(ss.slabs) != len(sizeProfile)+1 {
		t.Errorf("got N segs %d, expected %d", len(ss.slabs), len(sizeProfile)+1)
	}
	if ss.capacity != slab0Cap+oldCap {
		t.Errorf("got cap %d, expected %d", ss.capacity, slab0Cap+oldCap)
	}
	if ss.slabs[len(ss.slabs)-1].capacity != slab0Cap {
		t.Errorf("got appended slab cap %d, expected %d", ss.slabs[len(ss.slabs)-1].capacity, slab0Cap)
	}
}

func TestSlabSetGrowWithSize(t *testing.T) {
	sizeProfile := []uint64{10_000, 20_000, 30_000, 40_000}
	ss := MakeSlabSet(sizeProfile)

	oldCap := ss.capacity

	s := uint64(49984)
	ss.GrowWithSize(int(s))

	if len(ss.slabs) != len(sizeProfile)+1 {
		t.Errorf("got N segs %d, expected %d", len(ss.slabs), len(sizeProfile)+1)
	}
	if ss.capacity != oldCap+s {
		t.Errorf("got cap %d, expected %d", ss.capacity, s+oldCap)
	}
	if ss.slabs[len(ss.slabs)-1].capacity != s {
		t.Errorf("got appended slab cap %d, expected %d", ss.slabs[len(ss.slabs)-1].capacity, s)
	}
	if ss.on != len(ss.slabs)-1 {
		t.Errorf("got on %d, expected %d", ss.on, len(ss.slabs))
	}
}

func TestSlabSetAccept(t *testing.T) {
	sizeProfile := []uint64{10_000, 20_000, 30_000, 40_000}
	ss := MakeSlabSet(sizeProfile)
	b := MakeSlab(10_000)

	oldCap := ss.capacity
	oldOn := ss.on
	oldLen := len(ss.slabs)
	// this slab does not have the largest (cap - used), so "on"
	// should stay the same
	ss.Accept(b)
	if ss.capacity != oldCap+b.capacity {
		t.Errorf("op 1: got cap %d, expected %d", ss.capacity, oldCap+b.capacity)
	}
	if len(ss.slabs) != oldLen+1 {
		t.Errorf("op 1: got N slabs %d, expected %d", len(ss.slabs), oldLen+1)
	}
	if ss.on != oldOn {
		t.Errorf("op 1: got on %d, expected %d", ss.on, oldOn)
	}
	if ss.slabs[len(ss.slabs)-1] != b {
		t.Errorf("op 1: got app slab %p, expected %p", ss.slabs[len(ss.slabs)-1], b)
	}

	b2 := MakeSlab(50_000)
	// should change "on"
	ss.Accept(b2)
	if ss.on != len(ss.slabs)-1 {
		t.Errorf("op 2: got on %d, expected %d", ss.on, len(ss.slabs)-1)
	}
}

func TestSlabSetRemove(t *testing.T) {
	sizeProfile := []uint64{10_000, 20_000, 30_000, 40_000}
	ss := MakeSlabSet(sizeProfile)

	remO := 0
	slab0 := ss.slabs[remO]
	oldCap := ss.capacity
	oldLen := len(ss.slabs)

	// after removing slab 0, "on" should be set to the final slab (slab 2)
	ss.Remove(remO)
	if len(ss.slabs) != oldLen-1 {
		t.Errorf("op 1: got slab N %d, expected %d", len(ss.slabs), oldLen-1)
	}
	if ss.capacity != oldCap-slab0.capacity {
		t.Errorf("op 1: got cap %d, expected %d", ss.capacity, oldCap-slab0.capacity)
	}
	if ss.on != len(ss.slabs)-1 {
		t.Errorf("op 1: got on %d, expected %d", ss.on, len(ss.slabs)-1)
	}

	// more complex test
	N := 100
	sizeProfile = make([]uint64, N)
	for i := range N {
		s := rand.Uint64N(50_000)
		sizeProfile[i] = s
	}
	ss = MakeSlabSet(sizeProfile)
	for i := range N {
		if i%3 == 0 {
			_, _ = ss.slabs[i].MakeSegment(int(ss.slabs[i].capacity) / 3)
		}
	}

	N2 := 25
	remd := make([]uint64, 0, N2)
	for range N2 {
		for {
			r := rand.Uint64N(uint64(N - N2))
			if !slices.Contains(remd, r) {
				remd = append(remd, r)

				slabRem := ss.slabs[r]
				oldCap := ss.capacity
				oldLen := len(ss.slabs)

				ss.Remove(int(r))
				if len(ss.slabs) != oldLen-1 {
					t.Errorf("op 2: got len %d, expected %d", len(ss.slabs), oldLen-1)
				}
				if ss.capacity != oldCap-slabRem.capacity {
					t.Errorf("op 2: got cap %d, expected %d", ss.capacity, oldCap-slabRem.capacity)
				}

				o := 0
				diff := uint64(0)
				for i, v := range ss.slabs {
					if d := v.capacity - v.used; d > diff {
						o = i
						diff = d
					}
				}

				if ss.on != o {
					t.Errorf("op 2: got on %d, expected %d", ss.on, o)
				}

				break
			}
		}
	}

}

func TestSlabSetSetOn(t *testing.T) {
	sizeProfile := []uint64{10_000, 20_000, 30_000, 40_000}
	ss := MakeSlabSet(sizeProfile)

	// should set to final slab
	ss.SetOn()
	if ss.on != len(ss.slabs)-1 {
		t.Errorf("first SetOn: got on %d, expected %d", ss.on, len(ss.slabs)-1)
	}

	// should now be set to penultimate slab
	_, _ = ss.slabs[len(ss.slabs)-1].MakeSegment(int(sizeProfile[len(sizeProfile)-1] >> 1))
	ss.SetOn()
	if ss.on != len(ss.slabs)-2 {
		t.Errorf("second SetOn: got on %d, expected %d", ss.on, len(ss.slabs)-2)
	}

	// should go back to final slab
	ss.slabs[len(ss.slabs)-1].Clear()
	ss.SetOn()
	if ss.on != len(ss.slabs)-1 {
		t.Errorf("third SetOn: got on %d, expected %d", ss.on, len(ss.slabs)-1)
	}
}

func TestSlabSetMakeSegment(t *testing.T) {
	s := uint64(10_000)
	sizeProfile := []uint64{s, s, s, s}
	ss := MakeSlabSet(sizeProfile)

	// should be able to make these segments, all from slab 0
	for i := range 4 {
		oldUsed := ss.slabs[0].used
		g, ok := ss.MakeSegment(int(s / 5))
		if !ok {
			t.Errorf("call 1: on %d: shouldve been able to make seg, but couldnt", i)
		}
		if ss.slabs[0].used != oldUsed+g.capacity {
			t.Errorf("call 1: on %d: got used %d, expected %d", i, ss.slabs[0].used, oldUsed-g.capacity)
		}
	}

	// should pull from slab 1
	g, _ := ss.MakeSegment(int(s))
	if ss.slabs[1].used != g.capacity {
		t.Errorf("call 2: got used %d, expected %d", ss.slabs[1].used, g.capacity)
	}

	// should pull from slab 2
	g, _ = ss.MakeSegment(int(s))
	if ss.slabs[2].used != g.capacity {
		t.Errorf("call 3: got used %d, expected %d", ss.slabs[2].used, g.capacity)
	}

	// should pull from slab 3
	g, _ = ss.MakeSegment(int(s))
	if ss.slabs[3].used != g.capacity {
		t.Errorf("call 4: got used %d, expected %d", ss.slabs[3].used, g.capacity)
	}

	// should fail to make segment
	g, ok := ss.MakeSegment(int(s))
	if g != nil || ok {
		t.Errorf("call 5: shouldve failed, but succeeded")
	}

	ss.Clear()
	// should pull from slab 0
	g, _ = ss.MakeSegment(int(s))
	if ss.slabs[0].used != g.capacity {
		t.Errorf("call 6: got used %d, expected %d", ss.slabs[0].used, g.capacity)
	}
}

func TestSlabSetForceSegment(t *testing.T) {
	s := uint64(10_000)
	sizeProfile := []uint64{s, s, s, s}
	ss := MakeSlabSet(sizeProfile)

	_ = ss.ForceSegment(int(s))
	_ = ss.ForceSegment(int(s))
	_ = ss.ForceSegment(int(s))
	_ = ss.ForceSegment(int(s))

	// this should fail
	g, ok := ss.MakeSegment(int(s))
	if g != nil || ok {
		t.Errorf("call shouldve failed, but succeeded")
	}

	oldLen := len(ss.slabs)
	oldCap := ss.capacity

	g = ss.ForceSegment(int(s))
	if len(ss.slabs) != oldLen+1 {
		t.Errorf("got slab N %d, expected %d", len(ss.slabs), oldLen+1)
	}
	if ss.capacity != oldCap+(ss.slabs[len(ss.slabs)-1].capacity) {
		t.Errorf("got cap %d, expected %d", ss.capacity, oldCap+(ss.slabs[len(ss.slabs)-1].capacity))
	}
	if ss.slabs[len(ss.slabs)-1].capacity < s {
		t.Errorf("got final slab cap %d < %d", ss.slabs[len(ss.slabs)-1].capacity, s)
	}
	if ss.slabs[len(ss.slabs)-1].used != g.capacity {
		t.Errorf("got final slab used %d, expected %d", ss.slabs[len(ss.slabs)-1].used, g.capacity)
	}
}

func TestSlabSetGrowAndMakeSegment(t *testing.T) {
	s := uint64(10_000)
	sizeProfile := []uint64{s, s, s, s}
	ss := MakeSlabSet(sizeProfile)

	oldLen := len(ss.slabs)
	oldCap := ss.capacity

	req := 20_000
	g := ss.GrowAndMakeSegment(req)
	if len(ss.slabs) != oldLen+1 {
		t.Errorf("got slab len %d, expected %d", len(ss.slabs), oldLen+1)
	}
	if ss.capacity != oldCap+ss.slabs[len(ss.slabs)-1].capacity {
		t.Errorf("got cap %d, expected %d", ss.capacity, oldCap+ss.slabs[len(ss.slabs)-1].capacity)
	}
	if ss.slabs[len(ss.slabs)-1].used != g.capacity {
		t.Errorf("got final slab used %d, expected %d", ss.slabs[len(ss.slabs)-1].used, g.capacity)
	}
	if ss.on != len(ss.slabs)-1 {
		t.Errorf("got on %d, expected %d", ss.on, len(ss.slabs)-1)
	}
}

func TestSlabSetOptimize(t *testing.T) {}

func TestTransferSlab(t *testing.T) {}

func TestTransferSlabWithOffset(t *testing.T) {}
