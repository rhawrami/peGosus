package mem

import (
	"math/rand/v2"
	"testing"
)

func TestAllocStats(t *testing.T) {
	cycle := uint64(10)
	a := AllocStats{cycle: cycle}

	// request 100 bytes, from general
	r1 := 100
	f1 := reqGeneral
	a.updateState(r1, f1)
	if a.avgReq != uint64(r1) {
		t.Errorf("req 1 (avgReq): got %d, expected %d", a.avgReq, r1)
	}
	if a.nLocs[reqGeneral] != 1 {
		t.Errorf("req 1 (nLocs): got %d, expected %d", a.nLocs[reqGeneral], 1)
	}

	// request 200 bytes, from scratch
	r2 := 200
	f2 := reqScratch
	a.updateState(r2, f2)
	if a.avgReq != uint64((r2+r1)/2) {
		t.Errorf("req 2 (avgReq): got %d, expected %d", a.avgReq, uint64((r2+r1)/2))
	}
	if a.nLocs[reqScratch] != 1 {
		t.Errorf("req 2 (nLocs): got %d, expected %d", a.nLocs[reqScratch], 1)
	}

	// request 500 bytes, from general
	r3 := 500
	f3 := reqGeneral
	a.updateState(r3, f3)
	if a.avgReq != uint64((r2+r1)/2+r3)/2 {
		t.Errorf("req 3 (avgReq): got %d, expected %d", a.avgReq, uint64((r3+r2+r1)/3))
	}
	if a.nLocs[reqGeneral] != 2 {
		t.Errorf("req 3 (nLocs): got %d, expected %d", a.nLocs[reqGeneral], 2)
	}

	// reset
	a.resetState()
	if a.lastOptimized != 0 || a.nLocs[reqGeneral] != 0 || a.nLocs[reqScratch] != 0 {
		t.Errorf("called reset: got lastOpt %d, nLocs %v, expected lastOpt 0, nLocs [0 0]", a.lastOptimized, a.nLocs)
	}
}

func TestMakeAllocatorWithConfig(t *testing.T) {
	nGP, nSP := 10, 5
	gp, sp := make([]uint64, nGP), make([]uint64, nSP)
	for i := range gp {
		gp[i] = rand.Uint64N(50_000)
	}
	for i := range sp {
		sp[i] = rand.Uint64N(25_000)
	}

	cacheN := rand.Uint64N(10)
	cycleN := rand.Uint64N(10)

	cfg := MakeAllocConfig(gp, sp, cycleN, cacheN)

	a := MakeAllocatorWithConfig(cfg)

	// general
	for i, v := range a.general.slabs {
		if v.capacity < gp[i] {
			t.Errorf("on general %d: got cap %d, wanted at least %d", i, v.capacity, gp[i])
		}
	}
	// scratch
	for i, v := range a.scratch.slabs {
		if v.capacity < sp[i] {
			t.Errorf("on scratch %d: got cap %d, wanted at least %d", i, v.capacity, sp[i])
		}
	}
	// cache
	if cap(a.dataCache) != int(cacheN) {
		t.Errorf("got dataCache cap %d, expected %d", cap(a.dataCache), cacheN)
	}
	// cycle
	if a.stats.cycle != cycleN {
		t.Errorf("got cycle %d, expected %d", a.stats.cycle, cycleN)
	}
}

func TestMakeAllocatorWithProfiles(t *testing.T) {
	nGP, nSP := 10, 5
	gp, sp := make([]uint64, nGP), make([]uint64, nSP)
	for i := range gp {
		gp[i] = rand.Uint64N(50_000)
	}
	for i := range sp {
		sp[i] = rand.Uint64N(25_000)
	}

	a := MakeAllocatorWithProfiles(gp, sp)

	// general
	for i, v := range a.general.slabs {
		if v.capacity < gp[i] {
			t.Errorf("on general %d: got cap %d, wanted at least %d", i, v.capacity, gp[i])
		}
	}
	// scratch
	for i, v := range a.scratch.slabs {
		if v.capacity < sp[i] {
			t.Errorf("on scratch %d: got cap %d, wanted at least %d", i, v.capacity, sp[i])
		}
	}
	// cache
	if cap(a.dataCache) != int(dataCacheSizeDefault) {
		t.Errorf("got dataCache cap %d, expected %d", cap(a.dataCache), dataCacheSizeDefault)
	}
	// cycle
	if a.stats.cycle != optCycleDefault {
		t.Errorf("got cycle %d, expected %d", a.stats.cycle, optCycleDefault)
	}
}

