package plan

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkCompositeKeySort(b *testing.B) {
	const n = 32768
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	rng := rand.New(rand.NewPCG(997, 18))
	numeric := store.MakeVector(a, n, dtype.Int32T(), false)
	stringsIn := make([][]byte, n)
	for i := range numeric.I32s() {
		numeric.I32s()[i] = int32(rng.IntN(2001) - 1000)
		length := rng.IntN(5)
		stringsIn[i] = make([]byte, length)
		for j := range stringsIn[i] {
			stringsIn[i][j] = byte(rng.IntN(128))
		}
	}
	text := store.MakeStringVector(a, stringsIn, nil)
	columns := []store.Vector{numeric, text}
	want := make([]uint64, n)
	for i := range want {
		want[i] = uint64(i)
	}
	sort.SliceStable(want, func(i, j int) bool {
		x, y := int(want[i]), int(want[j])
		if numeric.I32s()[x] != numeric.I32s()[y] {
			return numeric.I32s()[x] < numeric.I32s()[y]
		}
		return text.Strings()[x].View() < text.Strings()[y].View()
	})
	for _, mode := range []string{"comparator", "packed-radix"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				idsSegment := a.AllocSegTemp(n * 8)
				ids := idsSegment.AsU64T()
				for i := range ids {
					ids[i] = uint64(i)
				}
				if mode == "comparator" {
					sort.SliceStable(ids, func(i, j int) bool {
						x, y := int(ids[i]), int(ids[j])
						if numeric.I32s()[x] != numeric.I32s()[y] {
							return numeric.I32s()[x] < numeric.I32s()[y]
						}
						return text.Strings()[x].View() < text.Strings()[y].View()
					})
				} else {
					stats := makePackedKeyStats([]dtype.Type{dtype.Int32T(), dtype.StringT()})
					if !stats.observe(a, columns, nil) {
						b.Fatal("key statistics failed")
					}
					layout, ok := stats.layout(keyOrder, []bool{false, false}, []bool{false, false})
					if !ok {
						b.Fatal("key layout did not fit")
					}
					keySegment := a.AllocSegTemp(n * 8)
					keys := keySegment.AsU64T()
					for row := range keys {
						var valid bool
						keys[row], valid = layout.encode(columns, row)
						if !valid {
							b.Fatal("key encoding failed")
						}
					}
					if !sortPackedKeyIDs(a, keys, ids, layout.width) {
						b.Fatal("packed sort failed")
					}
					keySegment.Dec()
				}
				for i := range ids {
					if ids[i] != want[i] {
						b.Fatalf("incorrect sort at %d: got %d, want %d", i, ids[i], want[i])
					}
				}
				idsSegment.Dec()
			}
		})
	}
	text.Release()
	numeric.Release()
}
