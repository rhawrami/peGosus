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

func TestCompactGroupCountDifferential(t *testing.T) {
	for _, keyType := range []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.DateT(), dtype.TimestampTZT(), dtype.BoolT()} {
		for _, length := range []int{0, 1, 7, 8, 9, 67, 257} {
			t.Run(fmt.Sprintf("%s/%d", keyType, length), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
				rng := rand.New(rand.NewPCG(uint64(keyType.ID()), uint64(length)+1))
				var batches []*store.Batch
				type expectedGroup struct {
					key            Scalar
					count, nonNull int64
				}
				var want []expectedGroup
				positions := make(map[uint64]int)
				nullIndex := -1
				for start := 0; start < length || start == 0; start += 17 {
					n := min(17, length-start)
					key := store.MakeVector(a, n, keyType, true)
					selected := store.MakeBitMap(a, n)
					var offsets []uint32
					for row := range n {
						index := start + row
						isNull := index%13 == 0
						value := int32(rng.IntN(23) - 11)
						if index%29 == 0 {
							value = 0
						}
						switch keyType.ID() {
						case dtype.INT32T, dtype.DATET:
							key.I32s()[row] = value
						case dtype.INT64T, dtype.TIMESTAMPTZT:
							key.I64s()[row] = int64(value)
						case dtype.FLOAT32T:
							key.F32s()[row] = float32(value)
							if index%31 == 0 {
								key.F32s()[row] = math.Float32frombits(0x7fc00000 | uint32(index&15))
							}
							if index%29 == 0 {
								key.F32s()[row] = float32(math.Copysign(0, -1))
							}
						case dtype.FLOAT64T:
							key.F64s()[row] = float64(value)
							if index%31 == 0 {
								key.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(index&15))
							}
							if index%29 == 0 {
								key.F64s()[row] = math.Copysign(0, -1)
							}
						case dtype.BOOLT:
							if value&1 != 0 {
								key.Bools()[row>>3] |= 1 << (row & 7)
							}
						}
						if isNull {
							key.Validity().Clear(row)
						}
						if index%5 == 2 {
							continue
						}
						selected.Set(row)
						offsets = append(offsets, uint32(row))
						v := scalarAt(&key, row)
						position, found := -1, false
						if isNull {
							position, found = nullIndex, nullIndex >= 0
						} else {
							bits := v.bits
							if keyType.ID() == dtype.FLOAT32T {
								if math.IsNaN(float64(v.f32())) {
									bits = 0x7fc00000
								} else if v.f32() == 0 {
									bits = 0
								}
							} else if keyType.ID() == dtype.FLOAT64T {
								if math.IsNaN(v.f64()) {
									bits = 0x7ff8000000000000
								} else if v.f64() == 0 {
									bits = 0
								}
							}
							position, found = positions[bits]
							if !found {
								positions[bits] = len(want)
							}
						}
						if !found {
							position = len(want)
							want = append(want, expectedGroup{key: v})
							if isNull {
								nullIndex = position
							}
						}
						want[position].count++
						if !isNull {
							want[position].nonNull++
						}
					}
					batch := store.MakeBatch([]store.Vector{key})
					if start%2 == 0 {
						batch.SetSelection(store.MakeRowSelectionFromBitMap(selected))
					} else {
						selected.Release()
						batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, n, offsets)))
					}
					batches = append(batches, batch)
				}
				table := store.MakeTable(batches)
				schema := MakeSchema([]string{"key"}, []dtype.Type{keyType})
				plan, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeCount(MakeColumn("key"))))
				if err != nil {
					t.Fatal(err)
				}
				step := plan.steps[0]
				compact := makeCompactGroupState(step)
				if compact == nil {
					t.Fatal("COUNT plan was not supported")
				}
				baseline := &groupState{}
				for _, batch := range batches {
					if !compact.add(a, batch, step, math.MaxInt64) || !baseline.add(a, batch, step, math.MaxInt64) {
						t.Fatal("grouping failed")
					}
				}
				check := func(name string, state *store.Batch) {
					t.Helper()
					if state == nil || state.Len() != len(want) {
						t.Fatalf("%s: got %v rows, want %d", name, state, len(want))
					}
					for i, expected := range want {
						key := scalarAt(state.VectorAt(0), i)
						if key.IsNull() != expected.key.IsNull() || (!key.IsNull() && key.bits != expected.key.bits) || state.VectorAt(1).I64s()[i] != expected.count || state.VectorAt(2).I64s()[i] != expected.nonNull {
							t.Fatalf("%s group %d: key=%v counts=%d/%d, want key=%v counts=%d/%d", name, i, key, state.VectorAt(1).I64s()[i], state.VectorAt(2).I64s()[i], expected.key, expected.count, expected.nonNull)
						}
					}
				}
				for name, state := range map[string]*store.Batch{"compact": compact.finish(a, step), "baseline": baseline.finish(a, step)} {
					check(name, state)
					state.Release()
				}
				called := false
				result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20}, func(output *store.Batch) bool {
					called = true
					check("pipeline", output)
					return true
				})
				if result.Code() != ExecutionCompleted || called != (len(want) != 0) {
					t.Fatalf("pipeline: result %v, output emitted %t", result.Code(), called)
				}
				compact.release()
				baseline.release()
				plan.Release()
				table.Release()
				for _, batch := range batches {
					batch.Release()
				}
			})
		}
	}
}

