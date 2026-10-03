package plan

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkParallelCompactSort(b *testing.B) {
	for _, rows := range []int{32768, 65536, 1048576} {
		for _, stringKey := range []bool{false, true} {
			for _, workers := range []int{1, 2, 4, 8} {
				b.Run(fmt.Sprintf("rows=%d/string=%t/workers=%d", rows, stringKey, workers), func(b *testing.B) {
					a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
					rng := rand.New(rand.NewPCG(351, 86))
					keyType := dtype.Int64T()
					key := store.MakeVector(a, rows, keyType, false)
					if stringKey {
						key.Release()
						keyType = dtype.StringT()
						texts := make([][]byte, rows)
						for i := range texts {
							texts[i] = []byte(fmt.Sprintf("%08x long string payload", rng.Uint32()))
						}
						key = store.MakeStringVector(a, texts, nil)
					} else {
						for i := range rows {
							key.I64s()[i] = int64(rng.Uint64())
						}
					}
					batch := store.MakeBatch([]store.Vector{key})
					table := store.MakeTable([]*store.Batch{batch})
					p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{keyType})).OrderBy(MakeOrderKey(MakeColumn("key"))))
					if err != nil {
						b.Fatal(err)
					}
					defer p.Release()
					defer table.Release()
					defer batch.Release()
					step := p.steps[0]
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						state := makeCompactSortState(step)
						state.workers, state.ctx = workers, context.Background()
						if !state.add(a, batch, step, 256<<20) {
							b.Fatal("add")
						}
						out := state.finish(a, step)
						state.release()
						if out == nil || out.Len() != rows {
							b.Fatal("finish")
						}
						out.Release()
					}
				})
			}
		}
	}
}
