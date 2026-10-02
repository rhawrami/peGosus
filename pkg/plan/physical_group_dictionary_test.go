package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestStringDictionaryGroupTypedDifferential(t *testing.T) {
	for _, typ := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T()} {
		for _, n := range []int{0, 1, 9, 65, 1025} {
			t.Run(fmt.Sprintf("%s/%d", typ, n), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
				rng := rand.New(rand.NewPCG(uint64(typ.ID()), uint64(n)+73))
				var batches []*store.Batch
				for part := range 4 {
					texts := [][]byte{[]byte(""), []byte("ø\x00"), []byte("common long dictionary key"), []byte("ø\x00"), []byte(fmt.Sprintf("part %d", part))}
					rng.Shuffle(len(texts), func(i, j int) { texts[i], texts[j] = texts[j], texts[i] })
					dictionary := store.MakeStringVector(a, texts, nil)
					indices := a.AllocSeg(n * 4)
					validity := store.MakeBitMap(a, n)
					flat, valid := make([][]byte, n), make([]bool, n)
					value := store.MakeVector(a, n, typ, true)
					selection := store.MakeBitMap(a, n)
					var offsets []uint32
					for row := range n {
						id := rng.IntN(len(texts))
						indices.AsU32T()[row] = uint32(id)
						flat[row], valid[row] = texts[id], row%11 != 0
						if valid[row] {
							validity.Set(row)
						} else {
							indices.AsU32T()[row] = math.MaxUint32
						}
						switch typ.ID() {
						case dtype.INT32T:
							value.I32s()[row] = []int32{math.MinInt32, math.MaxInt32, -1, 0, 1}[row%5]
						case dtype.INT64T:
							value.I64s()[row] = []int64{math.MinInt64, math.MaxInt64, -1, 0, 1}[row%5]
						case dtype.FLOAT32T:
							value.F32s()[row] = float32([]float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1), .25, -.5}[row%6])
						case dtype.FLOAT64T:
							value.F64s()[row] = []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1), .25, -.5}[row%6]
						}
						if row%13 == 0 || id == 0 {
							value.Validity().Clear(row)
						}
						if row%7 != 0 {
							selection.Set(row)
							offsets = append(offsets, uint32(row))
						}
					}
					key := store.MakeDictionaryStringVectorFromOwnedSegments(dictionary, n, indices, validity)
					if part == 2 {
						key.Release()
						key = store.MakeStringVector(a, flat, valid)
					}
					batch := store.MakeBatch([]store.Vector{key, value})
					if part%2 == 0 {
						batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
					} else {
						selection.Release()
						batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, n, offsets)))
					}
					batches = append(batches, batch)
				}
				table := store.MakeTable(batches)
				p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "value"}, []dtype.Type{dtype.StringT(), typ})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeCount(MakeColumn("value")), MakeSum(MakeColumn("value"))))
				if err != nil {
					t.Fatal(err)
				}
				step := p.steps[0]
				reference := &groupState{}
				reference.keys.index.stringKeys = true
				for _, batch := range batches {
					if !reference.add(a, batch, step, math.MaxInt64) {
						t.Fatal("reference failed")
					}
				}
				want := make(map[string][]Scalar)
				if reference.groupCount() != 0 {
					output := reference.finish(a, step)
					for row := range output.Len() {
						key := scalarAt(output.VectorAt(0), row)
						name := "null"
						if !key.IsNull() {
							name = "value:" + key.stringValue()
						}
						want[name] = []Scalar{scalarAt(output.VectorAt(1), row), scalarAt(output.VectorAt(2), row), scalarAt(output.VectorAt(3), row)}
					}
					output.Release()
				}
				reference.release()
				for _, workers := range []int{1, 4} {
					seen := make(map[string]bool)
					result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{Workers: workers, MemoryBudget: 8 << 20}, func(output *store.Batch) bool {
						for row := range output.Len() {
							key := scalarAt(output.VectorAt(0), row)
							name := "null"
							if !key.IsNull() {
								name = "value:" + key.stringValue()
							}
							expected, ok := want[name]
							if !ok || seen[name] {
								t.Fatalf("lost/duplicate group %q", name)
							}
							seen[name] = true
							for col, value := range expected {
								got := scalarAt(output.VectorAt(col+1), row)
								if got.IsNull() != value.IsNull() || !got.IsNull() && got.bits != value.bits && !(got.Type().ID() == dtype.FLOAT64T && math.IsNaN(got.f64()) && math.IsNaN(value.f64())) {
									t.Fatalf("group %q column %d: got %v want %v", name, col, got, value)
								}
							}
						}
						return true
					})
					if result.Code() != ExecutionCompleted || len(seen) != len(want) {
						t.Fatalf("execution: %v/%v, groups %d/%d", result.Code(), result.Err(), len(seen), len(want))
					}
				}
				p.Release()
				table.Release()
				for _, batch := range batches {
					batch.Release()
				}
				if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("leaked %+v", usage)
				}
			})
		}
	}
}