func TestCompactGroupCountBudgetAndUnsupported(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	key := store.MakeVector(a, 1, dtype.Int32T(), false)
	batch := store.MakeBatch([]store.Vector{key})
	table := store.MakeTable([]*store.Batch{batch})
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int32T()})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	state := makeCompactGroupState(plan.steps[0])
	if state.add(a, batch, plan.steps[0], 1) {
		t.Fatal("accepted group state above memory budget")
	}
	state.release()
	scope := mem.MakeAllocationScope(a, 255)
	state = makeCompactGroupState(plan.steps[0])
	state.scope, state.index.scope = scope, scope
	if state.add(a, batch, plan.steps[0], math.MaxInt64) || scope.Live() != 0 {
		t.Fatal("scoped allocator accepted oversized group state")
	}
	state.release()
	scope = mem.MakeAllocationScope(a, 512)
	state = makeCompactGroupState(plan.steps[0])
	state.scope, state.index.scope = scope, scope
	if !state.add(a, batch, plan.steps[0], math.MaxInt64) || scope.Live() != 512 {
		t.Fatalf("scoped group allocation: live=%d", scope.Live())
	}
	state.release()
	if scope.Live() != 0 {
		t.Fatalf("scoped group release left %d bytes", scope.Live())
	}
	manyKeys := store.MakeVector(a, 17, dtype.Int32T(), false)
	for i := range manyKeys.I32s() {
		manyKeys.I32s()[i] = int32(i)
	}
	many := store.MakeBatch([]store.Vector{manyKeys})
	state = makeCompactGroupState(plan.steps[0])
	if state.add(a, many, plan.steps[0], 1200) {
		t.Fatal("accepted growth above peak memory budget")
	}
	state.release()
	many.Release()
	if makeCompactGroupState(physicalStep{groupKeys: plan.steps[0].groupKeys, schema: plan.steps[0].schema, aggregates: []physicalAggregateExpr{{kind: AggregateAvg}}}) != nil {
		t.Fatal("accepted unsupported aggregate")
	}
	plan.Release()
	table.Release()
	batch.Release()
}

func TestCompactGroupCountConcurrentPlanReuse(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	key := store.MakeVector(a, 24, dtype.Int32T(), false)
	for i := range key.I32s() {
		key.I32s()[i] = int32(i % 3)
	}
	batch := store.MakeBatch([]store.Vector{key})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int32T()})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	var wait sync.WaitGroup
	failures := make(chan string, 16)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(output *store.Batch) bool {
				retained := output.Retain()
				defer retained.Release()
				if retained.Len() != 3 {
					failures <- "unexpected group count"
					return true
				}
				for i := range retained.Len() {
					if retained.VectorAt(0).I32s()[i] != int32(i) || retained.VectorAt(1).I64s()[i] != 8 {
						failures <- "incorrect concurrent COUNT"
						break
					}
				}
				return true
			})
			if result.Code() != ExecutionCompleted {
				failures <- "concurrent execution failed"
			}
		}()
	}
	wait.Wait()
	close(failures)
	plan.Release()
	for failure := range failures {
		t.Error(failure)
	}
}

func TestCompactGroupCountScopedHighCardinality(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	batches := make([]*store.Batch, 4)
	for batchIndex := range batches {
		v := store.MakeVector(a, 1024, dtype.Int32T(), false)
		for row := range v.I32s() {
			v.I32s()[row] = int32(batchIndex*1024 + row)
		}
		batches[batchIndex] = store.MakeBatch([]store.Vector{v})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key"}, []dtype.Type{dtype.Int32T()}, []bool{false})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 15}, func(*store.Batch) bool { t.Fatal("under-budget query reached sink"); return true })
	if result.Code() != ExecutionResourceExhausted {
		t.Fatalf("small budget result: %v", result.Code())
	}
	var output *store.Batch
	result = plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20}, func(batch *store.Batch) bool { output = batch.Retain(); return true })
	plan.Release()
	if result.Code() != ExecutionCompleted || output == nil || output.Len() != 4096 {
		t.Fatalf("scoped high-cardinality result: %v", result.Code())
	}
	defer output.Release()
	for i := range output.Len() {
		if output.VectorAt(0).I32s()[i] != int32(i) || output.VectorAt(1).I64s()[i] != 1 {
			t.Fatalf("group %d was lost during scoped growth", i)
		}
	}
}
