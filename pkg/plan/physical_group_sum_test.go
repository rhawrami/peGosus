package plan

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestCompactGroupedSumDifferential(t *testing.T) {
	for _, keyType := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.DateT(), dtype.TimestampTZT(), dtype.BoolT()} {
		for _, valueType := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T()} {
			for _, length := range []int{0, 1, 9, 65, 257} {
				t.Run(fmt.Sprintf("%s/%s/%d", keyType, valueType, length), func(t *testing.T) {
					a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
					rng := rand.New(rand.NewPCG(uint64(keyType.ID()), uint64(valueType.ID())+uint64(length)))
					type identity struct {
						null bool
						bits uint64
					}
					type expected struct {
						count, nonNull, included, isum int64
						fsum                           float64
					}
					want := make(map[identity]expected)
					identify := func(v *store.Vector, row int) identity {
						if v.Validity() != nil && !v.Validity().IsSet(row) {
							return identity{null: true}
						}
						var bits uint64
						switch keyType.ID() {
						case dtype.INT32T, dtype.DATET:
							bits = uint64(uint32(v.I32s()[row]))
						case dtype.INT64T, dtype.TIMESTAMPTZT:
							bits = uint64(v.I64s()[row])
						case dtype.FLOAT32T:
							f := v.F32s()[row]
							bits = uint64(math.Float32bits(f))
							if math.IsNaN(float64(f)) {
								bits = 0x7fc00000
							} else if f == 0 {
								bits = 0
							}
						case dtype.FLOAT64T:
							f := v.F64s()[row]
							bits = math.Float64bits(f)
							if math.IsNaN(f) {
								bits = 0x7ff8000000000000
							} else if f == 0 {
								bits = 0
							}
						case dtype.BOOLT:
							bits = uint64(v.Bools()[row>>3] >> (row & 7) & 1)
						}
						return identity{bits: bits}
					}
					var batches []*store.Batch
					for start := 0; start < length || start == 0; start += 17 {
						n := min(17, length-start)
						key := store.MakeVector(a, n, keyType, true)
						value := store.MakeVector(a, n, valueType, true)
						bitmap := store.MakeBitMap(a, n)
						var offsets []uint32
						for row := range n {
							at := start + row
							k := rng.IntN(11) - 5
							switch keyType.ID() {
							case dtype.INT32T, dtype.DATET:
								key.I32s()[row] = int32(k)
							case dtype.INT64T, dtype.TIMESTAMPTZT:
								key.I64s()[row] = int64(k)
							case dtype.FLOAT32T:
								key.F32s()[row] = float32(k)
								if at%19 == 0 {
									key.F32s()[row] = math.Float32frombits(0x7fc00000 | uint32(at))
								} else if at%23 == 0 {
									key.F32s()[row] = float32(math.Copysign(0, -1))
								} else if at%29 == 0 {
									key.F32s()[row] = float32(math.Inf(1))
								}
							case dtype.FLOAT64T:
								key.F64s()[row] = float64(k)
								if at%19 == 0 {
									key.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(at))
								} else if at%23 == 0 {
									key.F64s()[row] = math.Copysign(0, -1)
								} else if at%29 == 0 {
									key.F64s()[row] = math.Inf(-1)
								}
							case dtype.BOOLT:
								if k&1 != 0 {
									key.Bools()[row>>3] |= 1 << (row & 7)
								}
							}
							if at%13 == 0 {
								key.Validity().Clear(row)
							}
							var iv int64
							var fv float64
							switch valueType.ID() {
							case dtype.INT32T:
								iv = int64([]int32{math.MinInt32, math.MaxInt32, -1, 0, 1, 17}[at%6])
								value.I32s()[row] = int32(iv)
							case dtype.INT64T:
								iv = []int64{math.MinInt64, math.MaxInt64, -1, 0, 1, 17}[at%6]
								value.I64s()[row] = iv
							case dtype.FLOAT32T, dtype.FLOAT64T:
								fv = []float64{math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1), 0, 1, -2, 16, 0.5}[at%9]
								if k == 3 {
									fv = math.NaN()
								}
								if valueType.ID() == dtype.FLOAT32T {
									value.F32s()[row] = float32(fv)
									fv = float64(value.F32s()[row])
								} else {
									value.F64s()[row] = fv
								}
							}
							if at%7 == 0 || k == 2 {
								value.Validity().Clear(row)
							}
							if at%5 == 2 {
								continue
							}
							bitmap.Set(row)
							offsets = append(offsets, uint32(row))
							id := identify(&key, row)
							state := want[id]
							state.count++
							if value.Validity().IsSet(row) {
								state.nonNull++
								if !valueType.IsFloating() {
									state.isum += iv
									state.included++
								} else if !math.IsNaN(fv) {
									state.fsum += fv
									state.included++
								}
							}
							want[id] = state
						}
						batch := store.MakeBatch([]store.Vector{key, value})
						if start%2 == 0 {
							batch.SetSelection(store.MakeRowSelectionFromBitMap(bitmap))
						} else {
							bitmap.Release()
							batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, n, offsets)))
						}
						batches = append(batches, batch)
					}
					table := store.MakeTable(batches)
					schema := MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{keyType, valueType}, []bool{true, true})
					p, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeCount(MakeColumn("value")), MakeSum(MakeColumn("value"))))
					if err != nil {
						t.Fatal(err)
					}
					if makeCompactGroupState(p.steps[0]) == nil {
						t.Fatal("SUM missed compact dispatch")
					}
					check := func(batch *store.Batch, seen map[identity]bool) {
						t.Helper()
						for row := range batch.Len() {
							id := identify(batch.VectorAt(0), row)
							state, exists := want[id]
							valid := batch.VectorAt(3).Validity() == nil || batch.VectorAt(3).Validity().IsSet(row)
							if !exists || seen[id] || batch.VectorAt(1).I64s()[row] != state.count || batch.VectorAt(2).I64s()[row] != state.nonNull || valid != (state.included != 0) {
								t.Fatalf("group %v: incorrect count/validity or duplicate; want %+v", id, state)
							}
							seen[id] = true
							if valid {
								if valueType.IsFloating() {
									got := batch.VectorAt(3).F64s()[row]
									if !(math.IsNaN(got) && math.IsNaN(state.fsum)) && math.Float64bits(got) != math.Float64bits(state.fsum) {
										t.Fatalf("group %v: sum %v want %v", id, got, state.fsum)
									}
								} else if got := batch.VectorAt(3).I64s()[row]; got != state.isum {
									t.Fatalf("group %v: sum %d want %d", id, got, state.isum)
								}
							}
						}
					}
					for _, workers := range []int{1, 4, 8} {
						seen := make(map[identity]bool)
						result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8 << 20, Workers: workers}, func(output *store.Batch) bool { check(output, seen); return true })
						if result.Code() != ExecutionCompleted || len(seen) != len(want) {
							t.Fatalf("workers %d: %v/%v groups %d/%d", workers, result.Code(), result.Err(), len(seen), len(want))
						}
					}
					baseline := &groupState{}
					for _, batch := range batches {
						if !baseline.add(a, batch, p.steps[0], math.MaxInt64) {
							t.Fatal("general grouping failed")
						}
					}
					output := baseline.finish(a, p.steps[0])
					seen := make(map[identity]bool)
					check(output, seen)
					if len(seen) != len(want) {
						t.Fatal("general grouping dropped groups")
					}
					output.Release()
					baseline.release()
					p.Release()
					table.Release()
					for _, batch := range batches {
						batch.Release()
					}
					if used := a.Usage(); used.GeneralUsed != 0 || used.ScratchUsed != 0 {
						t.Fatalf("leaked allocation: %+v", used)
					}
				})
			}
		}
	}
}

