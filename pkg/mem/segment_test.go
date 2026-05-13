package mem

import (
	"math/rand/v2"
	"testing"
)

func TestMakeSegment(t *testing.T) {
	slabSizeReq := 100_000
	segSizeReq := 20_000

	s := MakeSlab(slabSizeReq)
	g1, ok := s.MakeSegment(segSizeReq)

	if !ok {
		t.Errorf("was not able to make segment with ample space in slab")
	}
	if g1.base != s.base {
		t.Errorf("seg 0 base %p != slab base %p", g1.base, s.base)
	}
	if g1.length != int(segSizeReq) {
		t.Errorf("requested %d, got len of %d", g1.length, segSizeReq)
	}
	if g1.capacity < int(segSizeReq) {
		t.Errorf("seg cap %d < req %d", g1.capacity, segSizeReq)
	}
	if g1.refCount.Load() != 1 {
		t.Errorf("got rfc %d, expected 1", g1.refCount.Load())
	}

	g2, ok := s.MakeSegment(slabSizeReq * 2)
	if g2 != nil || ok {
		t.Errorf("slab should not have ample space, but got back valid segment")
	}
}

func TestChangeLength(t *testing.T) {
	slabSizeReq := 100_000
	segSizeReq := 20_000

	s := MakeSlab(slabSizeReq)
	g1, _ := s.MakeSegment(segSizeReq)

	g1.SetLength(0)
	if g1.length != 0 {
		t.Errorf("set length to 0, but got %d", g1.length)
	}

	// on length setting greater than cap, should just round down to cap
	g1.SetLength(segSizeReq * 2)
	if g1.length != g1.capacity {
		t.Errorf("set length to too large val, got %d, expected %d", g1.length, g1.capacity)
	}

	g1.SetLength(0)
	// when subbing length below 0, should stay at 0
	g1.SubLength(1)
	if g1.length != 0 {
		t.Errorf("tried to sub length below 0, got length %d, expected 0", g1.length)
	}
	// when adding length above cap, should stay at cap
	g1.AddLength(segSizeReq * 2)
	if g1.length != g1.capacity {
		t.Errorf("tried to add length above 0, got %d, expected %d", g1.length, g1.capacity)
	}

	g1.SetLength(0)
	finalCtr := 0
	// at max, adding 10k to length, so should be valid
	for range 100 {
		inc := rand.IntN(100)
		g1.AddLength(inc)
		finalCtr += inc
	}
	if finalCtr != int(g1.length) {
		t.Errorf("addlen iters: got %d, expected %d", g1.length, finalCtr)
	}

	g1.SetLengthToCap()
	finalCtr = int(g1.length)
	for range 100 {
		dec := rand.IntN(100)
		g1.SubLength(dec)
		finalCtr -= dec
	}
	if finalCtr != int(g1.length) {
		t.Errorf("sublen iters: got %d, expected %d", g1.length, finalCtr)
	}

}

func TestDecrement(t *testing.T) {
	slabSizeReq := 100_000
	segSizeReq := 20_000

	s := MakeSlab(slabSizeReq)
	g1, _ := s.MakeSegment(segSizeReq)

	ok := g1.Dec()
	if ok {
		t.Error("Decd g1 to 0, but returned true")
	}
	if s.used != 0 {
		t.Errorf("Decd only seg to 0, but got slab used %d, expected 0", s.used)
	}
	if s.holes != 0 {
		t.Errorf("Decd only seg to 0, but got slab holes %d, expected 0", s.holes)
	}
	if s.on != s.base || s.segments[len(s.segments)-1].base != s.base {
		t.Errorf("Decd only seg to 0, but got slab on %p, edge base %p, expected %p",
			s.on, s.segments[len(s.segments)-1].base, s.base)
	}
	if len(s.segments) != 1 {
		t.Errorf("Decd only seg to 0, but got %d segments, expected 1", len(s.segments))
	}
	// decd seg shouldve technically become edge after penultimate merging
	if g1 != s.segments[len(s.segments)-1] {
		t.Errorf("Decd only seg to 0, seg didnt become edge")
	}

	g2, _ := s.MakeSegment(segSizeReq)

	finalCtr := 1
	for i := range 100 {
		if i%7 == 0 {
			finalCtr -= 1
			_ = g2.Dec()
		} else {
			finalCtr += 1
			g2.Inc()
		}
	}
	if int(g2.refCount.Load()) != finalCtr {
		t.Errorf("inc/dec iters: got %d, expected %d", g2.refCount.Load(), finalCtr)
	}
}

