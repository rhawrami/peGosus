package mem

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"unsafe"
)

func TestMakeSlab(t *testing.T) {
	N := 50
	slabSizes := make([]int, N)
	for i := 0; i < N; i++ {
		slabSizes[i] = int(rand.Uint64N(50_000_000))
	}

	for _, l := range slabSizes {
		t.Run(fmt.Sprintf("Request %d bytes", l), func(t *testing.T) {
			s := MakeSlab(l)

			// ensure capacity == slice length
			if s.capacity != uint64(len(s.buff)) {
				t.Errorf("capacity %d != slice length %d", s.capacity, len(s.buff))
			}
			// ensure underlying buffer is aligned
			if !isAligned((*byte)(unsafe.Pointer(&s.buff[0])), alignSize) {
				t.Errorf("buffer not aligned to %d bytes", alignSize)
			}
			if !isAligned(s.base, alignSize) {
				t.Errorf("base address not aligned to %d bytes", alignSize)
			}
			if !isAligned((*byte)(unsafe.Pointer(s.on)), alignSize) {
				t.Errorf("starting address not aligned to %d bytes", alignSize)
			}

			// ensure capacity is divisible by alignSize
			if s.capacity%uint64(alignSize) != 0 {
				t.Errorf("capacity %d not divisible by %d", s.capacity, alignSize)
			}

			// ensure capacity >= alignSize && capacity >= length request
			if s.capacity < uint64(alignSize) || s.capacity < uint64(l) {
				t.Errorf("capacity %d not >= %d and request %d", s.capacity, alignSize, l)
			}
		})
	}

}

func TestMakeOneSegment(t *testing.T) {
	size := 1_000
	s := MakeSlab(size)

	// make segment that should work
	req := size >> 1
	g, ok := s.MakeSegment(req)
	if g == nil || !ok {
		t.Errorf("segment request should have worked, but returned %v, %v", g, ok)
	}

	// ensure base aligned to alignSize
	if !isAligned(g.base, alignSize) {
		t.Errorf("segment base %p not aligned to %d bytes", g.base, alignSize)
	}

	// ensure capacity is at least size of request
	if g.Cap() < uint64(req) {
		t.Errorf("segment cap %d < req %d", g.Cap(), req)
	}

	// with one request, ensure that segment base is same as slab base
	if g.base != s.base {
		t.Errorf("segment base %p != slab base %p", g.base, s.base)
	}

	// ensure segment length == request length
	if g.length != uint64(req) {
		t.Errorf("starting segment length %d != request length %d", g.length, req)
	}

	// ensure ref count is 1
	if g.RefCount() != 1 {
		t.Errorf("segment ref count %d != 1", g.RefCount())
	}

}

func TestOneSegmentManagement(t *testing.T) {
	size := 1_000
	s := MakeSlab(size)

	g, ok := s.MakeSegment(size >> 1)
	if !ok || g == nil {
		t.Errorf("segment request should have worked, but returned %v, %v", g, ok)
	}

	// ensure `used` == segment capacity
	if g.capacity != s.used {
		t.Errorf("segment cap %d != slab used %d", g.capacity, s.used)
	}

	g.Inc()
	g.Inc()
	g.Inc()
	if g.RefCount() != 4 {
		t.Errorf("called Inc three times, expected 4, got %d", g.RefCount())
	}

	ok = g.Dec()
	ok = g.Dec()
	ok = g.Dec()
	if !ok || g.RefCount() != 1 {
		t.Errorf("called Dec three times (after calling Inc three times), expected 1, got %d and %v",
			g.RefCount(), ok)
	}

	// return segment, ensure Slab updates `used` correctly.
	g.Put()
	if s.used != 0 {
		t.Errorf("returned segment, but used (%d) != 0", s.used)
	}

	// quick test on penultimate segment merging
	if len(s.segments) != 1 {
		t.Errorf("merged penultimate segment, but did not merge to edge")
	}
	if edge := s.segments[len(s.segments)-1]; edge.Cap() != s.capacity {
		t.Errorf("merged segment cap %d != slab cap %d", edge.Cap(), s.capacity)
	}

}

