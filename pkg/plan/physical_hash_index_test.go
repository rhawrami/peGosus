package plan

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestU64IndexDifferentialAndGrowth(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	h := &u64Index{}
	defer h.release()
	rng := rand.New(rand.NewPCG(61, 72))
	reference := make(map[uint64]int)
	for i := range 4096 {
		key := rng.Uint64()
		if i%7 == 0 {
			key = uint64(i%17) << 32
		}
		if i%151 == 0 {
			key = math.MaxUint64
		}
		if _, exists := reference[key]; !exists {
			if !h.put(a, key, len(reference), math.MaxInt64) {
				t.Fatalf("insert key %#x failed", key)
			}
			reference[key] = len(reference)
		}
		if got, ok := h.get(key); !ok || got != reference[key] {
			t.Fatalf("key %#x: got %d/%t, expected %d", key, got, ok, reference[key])
		}
	}
	for key, want := range reference {
		if got, ok := h.get(key); !ok || got != want {
			t.Fatalf("rehash lost key %#x: got %d/%t, expected %d", key, got, ok, want)
		}
	}
	if _, found := h.get(0x1234567890abcdef); found {
		t.Fatal("index reported an absent key")
	}
}

func TestU64IndexResourceBound(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	h := &u64Index{}
	defer h.release()
	if h.put(a, 0, 0, 255) {
		t.Fatal("created the first table beyond the budget")
	}
	for i := range 12 {
		if !h.put(a, uint64(i), i, 700) {
			t.Fatalf("insert %d failed below load threshold", i)
		}
	}
	if h.put(a, 12, 12, 700) {
		t.Fatal("resized table beyond its peak budget")
	}
	if h.bytes() != 256 || h.used != 12 {
		t.Fatalf("failed resize altered the table: bytes=%d entries=%d", h.bytes(), h.used)
	}
	for i := range 12 {
		if got, ok := h.get(uint64(i)); !ok || got != i {
			t.Fatalf("failed resize lost key %d", i)
		}
	}
	if !h.put(a, 12, 12, 768) || h.bytes() != 512 {
		t.Fatal("valid resize did not complete")
	}
}
