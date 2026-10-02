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

func BenchmarkGroupedSumState(b *testing.B) {
	const rows = 32768
	const batchSize = 1024
	for _, keyKind := range []string{"numeric", "inline-string", "long-string"} {
		for _, cardinality := range []int{16, 4096, rows} {
			for _, fraction := range []int{1, 8} {
				b.Run(fmt.Sprintf("%s/groups=%d/selected=1in%d", keyKind, cardinality, fraction), func(b *testing.B) {
					a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
					rng := rand.New(rand.NewPCG(91, 37))
					keyType := dtype.Int64T()
					if keyKind != "numeric" {
						keyType = dtype.StringT()
					}
					var batches []*store.Batch
					var want int64
					var active int64
					for start := 0; start < rows; start += batchSize {
						value := store.MakeVector(a, batchSize, dtype.Int64T(), true)
						key := store.MakeVector(a, batchSize, keyType, true)
						var texts [][]byte
						var valid []bool
						if keyKind != "numeric" {
							key.Release()
							texts = make([][]byte, batchSize)
							valid = make([]bool, batchSize)
						}
						selection := store.MakeBitMap(a, batchSize)
						for row := range batchSize {
							id := rng.IntN(cardinality)
							if texts == nil {
								key.I64s()[row] = int64(id)
								if (start+row)%113 == 0 {
									key.Validity().Clear(row)
								}
							} else {
								text := fmt.Sprintf("k%06d", id)
								if keyKind == "long-string" {
									text = fmt.Sprintf("long grouping key with utf8 ø %06d and a shared prefix", id)
								}
								texts[row] = []byte(text)
								valid[row] = (start+row)%113 != 0
							}
							value.I64s()[row] = int64(rng.IntN(10001) - 5000)
							if (start+row)%17 == 0 {
								value.Validity().Clear(row)
							}
							if (start+row)%fraction == 0 {
								selection.Set(row)
								active++
								if value.Validity().IsSet(row) {
									want += value.I64s()[row]
								}
							}
						}
						if texts != nil {
							key = store.MakeStringVector(a, texts, valid)
						}
						batch := store.MakeBatch([]store.Vector{key, value})
						batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
						batches = append(batches, batch)
					}
					table := store.MakeTable(batches)
					p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{keyType, dtype.Int64T()}, []bool{true, true})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeSum(MakeColumn("value"))))
					if err != nil {
						b.Fatal(err)
					}
					defer p.Release()
					defer table.Release()
					defer func() {
						for _, batch := range batches {
							batch.Release()
						}
					}()
					for _, compact := range []bool{false, true} {
						name := "general"
						if compact {
							name = "dispatch"
						}
						b.Run(name, func(b *testing.B) {
							b.ReportAllocs()
							b.SetBytes(active * 16)
							b.ResetTimer()
							var charged int64
							var groups int
							for range b.N {
								var state *groupState
								if !compact {
									state = &groupState{}
								} else {
									state = makeGroupState(p.steps[0], nil)
								}
								for _, batch := range batches {
									if !state.add(a, batch, p.steps[0], math.MaxInt64) {
										b.Fatal("grouping failed")
									}
								}
								charged = state.charged(2)
								output := state.finish(a, p.steps[0])
								state.release()
								if output == nil {
									b.Fatal("no output")
								}
								groups = output.Len()
								var sum, count int64
								for row := range output.Len() {
									count += output.VectorAt(1).I64s()[row]
									if output.VectorAt(2).Validity().IsSet(row) {
										sum += output.VectorAt(2).I64s()[row]
									}
								}
								output.Release()
								if sum != want || count != active {
									b.Fatalf("sum/count %d/%d, want %d/%d", sum, count, want, active)
								}
							}
							if groups > 0 {
								b.ReportMetric(float64(charged)/float64(groups), "state-B/group")
							}
						})
					}
				})
			}
		}
	}
}