func TestCompactGroupedSumFloatOrder(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	key := store.MakeVector(a, 6, dtype.Int64T(), false)
	value := store.MakeVector(a, 6, dtype.Float64T(), false)
	copy(value.F64s(), []float64{1e20, 1, -1e20, 3, math.Copysign(0, -1), math.NaN()})
	batch := store.MakeBatch([]store.Vector{key, value})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "value"}, []dtype.Type{dtype.Int64T(), dtype.Float64T()})).GroupBy([]Expr{MakeColumn("key")}, MakeSum(MakeColumn("value"))))
	if err != nil {
		t.Fatal(err)
	}
	result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20, Workers: 1}, func(out *store.Batch) bool {
		if got := out.VectorAt(1).F64s()[0]; got != 3 {
			t.Fatalf("reassociated SUM: %v", got)
		}
		return true
	})
	if result.Code() != ExecutionCompleted {
		t.Fatal(result.Err())
	}
	p.Release()
	table.Release()
	batch.Release()
}

func TestCompactGroupedSumBudgetOwnershipAndCancellation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	var batches []*store.Batch
	for chunk := range 8 {
		key := store.MakeVector(a, 65, dtype.Int64T(), false)
		value := store.MakeVector(a, 65, dtype.Int64T(), true)
		for row := range 65 {
			key.I64s()[row] = int64(chunk*65 + row)
			value.I64s()[row] = int64(row - 32)
			if row%11 == 0 {
				value.Validity().Clear(row)
			}
		}
		batches = append(batches, store.MakeBatch([]store.Vector{key, value}))
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{dtype.Int64T(), dtype.Int64T()}, []bool{false, true})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeSum(MakeColumn("value"))))
	if err != nil {
		t.Fatal(err)
	}
	for budget := int64(0); budget < 4096; budget += 32 {
		scope := mem.MakeAllocationScope(a, budget)
		view := mem.MakeAllocatorWithScope(a, scope)
		state := makeGroupState(p.steps[0], scope)
		for batchIndex := range table.NBatches() {
			if !state.add(view, table.BatchAt(batchIndex), p.steps[0], math.MaxInt64) {
				break
			}
		}
		state.release()
		if scope.Live() != 0 {
			t.Fatalf("failed allocation left %d bytes at budget %d", scope.Live(), budget)
		}
	}
	for _, workers := range []int{1, 4} {
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096, Workers: workers}, func(*store.Batch) bool { t.Fatal("under-budget grouping reached sink"); return true })
		if result.Code() != ExecutionResourceExhausted {
			t.Fatalf("small budget workers %d: %v", workers, result.Code())
		}
		result = p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20, Workers: workers}, func(*store.Batch) bool { return false })
		if result.Code() != ExecutionStopped {
			t.Fatalf("sink stop workers %d: %v", workers, result.Code())
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result = p.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 1 << 20, Workers: workers}, func(*store.Batch) bool { t.Fatal("cancelled grouping reached sink"); return true })
		if result.Code() != ExecutionCancelled {
			t.Fatalf("cancelled workers %d: %v", workers, result.Code())
		}
	}
	var output *store.Batch
	result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20, Workers: 4}, func(batch *store.Batch) bool { output = batch.Retain(); return true })
	if result.Code() != ExecutionCompleted || output == nil || output.Len() != 520 {
		t.Fatalf("retained output: %v", result.Code())
	}
	p.Release()
	table.Release()
	for row := range output.Len() {
		id := output.VectorAt(0).I64s()[row]
		value := id%65 - 32
		valid := id%65%11 != 0
		if output.VectorAt(1).I64s()[row] != 1 || output.VectorAt(2).Validity().IsSet(row) != valid || valid && output.VectorAt(2).I64s()[row] != value {
			t.Fatalf("retained group %d changed", id)
		}
	}
	output.Release()
	if used := a.Usage(); used.GeneralUsed != 0 || used.ScratchUsed != 0 {
		t.Fatalf("leaked allocation: %+v", used)
	}
}

