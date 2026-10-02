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

func TestCompactTypedGroupDifferential(t *testing.T) {
	for _, keyType := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.DateT(), dtype.TimestampTZT(), dtype.Float32T(), dtype.Float64T(), dtype.BoolT()} {
		for _, valueType := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T()} {
			t.Run(fmt.Sprintf("%s/%s", keyType, valueType), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
				rng := rand.New(rand.NewPCG(uint64(keyType.ID()), uint64(valueType.ID())))
				var batches []*store.Batch
				for part := range 4 {
					key := store.MakeVector(a, 257, keyType, true)
					value := store.MakeVector(a, 257, valueType, true)
					var offsets []uint32
					selection := store.MakeBitMap(a, 257)
					for row := range 257 {
						k := rng.IntN(23) - 11
						switch keyType.ID() {
						case dtype.INT32T, dtype.DATET:
							key.I32s()[row] = int32(k)
						case dtype.INT64T, dtype.TIMESTAMPTZT:
							key.I64s()[row] = int64(k)
						case dtype.FLOAT32T:
							key.F32s()[row] = float32(k)
							if row%19 == 0 {
								key.F32s()[row] = math.Float32frombits(0x7fc00000 | uint32(row))
							}
							if row%29 == 0 {
								key.F32s()[row] = float32(math.Copysign(0, -1))
							}
						case dtype.FLOAT64T:
							key.F64s()[row] = float64(k)
							if row%19 == 0 {
								key.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(row))
							}
							if row%29 == 0 {
								key.F64s()[row] = math.Copysign(0, -1)
							}
						case dtype.BOOLT:
							if k&1 != 0 {
								key.Bools()[row>>3] |= 1 << (row & 7)
							}
						}
						if row%13 == 0 {
							key.Validity().Clear(row)
						}
						switch valueType.ID() {
						case dtype.INT32T:
							value.I32s()[row] = []int32{math.MinInt32, math.MaxInt32, -1, 0, 17}[row%5]
						case dtype.INT64T:
							value.I64s()[row] = []int64{math.MinInt64, math.MaxInt64, -1, 0, 17}[row%5]
						case dtype.FLOAT32T:
							value.F32s()[row] = float32([]float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1), 0, 1, -2, 0.5}[row%8])
						case dtype.FLOAT64T:
							value.F64s()[row] = []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1), 0, 1, -2, 0.5}[row%8]
						}
						if row%11 == 0 {
							value.Validity().Clear(row)
						}
						if part == 3 || row%7 == 0 {
							continue
						}
						selection.Set(row)
						offsets = append(offsets, uint32(row))
					}
					batch := store.MakeBatch([]store.Vector{key, value})
					if part%2 == 0 {
						batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
					} else {
						selection.Release()
						batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 257, offsets)))
					}
					batches = append(batches, batch)
				}
				table := store.MakeTable(batches)
				p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{keyType, valueType}, []bool{true, true})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeCount(MakeColumn("value")), MakeSum(MakeColumn("value"))))
				if err != nil {
					t.Fatal(err)
				}
				compact, general := makeCompactGroupState(p.steps[0]), &groupState{}
				for _, batch := range batches {
					if !compact.add(a, batch, p.steps[0], math.MaxInt64) || !general.add(a, batch, p.steps[0], math.MaxInt64) {
						t.Fatal("grouping failed")
					}
				}
				got, want := compact.finish(a, p.steps[0]), general.finish(a, p.steps[0])
				if got == nil || want == nil || got.Len() != want.Len() {
					t.Fatal("different group count")
				}
				for row := range got.Len() {
					for column := range got.NVectors() {
						left, right := scalarAt(got.VectorAt(column), row), scalarAt(want.VectorAt(column), row)
						if left.IsNull() != right.IsNull() || !left.IsNull() && left.bits != right.bits && !(left.Type().IsFloating() && math.IsNaN(left.f64()) && math.IsNaN(right.f64())) {
							t.Fatalf("row %d column %d differs: %v/%v", row, column, left, right)
						}
					}
				}
				got.Release()
				want.Release()
				compact.release()
				general.release()
				p.Release()
				table.Release()
				for _, batch := range batches {
					batch.Release()
				}
				if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("leaked storage: %+v", usage)
				}
			})
		}
	}
}

func TestCompactDenseFallbackMergeAndBounds(t *testing.T) {
	for _, keyType := range []dtype.Type{dtype.Int32T(), dtype.DateT(), dtype.Int64T(), dtype.TimestampTZT()} {
		for _, base := range []int64{math.MinInt32, -67, math.MaxInt32 - 31, math.MinInt64, math.MaxInt64 - 31} {
			if (keyType.ID() == dtype.INT32T || keyType.ID() == dtype.DATET) && (base < math.MinInt32 || base > math.MaxInt32) {
				continue
			}
			t.Run(fmt.Sprintf("%s/%d", keyType, base), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
				var batches []*store.Batch
				for part := range 3 {
					key := store.MakeVector(a, 129, keyType, true)
					value := store.MakeVector(a, 129, dtype.Int64T(), false)
					for row := range 129 {
						v := base + int64((row*17)%32)
						if part == 1 {
							v = int64(math.MinInt64 + row)
						}
						if keyType.ID() == dtype.INT32T || keyType.ID() == dtype.DATET {
							key.I32s()[row] = int32(v)
						} else {
							key.I64s()[row] = v
						}
						value.I64s()[row] = int64(row - 64)
						if row%17 == 0 {
							key.Validity().Clear(row)
						}
					}
					batches = append(batches, store.MakeBatch([]store.Vector{key, value}))
				}
				table := store.MakeTable(batches)
				p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{keyType, dtype.Int64T()}, []bool{true, false})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeSum(MakeColumn("value"))))
				if err != nil {
					t.Fatal(err)
				}
				step := p.steps[0]
				state, src, reference := makeCompactGroupState(step), makeCompactGroupState(step), makeCompactGroupState(step)
				if !state.add(a, batches[0], step, math.MaxInt64) || state.dense == nil || state.index.slots != nil {
					t.Fatal("bounded domain did not select dense lookup")
				}
				if !src.add(a, batches[1], step, math.MaxInt64) || !state.merge(a, src, math.MaxInt64) {
					t.Fatal("merge failed")
				}
				// The INT32 cast of the wide second batch can overlap the first domain.
				if base != math.MinInt32 && state.dense != nil {
					t.Fatal("wide keys did not fall back to hashing")
				}
				if !state.add(a, batches[2], step, math.MaxInt64) {
					t.Fatal("post-merge accumulation failed")
				}
				for _, batch := range batches {
					if !reference.add(a, batch, step, math.MaxInt64) {
						t.Fatal("reference failed")
					}
				}
				got, want := state.finish(a, step), reference.finish(a, step)
				if got == nil || want == nil || got.Len() != want.Len() {
					t.Fatal("merge group count differs")
				}
				for row := range got.Len() {
					for col := range got.NVectors() {
						left, right := scalarAt(got.VectorAt(col), row), scalarAt(want.VectorAt(col), row)
						if left.IsNull() != right.IsNull() || !left.IsNull() && left.bits != right.bits {
							t.Fatalf("merge row %d column %d differs", row, col)
						}
					}
				}
				got.Release()
				want.Release()
				state.release()
				src.release()
				reference.release()
				p.Release()
				table.Release()
				for _, batch := range batches {
					batch.Release()
				}
				if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("leaked storage: %+v", usage)
				}
			})
		}
	}
}