func TestPut(t *testing.T) {
	slabSizeReq := 100_000
	segSizeReq := 20_000

	s := MakeSlab(slabSizeReq)
	g1, _ := s.MakeSegment(segSizeReq)
	_, _ = s.MakeSegment(segSizeReq)
	g3, _ := s.MakeSegment(segSizeReq)

	oldUsed := s.used
	capG1 := g1.capacity
	g1.Put()
	if g1.refCount.Load() != 0 {
		t.Errorf("put g1 back, got rfc %d, expected 0", g1.refCount.Load())
	}
	if s.holes != 1 {
		t.Errorf("returned g1, got %d holes, expected 1", s.holes)
	}
	if newUsed := s.used; newUsed != oldUsed-capG1 {
		t.Errorf("returned g1, got used %d, expected %d", newUsed, oldUsed-capG1)
	}

	oldEdgeCap := s.segments[len(s.segments)-1].capacity
	capG3 := g3.capacity
	g3.Put()
	if s.holes != 1 {
		t.Errorf("returned penultimate seg: got %d holes, expected 1", s.holes)
	}
	if newCap := s.segments[len(s.segments)-1].capacity; newCap != oldEdgeCap+capG3 {
		t.Errorf("returned penultimate segment, got edge cap %d, expected %d", newCap, oldEdgeCap+capG3)
	}

}

func TestMemSetU8(t *testing.T) {
	slabSizeReq := 100_000
	segSizeReq := 20_000

	s := MakeSlab(slabSizeReq)
	g1, _ := s.MakeSegment(segSizeReq)

	setTo := uint8(rand.Uint32N(256))
	g1.MemSetU8(setTo)

	for i, v := range g1.AsBytes() {
		if v != setTo {
			t.Errorf("on %d: got %d, expected %d", i, v, setTo)
		}
	}

	// safety check to make sure slab buff also has value set
	overlap := s.buff[0:g1.length]
	for i, v := range overlap {
		if v != setTo {
			t.Errorf("slab base: on %d: got %d, expected %d", i, v, setTo)
		}
	}

	g1.MemSetU8(0)
	setTo = uint8(rand.Uint32N(256))
	// add 10 byte offset, and set 100 bytes
	o := int(10)
	setOnly := int(100)
	g1.MemSetU8Detailed(setTo, setOnly, o)

	for i, v := range g1.AsBytes() {
		if i > int(o)-1 && i < int(setOnly)+int(o) {
			if v != setTo {
				t.Errorf("detailed: on affected i %d, got %d, expected %d", i, v, setTo)
			}
		} else {
			if v != 0 {
				t.Errorf("detailed: on unaffected i %d, got %d, expected 0", i, v)
			}
		}
	}
}

func TestMemSetU32(t *testing.T) {
	slabSizeReq := 10_000
	// 2048 bytes => 512 uint32 elements
	segSizeReq := 2_048
	N := segSizeReq / 4

	s := MakeSlab(slabSizeReq)
	g1, _ := s.MakeSegment(segSizeReq)

	setTo := rand.Uint32()
	g1.MemSetU32(setTo)

	for i, v := range g1.AsU32T() {
		if v != setTo {
			t.Errorf("on %d: got %d, expected %d", i, v, setTo)
		}
	}

	// safety check to make sure slab buff also has value set
	overlap := asU32T(s.base, int(N))
	for i, v := range overlap {
		if v != setTo {
			t.Errorf("slab base: on %d: got %d, expected %d", i, v, setTo)
		}
	}

	g1.MemSetU32(0)
	setTo = rand.Uint32()
	// add 40 byte offset (10 elements), and set 200 bytes (50)
	o := int(40)
	setOnly := int(200)
	g1.MemSetU32Detailed(setTo, setOnly, o)

	for i, v := range g1.AsU32T() {
		if i > 9 && i < 60 {
			if v != setTo {
				t.Errorf("detailed: on affected i %d, got %d, expected %d", i, v, setTo)
			}
		} else {
			if v != 0 {
				t.Errorf("detailed: on unaffected i %d, got %d, expected 0", i, v)
			}
		}
	}
}

func TestMemSetU64(t *testing.T) {
	slabSizeReq := 10_000
	// 4096 bytes => 512 uint32 elements
	segSizeReq := 4096
	N := segSizeReq / 8

	s := MakeSlab(slabSizeReq)
	g1, _ := s.MakeSegment(segSizeReq)

	setTo := rand.Uint64()
	g1.MemSetU64(setTo)

	for i, v := range g1.AsU64T() {
		if v != setTo {
			t.Errorf("on %d: got %d, expected %d", i, v, setTo)
		}
	}

	// safety check to make sure slab buff also has value set
	overlap := asU64T(s.base, int(N))
	for i, v := range overlap {
		if v != setTo {
			t.Errorf("slab base: on %d: got %d, expected %d", i, v, setTo)
		}
	}

	g1.MemSetU64(0)
	setTo = rand.Uint64()
	// add 400 byte offset (50 elements), and set 200 bytes (25)
	o := int(400)
	setOnly := int(200)
	g1.MemSetU64Detailed(setTo, setOnly, o)

	for i, v := range g1.AsU64T() {
		if i > 49 && i < 75 {
			if v != setTo {
				t.Errorf("detailed: on affected i %d, got %d, expected %d", i, v, setTo)
			}
		} else {
			if v != 0 {
				t.Errorf("detailed: on unaffected i %d, got %d, expected 0", i, v)
			}
		}
	}
}