func TestCompactGroupedSumConcurrentReuse(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	var batches []*store.Batch
	for chunk := range 8 {
		key := store.MakeVector(a, 129, dtype.Int32T(), false)
		value := store.MakeVector(a, 129, dtype.Int64T(), false)
		for row := range 129 {
			key.I32s()[row] = int32(row % 3)
			value.I64s()[row] = int64(chunk + 1)
		}
		batches = append(batches, store.MakeBatch([]store.Vector{key, value}))
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "value"}, []dtype.Type{dtype.Int32T(), dtype.Int64T()})).GroupBy([]Expr{MakeColumn("key")}, MakeSum(MakeColumn("value"))))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20, Workers: 4}, func(out *store.Batch) bool {
				if out.Len() != 3 {
					t.Error("wrong group count")
				}
				for _, sum := range out.VectorAt(1).I64s() {
					if sum != 43*36 {
						t.Errorf("concurrent sum %d", sum)
					}
				}
				return true
			})
			if result.Code() != ExecutionCompleted {
				t.Errorf("concurrent execution %v", result.Code())
			}
		}()
	}
	wg.Wait()
	p.Release()
	if used := a.Usage(); used.GeneralUsed != 0 || used.ScratchUsed != 0 {
		t.Fatalf("leaked allocation: %+v", used)
	}
}

