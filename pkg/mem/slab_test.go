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
			if s.capacity != len(s.buff) {
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
			if s.capacity%alignSize != 0 {
				t.Errorf("capacity %d not divisible by %d", s.capacity, alignSize)
			}

			// ensure capacity >= alignSize && capacity >= length request
			if s.capacity < alignSize || s.capacity < l {
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
	if g.Cap() < req {
		t.Errorf("segment cap %d < req %d", g.Cap(), req)
	}

	// with one request, ensure that segment base is same as slab base
	if g.base != s.base {
		t.Errorf("segment base %p != slab base %p", g.base, s.base)
	}

	// ensure segment length == request length
	if g.length != req {
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
	if s.used != expectedUsed {
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
	ok := s.SimpleCoalesce()
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

	if ok = s.SimpleCoalesce(); !ok {
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

func TestFullCoalesce(t *testing.T) {
	size := 10_000
	s := MakeSlab(size)

	// make 4 segments, put back first 3
	gReq := 1_000
	g1, _ := s.MakeSegment(gReq)
	g2, _ := s.MakeSegment(gReq)
	g3, _ := s.MakeSegment(gReq)
	_, _ = s.MakeSegment(gReq)

	g1.Put()
	g2.Put()
	g3.Put()

	if ok := s.FullCoalesce(); !ok {
		t.Errorf("FullCoalesce (simple): didn't work")
	}
	// FullCoalesce should:
	// 1. make segment length 3
	// 2. make hole count 1
	// 3. make segment 0 have capacity 1024 * 3
	// 4. make segment 0 keep base as slab base
	if len(s.segments) != 3 {
		t.Errorf("FullCoalesce (simple): got seg length %d, expected 3", len(s.segments))
	}
	if s.holes != 1 {
		t.Errorf("FullCoalesce (simple): got %d holes , expected 1", s.holes)
	}
	if s.segments[0].capacity != (1024 * 3) {
		t.Errorf("FullCoalesce (simple): got seg 0 cap %d , expected %d", s.segments[0].capacity, (1024 * 3))
	}
	if s.segments[0].base != s.base {
		t.Errorf("FullCoalesce (simple): got seg 0 base %p, expected %p", s.segments[0].base, s.base)
	}

	s.Clear()
	// Make more complex sequence
	// 15 segments (+ 1 edge)
	// free 10 segments:
	// [u, f, u, f, f, f, f, u, f, f, u, f, f, f, u | f]
	// after FullCoalesce (new length of 10, 4 holes)
	// [u, f (merged 0), u, f (merged 4), u, f (merged 2), u, f (merged 3), u | f]
	// segment 1 should have capacity 320
	// segment 3 should have capacity 320 * 4 => 1280
	// segment 5 should have capacity 320 * 2 => 640
	// segment 7 should have capacity 320 * 3 => 960
	// edge segment should have the same capacity
	N := 15
	reqSmall := 300
	freeSegs := []int{1, 3, 4, 5, 6, 8, 9, 11, 12, 13}
	segs := make([]*Segment, N)
	for i := 0; i < N; i++ {
		g, _ := s.MakeSegment(reqSmall)
		segs[i] = g
	}
	for _, v := range freeSegs {
		segs[v].Put()
	}
	if s.holes != 10 {
		t.Errorf("FullCoalesce (complex): after puts, got %d holes, expected 10", s.holes)
	}

	edgeCapBefore := s.segments[len(s.segments)-1].capacity

	ok := s.FullCoalesce()
	if !ok {
		t.Error("FullCoalesce (complex): full coalesce call did not work")
	}
	if len(s.segments) != 10 {
		t.Errorf("FullCoalesce (complex): got seg length %d, expected 10", len(s.segments))
	}
	if s.holes != 4 {
		t.Errorf("FullCoalesce (complex): got %d holes, expected 4", s.holes)
	}
	if g1 := s.segments[1]; g1.capacity != 320 {
		t.Errorf("FullCoalesce (complex): seg 1, got cap %d, expected %d", g1.capacity, 320)
	}
	if g3 := s.segments[3]; g3.capacity != 1280 {
		t.Errorf("FullCoalesce (complex): seg 3, got cap %d, expected %d", g3.capacity, 1280)
	}
	if g5 := s.segments[5]; g5.capacity != 640 {
		t.Errorf("FullCoalesce (complex): seg 5, got cap %d, expected %d", g1.capacity, 640)
	}
	if g7 := s.segments[7]; g7.capacity != 960 {
		t.Errorf("FullCoalesce (complex): seg 7, got cap %d, expected %d", g1.capacity, 960)
	}
	if newEdgeCap := s.segments[len(s.segments)-1].capacity; newEdgeCap != edgeCapBefore {
		t.Errorf("FullCoalesce (complex): edge cap got %d, expected %d", newEdgeCap, edgeCapBefore)
	}

}

func TestMakeWithCoalesce(t *testing.T) {
	size := 10_000
	s := MakeSlab(size)

	g1, _ := s.MakeSegment(2_000)
	g2, _ := s.MakeSegment(4_000)
	_, _ = s.MakeSegment(3_000)

	g1Cap, g2Cap := g1.capacity, g2.capacity

	// this should fail
	g5, ok := s.MakeSegment(5_000)
	if ok || g5 != nil {
		t.Errorf("requested 5k bytes, shouldve failed, got back a segment")
	}

	g1.Put()
	g2.Put()
	// should still fail, as g1 and g2 each don't have enough space
	g5, ok = s.MakeSegment(5_000)
	if ok || g5 != nil {
		t.Errorf("requested 5k bytes (after puts), shouldve failed, got back a segment")
	}

	// make with coalesce will work, as g1 and g2 should be merged
	g5, ok = s.MakeSegmentWithCoalesce(5_000)
	if !ok || g5 == nil {
		t.Errorf("requested 5k bytes using MakeWithCoalesce, shouldve worked, but failed")
	}
	// g5 should have the same base as g1
	if g5.base != g1.base {
		t.Errorf("g5 base, got %p, expected %p", g5.base, g1.base)
	}
	// g5 cap should equal g1 + g2 cap
	if g5.capacity != g1Cap+g2Cap {
		t.Errorf("g5 cap: got %d, expected %d", g5.capacity, g1Cap+g2Cap)
	}
	// there should be zero holes now
	if s.holes != 0 {
		t.Errorf("got %d holes, expected 0", s.holes)
	}
}

func TestGrow(t *testing.T) {
	size := 10_000
	s := MakeSlab(size)

	// make two segments, memset each
	// ensure that after TestGrow
	// 1. data is still the same
	// 2. offset from slab base is still the same
	// 3. edge capacity is updated to new capacity
	// 4. edge offset is correct
	// 5. slab on address is updated

	g1, _ := s.MakeSegment(2000)
	g2, _ := s.MakeSegment(2000)

	g1Set := byte(100)
	g2Set := byte(200)
	g1.MemSetU8(g1Set)
	g2.MemSetU8(g2Set)

	offsetG1 := calcOffset(s.base, g1.base)
	offsetG2 := calcOffset(s.base, g2.base)
	offsetEdge := calcOffset(s.base, s.segments[len(s.segments)-1].base)
	offsetOn := calcOffset(s.base, s.on)

	s.Grow(size * 2)

	for i, v := range g1.AsBytes() {
		if v != g1Set {
			t.Errorf("g1 [%d]: got %d, expected %d", i, v, g1Set)
		}
	}
	for i, v := range g2.AsBytes() {
		if v != g2Set {
			t.Errorf("g2 [%d]: got %d, expected %d", i, v, g2Set)
		}
	}

	if newOG1 := calcOffset(s.base, g1.base); newOG1 != offsetG1 {
		t.Errorf("g1: got offset %d, expected %d", newOG1, offsetG1)
	}
	if newOG2 := calcOffset(s.base, g2.base); newOG2 != offsetG2 {
		t.Errorf("g2: got offset %d, expected %d", newOG2, offsetG2)
	}
	if newEdgeOff := calcOffset(s.base, s.segments[len(s.segments)-1].base); newEdgeOff != offsetEdge {
		t.Errorf("edge: got offset %d, expected %d", newEdgeOff, offsetEdge)
	}
	if newOnOff := calcOffset(s.base, s.on); newOnOff != offsetOn {
		t.Errorf("slab on: got offset %d, expected %d", newOnOff, offsetOn)
	}

	if got, expected := s.segments[len(s.segments)-1].capacity, s.capacity-(g1.capacity+g2.capacity); got != expected {
		t.Errorf("edge cap: got %d, expected %d", got, expected)
	}
}

func TestClear(t *testing.T) {
	size := 10_000
	s := MakeSlab(size)

	reqSize := 1_000
	_, _ = s.MakeSegment(reqSize)
	_, _ = s.MakeSegment(reqSize)
	g, _ := s.MakeSegment(reqSize)
	_, _ = s.MakeSegment(reqSize)
	_, _ = s.MakeSegment(reqSize)
	g.Put()

	s.Clear()
	// ensure after clear that:
	// 1. used == 0
	// 2. N segments == 1
	// 3. on == base
	// 4. edge base == on
	// 5. edge capacity == slab capacity
	if s.used != 0 {
		t.Errorf("slab used: got %d, expected 0", s.used)
	}
	if len(s.segments) != 1 {
		t.Errorf("N segs: got %d, expected 1", len(s.segments))
	}
	if s.on != s.base {
		t.Errorf("slab on: got %p, expected %p", s.on, s.base)
	}
	edge := s.segments[len(s.segments)-1]
	if edge.base != s.on || edge.base != s.base {
		t.Errorf("edge base: got %p, expected %p", edge.base, s.on)
	}
	if edge.capacity != s.capacity {
		t.Errorf("edge cap: got %d, expected %d", edge.capacity, s.capacity)
	}
}

func TestHoleReuse(t *testing.T) {
	size := 10_000
	s := MakeSlab(size)

	// make three segs, each with size 1_024, free the second one
	_, _ = s.MakeSegment(1024)
	g2, _ := s.MakeSegment(1024)
	_, _ = s.MakeSegment(1024)
	g2.Put()

	// when we request g4, it should be the same as g2, as MakeSegment should've
	// found the hole, and been able to use it.
	g4, _ := s.MakeSegment(1024)
	if g2 != g4 {
		t.Errorf("g4 %p != g2 %p", g4, g2)
	}
	if g2.capacity != g4.capacity {
		t.Errorf("g4 cap %d != g2 cap %d", g4.capacity, g2.capacity)
	}
	if g2.base != g4.base {
		t.Errorf("g4 base %p != g2 base %p", g4.base, g2.base)
	}
	// quick check that slab used is updated
	if s.used != 1024*3 {
		t.Errorf("slab used: got %d, expected %d", s.used, 1024*3)
	}
	// also decrease holes to 0
	if s.holes != 0 {
		t.Errorf("slab holes: got %d, expected 0", s.holes)
	}

	g4.Put()

	// when we request g5 with 1025 bytes, we CANNOT reuse g4, as g4 can't support 1024 bytes
	g5, _ := s.MakeSegment(1025)

	if g5 == g4 || g5.base == g4.base {
		t.Error("g5 == g4")
	}
	if s.holes != 1 {
		t.Errorf("slab holes (second): got %d, expected 1", s.holes)
	}

}
func TestBisect(t *testing.T) {
	size := 10_000
	s := MakeSlab(size)

	// make three segs, each with size 1_024, free the second one
	g1, _ := s.MakeSegment(1024)
	g2, _ := s.MakeSegment(1024)
	g3, _ := s.MakeSegment(1024)
	g2.Put()

	// requesting 128 bytes should reuse g2 and cause a bisection of g2,
	// with 128 bytes on the left and 896 on the right.
	g4, _ := s.MakeSegment(128)
	if s.holes != 1 {
		t.Errorf("didnt bisect holes: got %d, expected 1", s.holes)
	}
	if g4 != g2 {
		t.Errorf("didnt reuse g2")
	}
	if g4.capacity != 128 {
		t.Errorf("didnt bisect g2: cap = %d", g4.capacity)
	}
	if len(s.segments) != 5 {
		t.Errorf("didnt bisect g2: N segs = %d", len(s.segments))
	}

	left, right := g4, s.segments[2]
	if right.capacity != 896 {
		t.Errorf("right cap: got %d, expected 896", right.capacity)
	}
	if right.base != incPtr(left.base, int(left.capacity)) {
		t.Errorf("right base: got %p, got %p", right.base, incPtr(left.base, int(left.capacity)))
	}

	// ensure g1 is still g1, and g3 is still g3 (with new index)
	if g1 != s.segments[0] {
		t.Errorf("g1 got moved: got %p, expected %p", s.segments[0], g1)
	}
	if g3 != s.segments[3] {
		t.Errorf("g3 got moved: got %p, expected %p", s.segments[3], g3)
	}

}
