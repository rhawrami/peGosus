package plan

import (
	"context"
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestCompactTwoKeyTopNMatchesScalarSort(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	rng := rand.New(rand.NewPCG(167, 251))
	type record struct {
		first                 float64
		second                int64
		id                    int32
		nullFirst, nullSecond bool
	}
	var batches []*store.Batch
	var active []record
	values := []float64{math.Inf(-1), math.Copysign(0, -1), 0, 1.5, math.NaN(), math.Inf(1), -3}
	for start := 0; start < 257; start += 19 {
		length := min(19, 257-start)
		first := store.MakeVector(a, length, dtype.Float64T(), true)
		second := store.MakeVector(a, length, dtype.Int64T(), true)
		ids := store.MakeVector(a, length, dtype.Int32T(), false)
		mask := store.MakeBitMap(a, length)
		var offsets []uint32
		for row := range length {
			id := start + row
			first.F64s()[row] = values[rng.IntN(len(values))]
			second.I64s()[row] = int64(rng.IntN(9) - 4)
			ids.I32s()[row] = int32(id)
			if id%13 == 0 {
				first.Validity().Clear(row)
			}
			if id%11 == 0 {
				second.Validity().Clear(row)
			}
			if id%7 == 3 {
				continue
			}
			mask.Set(row)
			offsets = append(offsets, uint32(row))
			active = append(active, record{first: first.F64s()[row], second: second.I64s()[row], id: int32(id), nullFirst: !first.Validity().IsSet(row), nullSecond: !second.Validity().IsSet(row)})
		}
		batch := store.MakeBatch([]store.Vector{first, second, ids})
		if start%2 == 0 {
			batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
		} else {
			mask.Release()
			batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, length, offsets)))
		}
		batches = append(batches, batch)
	}
	table := store.MakeTable(batches)
	for _, firstDesc := range []bool{false, true} {
		for _, firstNulls := range []bool{false, true} {
			for _, secondDesc := range []bool{false, true} {
				for _, secondNulls := range []bool{false, true} {
					ordered := append([]record(nil), active...)
					sort.SliceStable(ordered, func(i, j int) bool {
						x, y := ordered[i], ordered[j]
						if x.nullFirst != y.nullFirst {
							return x.nullFirst == firstNulls
						}
						if !x.nullFirst {
							compare := 0
							if math.IsNaN(x.first) {
								if !math.IsNaN(y.first) {
									compare = 1
								}
							} else if math.IsNaN(y.first) {
								compare = -1
							} else if x.first < y.first {
								compare = -1
							} else if x.first > y.first {
								compare = 1
							}
							if compare != 0 {
								if firstDesc {
									return compare > 0
								}
								return compare < 0
							}
						}
						if x.nullSecond != y.nullSecond {
							return x.nullSecond == secondNulls
						}
						if !x.nullSecond && x.second != y.second {
							if secondDesc {
								return x.second > y.second
							}
							return x.second < y.second
						}
						return x.id < y.id
					})
					for _, offset := range []int64{0, 7} {
						for _, count := range []int64{0, 1, 9, 27} {
							firstKey := MakeOrderKey(MakeColumn("first"))
							secondKey := MakeOrderKey(MakeColumn("second"))
							if firstDesc {
								firstKey = firstKey.Desc()
							}
							if firstNulls {
								firstKey = firstKey.NullsFirst()
							}
							if secondDesc {
								secondKey = secondKey.Desc()
							}
							if secondNulls {
								secondKey = secondKey.NullsFirst()
							}
							p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"first", "second", "id"}, []dtype.Type{dtype.Float64T(), dtype.Int64T(), dtype.Int32T()}, []bool{true, true, false})).OrderBy(firstKey, secondKey).Limit(count, offset))
							if err != nil {
								t.Fatal(err)
							}
							if makeCompactTopNState(p.steps[0]) == nil {
								t.Fatal("two-key numeric TopN missed compact heap")
							}
							var got []int32
							result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 16}, func(output *store.Batch) bool {
								selection := output.Selection().MakeBitMapTemp(a)
								for row := range output.Len() {
									if selection == nil || selection.IsSet(row) {
										got = append(got, output.VectorAt(2).I32s()[row])
									}
								}
								if selection != nil {
									selection.Release()
								}
								return true
							})
							p.Release()
							if result.Code() != ExecutionCompleted {
								t.Fatalf("two-key TopN: %v/%v", result.Code(), result.Err())
							}
							begin := min(int(offset), len(ordered))
							end := min(int(offset+count), len(ordered))
							if len(got) != end-begin {
								t.Fatalf("TopN count %d offset %d: returned %d rows, want %d", count, offset, len(got), end-begin)
							}
							for i, want := range ordered[begin:end] {
								if got[i] != want.id {
									t.Fatalf("desc %t/%t nulls %t/%t count %d offset %d row %d: id %d want %d", firstDesc, secondDesc, firstNulls, secondNulls, count, offset, i, got[i], want.id)
								}
							}
						}
					}
				}
			}
		}
	}
	table.Release()
	for _, batch := range batches {
		batch.Release()
	}
}