func TestMakeMultiSegment(t *testing.T) {
	size := 10_000
	s := MakeSlab(size)

	// with a 10K byte request (rounded to 10_048), if we request 1k bytes (rounded to 1024),
	// we can get (10_048 / 1_024 => 9) requests before getting denied
	req := 1_000
	expected := 9
	got := 0
	ok := true
	for ok {
		_, ok = s.MakeSegment(req)
		if ok {
			got++
		} else {
			ok = false
		}
	}
	if got != expected {
		t.Errorf("# successful requests: got %d, expected %d", got, expected)
	}

	// ensure no holes
	if s.holes != 0 {
		t.Errorf("got %d holes, expected 0", s.holes)
	}

	// after 9 requests, bytes used => 9 * 1_024
	expectedUsed := expected * 1_024
	if s.used != uint64(expectedUsed) {
		t.Errorf("bytes used: got %d, expected %d", s.used, expectedUsed)
	}

	// 1. after 9 * 1024 bytes used, address on should be (base + (9 * 1024))
	// 2. edge segment should have base as (slab base + (9 * 1024))
	// 3. edge segment should have capacity as slab cap - (9 * 1024)
	expectedOn := incPtr(s.base, 1024*9)
	expectedEdgeCap := s.capacity - (9 * 1024)
	edge := s.segments[len(s.segments)-1]
	if expectedOn != s.on {
		t.Errorf("s.on: got %p, expected %p", s.on, expectedOn)
	}
	if edge.base != expectedOn {
		t.Errorf("edge base: got %p, expected %p", edge.base, expectedOn)
	}
	if expectedEdgeCap != edge.capacity {
		t.Errorf("edge cap: got %d, expected %d", edge.capacity, expectedEdgeCap)
	}

	s.Clear()
	// after clearing
	// 1. slab use should be 0
	// 2. slab segment length should be 1 (edge)
	// 3. on should equal base
	// 4. edge cap should equal slab cap
	// 5. edge base should equal slab base
	if s.used != 0 {
		t.Errorf("slab use after clear: got %d, expected, 0", s.used)
	}
	if len(s.segments) != 1 {
		t.Errorf("slab seg length after clear: got %d, expected 1", len(s.segments))
	}
	if s.on != s.base {
		t.Errorf("slab on addr after clear: got %p, expected %p", s.on, s.base)
	}
	edge = s.segments[len(s.segments)-1]
	if edge.capacity != s.capacity {
		t.Errorf("edge cap after clear: got %d, expected %d", edge.capacity, s.capacity)
	}
	if edge.base != s.base {
		t.Errorf("edge base after clear: got %p, expected %p", edge.base, s.base)
	}
}

func TestMultiSegmentManagement(t *testing.T) {
	size := 10_000
	s := MakeSlab(size)

	// alloc 3k bytes
	gReq := 1000
	g1, _ := s.MakeSegment(gReq)
	g2, _ := s.MakeSegment(gReq)
	g3, _ := s.MakeSegment(gReq)

	// if we return third segment:
	// 1. there should be no hole added
	// 2. the segment should be merged into the edge segment
	// 3. the slab on AND edge base should be third seg base
	// 4. edge cap should be penultimate cap + edge cap
	// 5. slab used should be seg 1 cap + seg2 cap
	baseG3 := g3.base
	expectedCap := s.segments[len(s.segments)-1].capacity + g3.capacity
	expectedUsed := g1.capacity + g2.capacity
	g3.Put()
	if s.holes != 0 {
		t.Errorf("put penultimate seg: got %d holes, expected 0", s.holes)
	}
	if len(s.segments) != 3 {
		t.Errorf("put penultimate seg: got %d segments, expected %d", len(s.segments), 3)
	}
	if baseG3 != s.on || baseG3 != s.segments[len(s.segments)-1].base {
		t.Errorf("put penultimate seg: got slab on %p, edge base %p, expected %p",
			s.on, s.segments[len(s.segments)-1].base, baseG3)
	}
	if edgeCap := s.segments[len(s.segments)-1].capacity; edgeCap != expectedCap {
		t.Errorf("put penultimate seg: got edge cap %d, expected %d", edgeCap, expectedCap)
	}
	if s.used != expectedUsed {
		t.Errorf("put penultimate seg: got %d used, expected %d", s.used, expectedUsed)
	}

	// return first segment:
	// 1. should create a hole
	// 2. update used
	expectedUsed = g2.capacity
	g1.Put()
	if s.holes != 1 {
		t.Errorf("put first seg: got %d holes, expected 1", s.holes)
	}
	if s.used != expectedUsed {
		t.Errorf("put first seg: got %d used, expected %d", s.used, expectedUsed)
	}

}

