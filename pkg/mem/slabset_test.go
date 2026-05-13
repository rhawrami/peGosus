package mem

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestMakeSlabSet(t *testing.T) {
	sizeProfile := []int{10_000, 20_000, 30_000, 40_000, 50_000}
	ss := MakeSlabSet(sizeProfile)

	if len(ss.slabs) != len(sizeProfile) {
		t.Errorf("got %d segments, expected %d", len(ss.slabs), len(sizeProfile))
	}

	if ss.on != 0 {
		t.Errorf("got on %d, expected 0", ss.on)
	}

	var trueCap int
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
	sizeProfile := []int{10_000, 20_000, 30_000, 40_000}
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
	sizeProfile := []int{10_000, 20_000, 30_000, 40_000}
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
	sizeProfile := []int{10_000, 20_000, 30_000, 40_000}
	ss := MakeSlabSet(sizeProfile)

	oldCap := ss.capacity

	s := int(49984)
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
	sizeProfile := []int{10_000, 20_000, 30_000, 40_000}
	ss := MakeSlabSet(sizeProfile)
	b := MakeSlab(10_000)

	oldCap := ss.capacity
	oldLen := len(ss.slabs)
	// this slab does not have the largest (cap - used), so "on"
	// should update to the penultimate slab
	ss.Accept(b)
	if ss.capacity != oldCap+b.capacity {
		t.Errorf("op 1: got cap %d, expected %d", ss.capacity, oldCap+b.capacity)
	}
	if len(ss.slabs) != oldLen+1 {
		t.Errorf("op 1: got N slabs %d, expected %d", len(ss.slabs), oldLen+1)
	}
	if ss.on != 3 {
		t.Errorf("op 1: got on %d, expected %d", ss.on, 3)
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
	sizeProfile := []int{10_000, 20_000, 30_000, 40_000}
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
	sizeProfile = make([]int, N)
	for i := range N {
		s := rand.IntN(50_000)
		sizeProfile[i] = s
	}
	ss = MakeSlabSet(sizeProfile)
	for i := range N {
		if i%3 == 0 {
			_, _ = ss.slabs[i].MakeSegment(int(ss.slabs[i].capacity) / 3)
		}
	}

	N2 := 25
	remd := make([]int, 0, N2)
	for range N2 {
		for {
			r := rand.IntN(int(N - N2))
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
				diff := int(0)
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
	sizeProfile := []int{10_000, 20_000, 30_000, 40_000}
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
	s := int(10_000)
	sizeProfile := []int{s, s, s, s}
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
	s := int(10_000)
	sizeProfile := []int{s, s, s, s}
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
	s := int(10_000)
	sizeProfile := []int{s, s, s, s}
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

func TestTransferSlab(t *testing.T) {
	s := int(10_000)
	sizeProfileA := []int{s, s, s, s}
	sizeProfileB := []int{s * 2, s * 2, s * 2, s * 2}
	ssA := MakeSlabSet(sizeProfileA)
	ssB := MakeSlabSet(sizeProfileB)

	// fill up ssA
	var ok bool = true
	for ok {
		_, ok = ssA.MakeSegment(int(s))
	}

	onSSA, lenSSA, capSSA := ssA.on, len(ssA.slabs), ssA.capacity
	onSSB, lenSSB, capSSB := ssB.on, len(ssB.slabs), ssB.capacity
	// this should do nothing, as ssA has no open slabs
	TransferSlab(ssB, ssA)
	if ssA.on != onSSA || len(ssA.slabs) != lenSSA || ssA.capacity != capSSA {
		t.Errorf("call 1 (ssA): on/len/cap %d/%d/%d != %d/%d/%d",
			ssA.on, onSSA, len(ssA.slabs), lenSSA, ssA.capacity, capSSA,
		)
	}
	if ssB.on != onSSB || len(ssB.slabs) != lenSSB || ssB.capacity != capSSB {
		t.Errorf("call 1 (ssB): on/len/cap %d/%d/%d != %d/%d/%d",
			ssB.on, onSSB, len(ssB.slabs), lenSSB, ssB.capacity, capSSB,
		)
	}

	// if we transfer B to A
	// 1. the first slab in B should be transferred to A
	// 2. B.on should update to the second slab (e.g., stay at 0)
	// 3. A.on should update to the final slab
	// 4. A|B caps should appropriately update
	slab0 := ssB.slabs[0]

	TransferSlab(ssA, ssB)
	if ssA.slabs[len(ssA.slabs)-1] != slab0 {
		t.Errorf("call 2: got final slab %p, expected %p", ssA.slabs[len(ssA.slabs)-1], slab0)
	}
	if ssB.slabs[0] == slab0 {
		t.Errorf("call 2: ssB transferred slab 0, but stayed in set")
	}
	if ssA.on != len(ssA.slabs)-1 {
		t.Errorf("call 2: got ssA on %d, expected %d", ssA.on, len(ssA.slabs)-1)
	}
	if ssB.on != 0 {
		t.Errorf("call 2: got ssB on %d, expected %d", ssB.on, 0)
	}
	if ssA.capacity != capSSA+slab0.capacity || ssB.capacity != capSSB-slab0.capacity {
		t.Errorf("call 2: got ssA cap %d expected %d; got ssB cap %d expected %d",
			ssA.capacity, capSSA+slab0.capacity, ssB.capacity, capSSB-slab0.capacity,
		)
	}
}

func TestTransferSlabWithOffset(t *testing.T) {
	s := int(10_000)
	sizeProfileA := []int{s, s, s, s}
	sizeProfileB := []int{s * 2, s * 2, s * 2, s * 2}
	ssA := MakeSlabSet(sizeProfileA)
	ssB := MakeSlabSet(sizeProfileB)

	// fill up ssA, except at 1
	_, _ = ssA.MakeSegment(int(s))
	_, _ = ssA.MakeSegment(int(s))
	_, _ = ssA.MakeSegment(int(s))
	_, _ = ssA.MakeSegment(int(s))
	ssA.slabs[1].Clear()

	onSSA, lenSSA, capSSA := ssA.on, len(ssA.slabs), ssA.capacity
	onSSB, lenSSB, capSSB := ssB.on, len(ssB.slabs), ssB.capacity
	// this should do nothing, as ssA at 2 is not open
	TransferSlabWithOffset(ssB, ssA, 2)
	if ssA.on != onSSA || len(ssA.slabs) != lenSSA || ssA.capacity != capSSA {
		t.Errorf("call 1 (ssA): on/len/cap %d/%d/%d != %d/%d/%d",
			ssA.on, onSSA, len(ssA.slabs), lenSSA, ssA.capacity, capSSA,
		)
	}
	if ssB.on != onSSB || len(ssB.slabs) != lenSSB || ssB.capacity != capSSB {
		t.Errorf("call 1 (ssB): on/len/cap %d/%d/%d != %d/%d/%d",
			ssB.on, onSSB, len(ssB.slabs), lenSSB, ssB.capacity, capSSB,
		)
	}

	// fill up ssB, except at 2 and 3
	_, _ = ssB.MakeSegment(int(s))
	_, _ = ssB.MakeSegment(int(s))
	_, _ = ssB.MakeSegment(int(s))
	_, _ = ssB.MakeSegment(int(s))
	ssB.slabs[2].Clear()
	ssB.slabs[3].Clear()

	// if we transfer B to A at 2
	// 1. the third slab in B should be transferred to A
	// 2. B.on should update to the final slab
	// 3. A.on should update to the final slab
	// 4. A|B caps should appropriately update
	slab2 := ssB.slabs[2]

	TransferSlabWithOffset(ssA, ssB, 2)
	if ssA.slabs[len(ssA.slabs)-1] != slab2 {
		t.Errorf("call 2: got final slab %p, expected %p", ssA.slabs[len(ssA.slabs)-1], slab2)
	}
	if ssB.slabs[0] == slab2 {
		t.Errorf("call 2: ssB transferred slab 0, but stayed in set")
	}
	if ssA.on != len(ssA.slabs)-1 {
		t.Errorf("call 2: got ssA on %d, expected %d", ssA.on, len(ssA.slabs)-1)
	}
	if ssB.on != len(ssB.slabs)-1 {
		t.Errorf("call 2: got ssB on %d, expected %d", ssB.on, len(ssB.slabs)-1)
	}
	if ssA.capacity != capSSA+slab2.capacity || ssB.capacity != capSSB-slab2.capacity {
		t.Errorf("call 2: got ssA cap %d expected %d; got ssB cap %d expected %d",
			ssA.capacity, capSSA+slab2.capacity, ssB.capacity, capSSB-slab2.capacity,
		)
	}
}

func TestSlabSetOptimize(t *testing.T) {
	s := int(10_000)
	sizeProfile := []int{s, s, s}
	ss := MakeSlabSet(sizeProfile)

	a, b, c := ss.slabs[0], ss.slabs[1], ss.slabs[2]

	req := 500

	// take up some space in a
	g1, _ := a.MakeSegment(req)
	g2, _ := a.MakeSegment(req)
	g3, _ := a.MakeSegment(req)
	_, _ = a.MakeSegment(req)

	// take up some space in b
	_, _ = b.MakeSegment(req)
	_, _ = b.MakeSegment(req)

	// take up some space in c
	_, _ = c.MakeSegment(req)
	_, _ = c.MakeSegment(req)
	_, _ = c.MakeSegment(req)

	// after optimization, on should be set to b
	ss.Optimize()
	if ss.on != 1 || ss.slabs[ss.on] != b {
		t.Errorf("call 1: got on %d, expected %d, got slab %p, expected %p",
			ss.on, 1, ss.slabs[ss.on], b,
		)
	}

	// free 3 adjacent segments in a
	g1.Put()
	g2.Put()
	g3.Put()

	// afte optimization
	// 1. on should be set to a
	// 2. holes in a should be reduced by 2
	// 3. a's first segment should have triple the prior size
	h := a.holes
	capOld := a.segments[0].capacity
	ss.Optimize()
	if ss.on != 0 || ss.slabs[ss.on] != a {
		t.Errorf("call 2: got on %d, expected %d, got slab %p, expected %p",
			ss.on, 0, ss.slabs[ss.on], a,
		)
	}
	if a.holes != h-2 {
		t.Errorf("call 2: in a, got %d holes, expected %d", a.holes, h-2)
	}
	if a.segments[0].capacity != capOld*3 {
		t.Errorf("call 2: got seg 0 cap %d, expected %d", a.segments[0].capacity, capOld*3)
	}

}