func TestAllocatorClear(t *testing.T) {
	nGP, nSP := 10, 5
	gp, sp := make([]uint64, nGP), make([]uint64, nSP)
	for i := range gp {
		gp[i] = rand.Uint64N(50_000)
	}
	for i := range sp {
		sp[i] = rand.Uint64N(25_000)
	}

	a := MakeAllocatorWithProfiles(gp, sp)

	N := 50

	// all Alloc calls go to general
	for range N {
		r := rand.IntN(5_000)
		_ = a.Alloc(r)
	}
	a.ClearGeneral()
	for i, v := range a.general.slabs {
		if v.used != 0 || len(v.segments) != 1 {
			t.Errorf("on general %d: got used %d, %d segs, expected 0, 1", i, v.used, len(v.segments))
		}
	}

	// most AllocTemp calls go to scratch
	for range N {
		r := rand.IntN(1_000)
		_ = a.AllocTemp(r)
	}
	a.ClearScratch()
	for i, v := range a.scratch.slabs {
		if v.used != 0 || len(v.segments) != 1 {
			t.Errorf("on scratch %d: got used %d, %d segs, expected 0, 1", i, v.used, len(v.segments))
		}
	}

	d := make([]*Data, dataCacheSizeDefault)
	for i := range dataCacheSizeDefault {
		d[i] = a.Alloc(100)
	}
	for _, v := range d {
		a.TakeData(v)
	}
	// cache should be full now
	a.ClearDataCache()
	if len(a.dataCache) != 0 {
		t.Errorf("cleared cache: got len %d %v, expected 0 []", len(a.dataCache), a.dataCache)
	}

	d = d[:0]
	for range N {
		_ = a.Alloc(100)
		b := a.AllocTemp(100)
		if len(d) != cap(d) {
			d = append(d, b)
		}
	}
	for _, v := range d {
		a.TakeData(v)
	}

	a.Clear()
	// everything should be clean now
	for i, v := range a.general.slabs {
		if v.used != 0 || len(v.segments) != 1 {
			t.Errorf("cleared all: on general %d: got used %d, %d segs, expected 0, 1", i, v.used, len(v.segments))
		}
	}
	for i, v := range a.scratch.slabs {
		if v.used != 0 || len(v.segments) != 1 {
			t.Errorf("cleared all: on scratch %d: got used %d, %d segs, expected 0, 1", i, v.used, len(v.segments))
		}
	}
	if len(a.dataCache) != 0 {
		t.Errorf("cleared all: cache: got len %d %v, expected 0 []", len(a.dataCache), a.dataCache)
	}
}

func TestAllocatorGrow(t *testing.T) {
	nGP, nSP := 10, 5
	gp, sp := make([]uint64, nGP), make([]uint64, nSP)
	for i := range gp {
		gp[i] = rand.Uint64N(50_000)
	}
	for i := range sp {
		sp[i] = rand.Uint64N(25_000)
	}

	a := MakeAllocatorWithProfiles(gp, sp)

	// add general slab with at least 10_000 bytes
	r1 := 10_048
	nBefore := len(a.general.slabs)
	cBefore := a.general.capacity
	a.GrowGeneral(r1)
	if c := a.general.capacity; c != cBefore+uint64(r1) {
		t.Errorf("grew general: got tot cap %d, expected %d", c, cBefore+uint64(r1))
	}
	if c := a.general.slabs[len(a.general.slabs)-1].capacity; c != uint64(r1) {
		t.Errorf("grew general: got final slab cap %d, expected %d", c, r1)
	}
	if n := len(a.general.slabs); n != nBefore+1 {
		t.Errorf("grew general: got N slabs of %d, expected %d", n, nBefore+1)
	}

	// add scratch slab with at least 5_000 bytes
	r1 = 5056
	nBefore = len(a.scratch.slabs)
	cBefore = a.scratch.capacity
	a.GrowScratch(r1)
	if c := a.scratch.capacity; c != cBefore+uint64(r1) {
		t.Errorf("grew scratch: got tot cap %d, expected %d", c, cBefore+uint64(r1))
	}
	if c := a.scratch.slabs[len(a.scratch.slabs)-1].capacity; c != uint64(r1) {
		t.Errorf("grew scratch: got final slab cap %d, expected %d", c, r1)
	}
	if n := len(a.scratch.slabs); n != nBefore+1 {
		t.Errorf("grew scratch: got N slabs of %d, expected %d", n, nBefore+1)
	}
}

func TestAllocatorDataCache(t *testing.T) {
	nGP, nSP := 10, 5
	gp, sp := make([]uint64, nGP), make([]uint64, nSP)
	for i := range gp {
		gp[i] = rand.Uint64N(50_000)
	}
	for i := range sp {
		sp[i] = rand.Uint64N(25_000)
	}

	a := MakeAllocatorWithProfiles(gp, sp)

	d := make([]*Data, dataCacheSizeDefault+1)
	for i := range d {
		d[i] = a.Alloc(1_000)
	}

	// fill up cache
	for i := range dataCacheSizeDefault {
		a.TakeData(d[i])
		if a.dataCache[i] != d[i] || len(a.dataCache) != int(i+1) {
			t.Errorf("got cache %v, expected [..., %p]", a.dataCache, d[i])
		}
	}
	// final data in d shouldn't be added
	a.TakeData(d[dataCacheSizeDefault])
	if a.dataCache[len(a.dataCache)-1] == d[dataCacheSizeDefault] || len(a.dataCache) != cap(a.dataCache) {
		t.Errorf("added final element %p to %v, when it shouldnt have", d[dataCacheSizeDefault], a.dataCache)
	}

	// make segments, should deplete cache
	for i := range dataCacheSizeDefault {
		v := a.Alloc(100)
		if v != d[dataCacheSizeDefault-i-1] {
			t.Errorf("on %d, expected %p, got %p", i, v, d[i])
		}
		if len(a.dataCache) != int(dataCacheSizeDefault-i-1) {
			t.Errorf("got cache len %d, expected %d", len(a.dataCache), int(dataCacheSizeDefault-i-1))
		}
	}
}

func TestAllocatorOptimize(t *testing.T) {}

func TestAllocatorAllocTemp(t *testing.T) {}

func TestAllocatorAllocTempWithProfile(t *testing.T) {}

func TestAllocatorAlloc(t *testing.T) {}

func TestAllocatorAllocWithProfile(t *testing.T) {}