func TestSimpleCoalesce(t *testing.T) {
	size := 10_000
	s := MakeSlab(size)

	gReq := 1000
	g1, _ := s.MakeSegment(gReq)
	g2, _ := s.MakeSegment(gReq)
	g3, _ := s.MakeSegment(gReq)

	// put first and second segment
	g1.Put()
	g2.Put()

	if s.holes != 2 {
		t.Errorf("put two segs: got %d holes, expected 2", s.holes)
	}

	// simple coalesce should be able to coalesce first 2 free segments
	// and reduce hole count from 2 to 1
	ok := s.coalesce()
	if !ok || s.holes != 1 {
		t.Errorf("attempt coalesce: got %v and %d holes, expected %v and %d holes",
			ok, s.holes, true, 1)
	}

	g3.Put()

	// more complicated version
	// 10 requested segments
	// u[sed] & f[ree] [u, f, f, u, f, f, f, u, f, u | f]
	// simple coalesce should
	// 1. reduce 6 holes to 4
	// 2. have segments[1|3|4|6]
	// 3. reduce segment length from 11 to 9
	// 4. have segments[1|3].capacity as 1024
	// 5. have segments[1|3].base as their left-merged bases
	sizeSmall := 500
	N := 10
	segs := make([]*Segment, 0, N)
	for range N {
		g, _ := s.MakeSegment(sizeSmall)
		segs = append(segs, g)
	}
	leftBase1 := segs[1].base
	leftBase2 := segs[4].base

	for i := range N {
		if slices.Contains([]int{1, 2, 4, 5, 6, 8}, i) {
			segs[i].Put()
		}
	}

	if ok = s.coalesce(); !ok {
		t.Error("simple coalesce did not work in complex test")
	}
	if s.holes != 4 {
		t.Errorf("complex simple coalesce: got %d holes, expected 4", s.holes)
	}
	if len(s.segments) != 9 {
		t.Errorf("complex simple coalesce: got %d segments, expected 9", len(s.segments))
	}

	expectedFreeSegs := [4]int{1, 3, 4, 6}
	for _, v := range expectedFreeSegs {
		if !s.segments[v].IsFree() {
			t.Errorf("complex simple coalesce: segment %d is not free", v)
		}
		// merged indices
		if v == 1 || v == 3 {
			if s.segments[v].capacity != 1024 {
				t.Errorf("complex simple coalesce: segment %d got %d cap, expected 1024", v, s.segments[v].capacity)
			}
		}
		// base check
		if v == 1 {
			if s.segments[v].base != leftBase1 {
				t.Errorf("complex simple coalesce: seg 1 got base %p, expected %p", s.segments[v].base, leftBase1)
			}
		}
		if v == 3 {
			if s.segments[v].base != leftBase2 {
				t.Errorf("complex simple coalesce: seg 3 got base %p, expected %p", s.segments[v].base, leftBase2)
			}
		}
	}

}

func TestFullCoalesce(t *testing.T) {}

func TestMakeWithCoalesce(t *testing.T) {}

func TestGrow(t *testing.T) {}

func TestClear(t *testing.T) {}

func TestHoleReuse(t *testing.T) {}
func TestBisect(t *testing.T)    {}
