package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkPhysicalPackedMultiKeySort(b *testing.B) {
	for _, rows := range []int{1, 8, 32, 64, 256, 8192, 32768} {
		for _, fraction := range []int{1, 8} {
			for _, tryPacked := range []bool{false, true} {
				name := fmt.Sprintf("rows=%d/selected=1in%d", rows, fraction)
				if tryPacked {
					name += "/packed"
				} else {
					name += "/current"
				}
				b.Run(name, func(b *testing.B) {
					b.StopTimer()
					a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
					rng := rand.New(rand.NewPCG(701, 53))
					var batches []*store.Batch
					active := 0
					for start := 0; start < rows; start += 1024 {
						length := min(1024, rows-start)
						number := store.MakeVector(a, length, dtype.Int32T(), true)
						ids := store.MakeVector(a, length, dtype.Int32T(), false)
						stringsIn, payloads := make([][]byte, length), make([][]byte, length)
						valid := make([]bool, length)
						selection := store.MakeBitMap(a, length)
						for row := range length {
							i := start + row
							number.I32s()[row] = int32(rng.IntN(65536) - 32768)
							if i%32 == 0 {
								number.Validity().Clear(row)
							}
							stringsIn[row] = make([]byte, rng.IntN(5))
							for j := range stringsIn[row] {
								stringsIn[row][j] = byte(rng.IntN(128))
							}
							valid[row] = i%31 != 0
							payloads[row] = strconv.AppendInt([]byte("retained long payload "), int64(i), 10)
							ids.I32s()[row] = int32(i)
							if i%fraction == 0 {
								selection.Set(row)
								active++
							}
						}
						text := store.MakeStringVector(a, stringsIn, valid)
						payload := store.MakeStringVector(a, payloads, nil)
						batch := store.MakeBatch([]store.Vector{number, text, ids, payload})
						batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
						batches = append(batches, batch)
					}
					table := store.MakeTable(batches)
					for _, batch := range batches {
						batch.Release()
					}
					plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"number", "text", "id", "payload"}, []dtype.Type{dtype.Int32T(), dtype.StringT(), dtype.Int32T(), dtype.StringT()})).OrderBy(MakeOrderKey(MakeColumn("number")), MakeOrderKey(MakeColumn("text"))))
					if err != nil {
						b.Fatal(err)
					}
					b.ReportAllocs()
					b.StartTimer()
					for range b.N {
						count := 0
						result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: math.MaxInt64, TryPackedSort: tryPacked}, func(output *store.Batch) bool {
							count += output.ActiveLen()
							return true
						})
						if result.Code() != ExecutionCompleted || count != active {
							b.Fatalf("packed=%t result=%v rows=%d, want=%d", tryPacked, result.Code(), count, active)
						}
					}
					b.StopTimer()
					b.ReportMetric(float64(active), "rows/op")
					plan.Release()
					table.Release()
				})
			}
		}
	}
}
