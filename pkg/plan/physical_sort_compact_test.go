package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestCompactSortDifferential(t *testing.T) {
	for _, keyType := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.DateT(), dtype.TimestampTZT()} {
		for _, descending := range []bool{false, true} {
			for _, nullsFirst := range []bool{false, true} {
				for _, length := range []int{0, 1, 7, 8, 9, 37, 129, 1025} {
					t.Run(fmt.Sprintf("%s/desc=%t/nullsFirst=%t/n=%d", keyType, descending, nullsFirst, length), func(t *testing.T) {
						a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
						rng := rand.New(rand.NewPCG(uint64(keyType.ID()), uint64(length)+1))
						var batches []*store.Batch
						keys := make([]Scalar, length)
						var active []int
						for start := 0; start < length || start == 0; start += 17 {
							n := min(17, length-start)
							key := store.MakeVector(a, n, keyType, true)
							ids := store.MakeVector(a, n, dtype.Int32T(), false)
							flags := store.MakeVector(a, n, dtype.BoolT(), true)
							selection := store.MakeBitMap(a, n)
							var offsets []uint32
							for row := range n {
								i := start + row
								value := int32(rng.IntN(23) - 11)
								if i%17 == 0 {
									value = 0
								}
								switch keyType.ID() {
								case dtype.INT32T, dtype.DATET:
									key.I32s()[row] = value
								case dtype.INT64T, dtype.TIMESTAMPTZT:
									key.I64s()[row] = int64(value)
								case dtype.FLOAT32T:
									key.F32s()[row] = float32(value)
									if i%19 == 0 {
										key.F32s()[row] = float32(math.Inf(1))
									}
									if i%23 == 0 {
										key.F32s()[row] = math.Float32frombits(0x7fc00000 | uint32(i&15))
									}
									if i%17 == 0 {
										key.F32s()[row] = float32(math.Copysign(0, -1))
									}
								case dtype.FLOAT64T:
									key.F64s()[row] = float64(value)
									if i%19 == 0 {
										key.F64s()[row] = math.Inf(1)
									}
									if i%23 == 0 {
										key.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(i&15))
									}
									if i%17 == 0 {
										key.F64s()[row] = math.Copysign(0, -1)
									}
								}
								if i%11 == 0 {
									key.Validity().Clear(row)
									flags.Validity().Clear(row)
								}
								if i%3 == 0 {
									flags.Bools()[row>>3] |= 1 << (row & 7)
								}
								ids.I32s()[row] = int32(i)
								keys[i] = scalarAt(&key, row)
								if i%5 == 2 {
									continue
								}
								selection.Set(row)
								offsets = append(offsets, uint32(row))
								active = append(active, i)
							}
							batch := store.MakeBatch([]store.Vector{key, ids, flags})
							if start%2 == 0 {
								batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
							} else {
								selection.Release()
								batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, n, offsets)))
							}
							batches = append(batches, batch)
						}
						sort.SliceStable(active, func(i, j int) bool {
							left, right := keys[active[i]], keys[active[j]]
							if left.IsNull() || right.IsNull() {
								if left.IsNull() != right.IsNull() {
									return left.IsNull() == nullsFirst
								}
								return false
							}
							comparison := 0
							switch keyType.ID() {
							case dtype.INT32T, dtype.DATET:
								if left.i32() < right.i32() {
									comparison = -1
								} else if left.i32() > right.i32() {
									comparison = 1
								}
							case dtype.INT64T, dtype.TIMESTAMPTZT:
								if left.i64() < right.i64() {
									comparison = -1
								} else if left.i64() > right.i64() {
									comparison = 1
								}
							case dtype.FLOAT32T, dtype.FLOAT64T:
								x, y := left.f64(), right.f64()
								if keyType.ID() == dtype.FLOAT32T {
									x, y = float64(left.f32()), float64(right.f32())
								}
								if math.IsNaN(x) && !math.IsNaN(y) {
									comparison = 1
								} else if !math.IsNaN(x) && math.IsNaN(y) {
									comparison = -1
								} else if x < y {
									comparison = -1
								} else if x > y {
									comparison = 1
								}
							}
							if descending {
								comparison = -comparison
							}
							return comparison < 0
						})
						table := store.MakeTable(batches)
						order := MakeOrderKey(MakeColumn("key"))
						if descending {
							order = order.Desc()
						}
						if nullsFirst {
							order = order.NullsFirst()
						}
						plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "id", "flag"}, []dtype.Type{keyType, dtype.Int32T(), dtype.BoolT()})).OrderBy(order))
						if err != nil {
							t.Fatal(err)
						}
						step := plan.steps[0]
						if makeCompactSortState(step) == nil {
							t.Fatal("expected a compact sort")
						}
						check := func(name string, batch *store.Batch) {
							t.Helper()
							if batch == nil || batch.Len() != len(active) {
								t.Fatalf("%s: got %v rows, expected %d", name, batch, len(active))
							}
							for row, original := range active {
								got := scalarAt(batch.VectorAt(0), row)
								if batch.VectorAt(1).I32s()[row] != int32(original) || got.IsNull() != keys[original].IsNull() || !got.IsNull() && got.bits != keys[original].bits {
									t.Fatalf("%s row %d: got id=%d/key=%v, expected id=%d/key=%v", name, row, batch.VectorAt(1).I32s()[row], got, original, keys[original])
								}
								flag := batch.VectorAt(2)
								if (flag.Validity() == nil || flag.Validity().IsSet(row)) != (original%11 != 0) || (flag.Bools()[row>>3]&(1<<(row&7)) != 0) != (original%3 == 0) && original%11 != 0 {
									t.Fatalf("%s row %d: boolean payload/validity changed", name, row)
								}
							}
						}
						compact := makeCompactSortState(step)
						legacy := &sortState{}
						for _, batch := range batches {
							if !compact.add(a, batch, step, math.MaxInt64) || !legacy.add(a, batch, step, math.MaxInt64) {
								t.Fatal("sort failed")
							}
						}
						if length != 0 {
							compactOutput, legacyOutput := compact.finish(a, step), legacy.finish(a, step)
							check("compact", compactOutput)
							check("legacy", legacyOutput)
							compactOutput.Release()
							legacyOutput.Release()
						}
						compact.release()
						legacy.release()
						called := false
						result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20}, func(output *store.Batch) bool { called = true; check("pipeline", output); return true })
						if result.Code() != ExecutionCompleted || called != (len(active) > 0) {
							t.Fatalf("pipeline: result %v, called=%t", result.Code(), called)
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
}

