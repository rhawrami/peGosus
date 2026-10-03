package plan

import (
	"fmt"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func BenchmarkKeyIndexControls(b *testing.B) {
	for _, capacity := range []int{1024, 65536, 1048576} {
		b.Run(fmt.Sprintf("capacity=%d", capacity), func(b *testing.B) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			h := keyIndex{tryControls: true}
			keys := keyArena{}
			count := capacity - capacity/4
			queries := make([][]byte, 8192)
			for row := range count {
				key := []byte(fmt.Sprintf("long overlapping binary \x00\xff key-%08d", row))
				if !keys.append(a, key, 1<<30) || !h.put(a, key, row, 1<<30) {
					b.Fatal("insert")
				}
			}
			defer h.release()
			defer keys.release()
			cache := h.controls
			for _, missing := range []bool{false, true} {
				for row := range queries {
					id := (row * 7919) % count
					if missing {
						id += count
					}
					queries[row] = []byte(fmt.Sprintf("long overlapping binary \x00\xff key-%08d", id))
				}
				for _, controlled := range []bool{false, true} {
					b.Run(fmt.Sprintf("missing=%t/controls=%t", missing, controlled), func(b *testing.B) {
						if controlled {
							h.controls = cache
						} else {
							h.controls = nil
						}
						b.ReportAllocs()
						var found int
						b.ResetTimer()
						for i := range b.N {
							if _, ok := h.lookup(queries[i&(len(queries)-1)], &keys); ok {
								found++
							}
						}
						b.StopTimer()
						b.ReportMetric(float64(h.bytes()), "index-bytes")
						h.controls = cache
						if missing && found != 0 || !missing && found != b.N {
							b.Fatalf("found=%d N=%d", found, b.N)
						}
					})
				}
			}
		})
	}
}
