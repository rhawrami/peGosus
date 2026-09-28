package plan

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestSortPackedKeyIDsDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(529, 791))
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	for _, length := range []int{0, 1, 7, 8, 9, 17, 257} {
		for _, width := range []int{0, 1, 8, 16, 51, 64} {
			data := a.AllocSeg(length * 16)
			values := data.AsU64T()
			keys, ids := values[:length], values[length:]
			var mask uint64
			if width == 64 {
				mask = ^uint64(0)
			} else if width != 0 {
				mask = (uint64(1) << width) - 1
			}
			for i := range keys {
				keys[i] = rng.Uint64() & mask
				if i%7 == 0 {
					keys[i] = 0
				}
				ids[i] = uint64(i)
			}
			wantKeys := append([]uint64(nil), keys...)
			wantIDs := append([]uint64(nil), ids...)
			sort.SliceStable(wantIDs, func(i, j int) bool { return wantKeys[wantIDs[i]] < wantKeys[wantIDs[j]] })
			if !sortPackedKeyIDs(a, keys, ids, width) {
				t.Fatal("radix sort failed")
			}
			for i, id := range ids {
				if id != wantIDs[i] || keys[i] != wantKeys[id] {
					t.Fatalf("length=%d width=%d row=%d: got (%d,%d), want (%d,%d)", length, width, i, keys[i], id, wantKeys[wantIDs[i]], wantIDs[i])
				}
			}
			data.Dec()
		}
	}
}