func TestCompactGroupedSumMixedGrowthMerge(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	types := []dtype.Type{dtype.Int64T(), dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.StringT()}
	names := []string{"key", "i32", "i64", "f32", "f64", "text"}
	var batches []*store.Batch
	for chunk := range 4 {
		vectors := make([]store.Vector, len(types))
		for col := range 5 {
			vectors[col] = store.MakeVector(a, 129, types[col], true)
		}
		texts := make([][]byte, 129)
		valid := make([]bool, 129)
		for row := range 129 {
			id := (chunk*129 + row) % 257
			vectors[0].I64s()[row] = int64(id)
			vectors[1].I32s()[row] = int32(id - 128)
			vectors[2].I64s()[row] = int64(chunk + 1)
			vectors[3].F32s()[row] = float32(id) / 2
			vectors[4].F64s()[row] = float64(chunk) / 4
			texts[row] = []byte("independently owned long COUNT input")
			valid[row] = id%7 != 0
			if id%11 == 0 {
				vectors[1].Validity().Clear(row)
			}
			if id%13 == 0 {
				vectors[3].F32s()[row] = float32(math.NaN())
			}
			if id%17 == 0 {
				vectors[4].Validity().Clear(row)
			}
		}
		vectors[5] = store.MakeStringVector(a, texts, valid)
		batches = append(batches, store.MakeBatch(vectors))
	}
	table := store.MakeTable(batches)
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema(names, types)).GroupBy([]Expr{MakeColumn("key")}, MakeSum(MakeColumn("i32").Add(MakeLiteral(int32(1)))), MakeSum(MakeColumn("i64")), MakeSum(MakeColumn("f32")), MakeSum(MakeColumn("f64")), MakeCountStar(), MakeCount(MakeColumn("text"))))
	if err != nil {
		t.Fatal(err)
	}
	step := p.steps[0]
	if makeCompactGroupState(step) == nil {
		t.Fatal("mixed SUM plan missed compact path")
	}
	for _, kind := range []AggregateKind{AggregateAvg, AggregateMin, AggregateMax} {
		unsupported := step
		unsupported.aggregates = append([]physicalAggregateExpr(nil), step.aggregates...)
		unsupported.aggregates[0].kind = kind
		if makeCompactGroupState(unsupported) != nil {
			t.Fatalf("accepted unsupported kind %v", kind)
		}
	}
	distinct := step
	distinct.aggregates = append([]physicalAggregateExpr(nil), step.aggregates...)
	distinct.aggregates[0].distinct = true
	if makeCompactGroupState(distinct) != nil {
		t.Fatal("accepted distinct SUM")
	}
	scope := mem.MakeAllocationScope(a, 1<<20)
	view := mem.MakeAllocatorWithScope(a, scope)
	first, second := makeGroupState(step, scope), makeGroupState(step, scope)
	referenceFirst, referenceSecond := &groupState{}, &groupState{}
	for i, batch := range batches {
		actual, reference := first, referenceFirst
		if i%2 != 0 {
			actual, reference = second, referenceSecond
		}
		if !actual.add(view, batch, step, math.MaxInt64) || !reference.add(a, batch, step, math.MaxInt64) {
			t.Fatal("grouping failed")
		}
	}
	if !first.merge(view, second, step, math.MaxInt64) || !referenceFirst.merge(a, referenceSecond, step, math.MaxInt64) {
		t.Fatal("merge failed")
	}
	second.release()
	referenceSecond.release()
	output, reference := first.finish(view, step), referenceFirst.finish(a, step)
	first.release()
	referenceFirst.release()
	p.Release()
	table.Release()
	for _, batch := range batches {
		batch.Release()
	}
	if output == nil || reference == nil || output.Len() != 257 || reference.Len() != 257 {
		t.Fatal("growth lost groups")
	}
	rows := make(map[int64]int)
	for row := range reference.Len() {
		rows[reference.VectorAt(0).I64s()[row]] = row
	}
	for row := range output.Len() {
		expectedRow, found := rows[output.VectorAt(0).I64s()[row]]
		if !found {
			t.Fatal("unknown group")
		}
		for col := range output.NVectors() {
			got, want := scalarAt(output.VectorAt(col), row), scalarAt(reference.VectorAt(col), expectedRow)
			if got.IsNull() != want.IsNull() || !got.IsNull() && got.bits != want.bits {
				t.Fatalf("mixed group %d col %d: got %v want %v", row, col, got, want)
			}
		}
	}
	if scope.Live() == 0 {
		t.Fatal("retained output lost its scope")
	}
	output.Release()
	reference.Release()
	if scope.Live() != 0 {
		t.Fatalf("mixed SUM leaked %d bytes", scope.Live())
	}
}
