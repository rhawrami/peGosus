package plan

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkGroupedCountState(b *testing.B) {
	const rows = 32768
	const batchSize = 1024
	for _, cardinality := range []int{16, 4096} {
		for _, selectedFraction := range []int{1, 8} {
			name := fmt.Sprintf("groups=%d/selected=1in%d", cardinality, selectedFraction)
			for _, compact := range []bool{false, true} {
				implementation := "baseline"
				if compact {
					implementation = "compact"
				}
				b.Run(name+"/"+implementation, func(b *testing.B) {
					b.StopTimer()
					a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
					rng := rand.New(rand.NewPCG(571, 19))
					batches := make([]*store.Batch, 0, rows/batchSize)
					activeRows := 0
					for start := 0; start < rows; start += batchSize {
						key := store.MakeVector(a, batchSize, dtype.Int32T(), true)
						selection := store.MakeBitMap(a, batchSize)
						for row := range batchSize {
							key.I32s()[row] = int32(rng.IntN(cardinality))
							if rng.IntN(32) == 0 {
								key.Validity().Clear(row)
							}
							if (start+row)%selectedFraction == 0 {
								selection.Set(row)
								activeRows++
							}
						}
						batch := store.MakeBatch([]store.Vector{key})
						batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
						batches = append(batches, batch)
					}
					table := store.MakeTable(batches)
					plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int32T()})).GroupBy(
						[]Expr{MakeColumn("key")}, MakeCountStar(), MakeCount(MakeColumn("key")),
					))
					if err != nil {
						b.Fatal(err)
					}
					step := plan.steps[0]
					b.ReportAllocs()
					b.SetBytes(int64(activeRows * 4))
					var groupCount int
					var stateBytes int64
					b.StartTimer()
					for range b.N {
						var output *store.Batch
						if compact {
							state := makeCompactGroupState(step)
							for _, batch := range batches {
								if !state.add(a, batch, step, math.MaxInt64) {
									b.Fatal("compact grouping failed")
								}
							}
							groupCount, stateBytes = state.length, state.charged()
							output = state.finish(a, step)
							state.release()
						} else {
							state := &groupState{}
							for _, batch := range batches {
								if !state.add(a, batch, step, math.MaxInt64) {
									b.Fatal("baseline grouping failed")
								}
							}
							groupCount, stateBytes = len(state.values), state.charged(len(step.aggregates))
							output = state.finish(a, step)
							state.release()
						}
						if output == nil || output.Len() != groupCount {
							b.Fatal("group output mismatch")
						}
						output.Release()
					}
					b.StopTimer()
					if groupCount > 0 {
						b.ReportMetric(float64(stateBytes)/float64(groupCount), "estimated-state-B/group")
					}
					plan.Release()
					table.Release()
					for _, batch := range batches {
						batch.Release()
					}
				})
			}
		}
	}
}