func TestCompactSortStringsAndTopNFallback(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	strings := store.MakeStringVector(a, [][]byte{[]byte("a\x00"), []byte("z"), []byte("a"), []byte("a\x00\x00")}, nil)
	batch := store.MakeBatch([]store.Vector{strings})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	scan := MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{dtype.StringT()}))
	plan, err := MakePhysicalPlan(scan.OrderBy(MakeOrderKey(MakeColumn("key"))))
	if err != nil {
		t.Fatal(err)
	}
	if makeCompactSortState(plan.steps[0]) == nil {
		t.Fatal("string sort did not select compact state")
	}
	var output *store.Batch
	if result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(b *store.Batch) bool { output = b.Retain(); return true }); result.Code() != ExecutionCompleted {
		t.Fatalf("string sort failed: %v", result.Code())
	}
	plan.Release()
	table.Release()
	for i, expected := range []string{"a", "a\x00", "a\x00\x00", "z"} {
		if got := output.VectorAt(0).Strings()[i].View(); got != expected {
			t.Fatalf("row %d: got %q, expected %q", i, got, expected)
		}
	}
	output.Release()

	v := store.MakeVector(a, 1, dtype.Int32T(), false)
	batch = store.MakeBatch([]store.Vector{v})
	table = store.MakeTable([]*store.Batch{batch})
	batch.Release()
	plan, err = MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int32T()})).OrderBy(MakeOrderKey(MakeColumn("key"))).Limit(1))
	if err != nil {
		t.Fatal(err)
	}
	if makeCompactSortState(plan.steps[0]) != nil {
		t.Fatal("accepted TopN in the full radix sort path")
	}
	if makeCompactTopNState(plan.steps[0]) == nil {
		t.Fatal("fixed-width TopN did not select allocator-backed state")
	}
	plan.Release()
	table.Release()
}

func TestCompactTopNRandomizedDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 99))
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	var batches []*store.Batch
	type entry struct {
		key  int32
		id   int
		null bool
	}
	var active []entry
	for start := 0; start < 257; start += 17 {
		length := min(17, 257-start)
		key := store.MakeVector(a, length, dtype.Int32T(), true)
		ids := store.MakeVector(a, length, dtype.Int32T(), false)
		mask := store.MakeBitMap(a, length)
		var offsets []uint32
		for row := range length {
			i := start + row
			key.I32s()[row] = int32(rng.IntN(13) - 6)
			ids.I32s()[row] = int32(i)
			if i%11 == 0 {
				key.Validity().Clear(row)
			}
			if i%5 == 2 {
				continue
			}
			mask.Set(row)
			offsets = append(offsets, uint32(row))
			active = append(active, entry{key: key.I32s()[row], id: i, null: !key.Validity().IsSet(row)})
		}
		batch := store.MakeBatch([]store.Vector{key, ids})
		if start%2 == 0 {
			batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
		} else {
			mask.Release()
			batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, length, offsets)))
		}
		batches = append(batches, batch)
	}
	table := store.MakeTable(batches)
	for _, descending := range []bool{false, true} {
		for _, nullsFirst := range []bool{false, true} {
			want := append([]entry(nil), active...)
			sort.SliceStable(want, func(i, j int) bool {
				if want[i].null != want[j].null {
					return want[i].null == nullsFirst
				}
				if want[i].null {
					return false
				}
				if descending {
					return want[i].key > want[j].key
				}
				return want[i].key < want[j].key
			})
			for _, offset := range []int64{0, 7} {
				for _, count := range []int64{0, 1, 9, 27} {
					order := MakeOrderKey(MakeColumn("key"))
					if descending {
						order = order.Desc()
					}
					if nullsFirst {
						order = order.NullsFirst()
					}
					plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "id"}, []dtype.Type{dtype.Int32T(), dtype.Int32T()})).OrderBy(order).Limit(count, offset))
					if err != nil {
						t.Fatal(err)
					}
					if plan.steps[0].topN > 0 && makeCompactTopNState(plan.steps[0]) == nil {
						t.Fatal("no compact TopN state")
					}
					var got []int
					result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(output *store.Batch) bool {
						mask := output.Selection().MakeBitMapTemp(a)
						for row := range output.Len() {
							if mask == nil || mask.IsSet(row) {
								got = append(got, int(output.VectorAt(1).I32s()[row]))
							}
						}
						if mask != nil {
							mask.Release()
						}
						return true
					})
					plan.Release()
					if result.Code() != ExecutionCompleted {
						t.Fatalf("TopN result: %v", result.Code())
					}
					stop := min(len(want), int(offset+count))
					start := min(len(want), int(offset))
					if len(got) != stop-start {
						t.Fatalf("count=%d offset=%d: got %v, want %v", count, offset, got, want[start:stop])
					}
					for i, row := range want[start:stop] {
						if got[i] != row.id {
							t.Fatalf("desc=%t nullsFirst=%t count=%d offset=%d row=%d: got id %d, want %d", descending, nullsFirst, count, offset, i, got[i], row.id)
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
