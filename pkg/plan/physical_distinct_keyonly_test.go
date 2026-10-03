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

func TestKeyOnlyDistinctStateDifferential(t *testing.T) {
	types := []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.DateT(), dtype.TimestampTZT(), dtype.BoolT(), dtype.StringT()}
	for _, dType := range types {
		t.Run(dType.String(), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			const rows = 2053
			rng := rand.New(rand.NewPCG(38, 57))
			vector := store.MakeVector(a, rows, dType, true)
			if dType.ID() == dtype.STRT {
				vector.Release()
				texts, valid := make([][]byte, rows), make([]bool, rows)
				for row := range rows {
					texts[row] = []byte(fmt.Sprintf("binary\x00 owned long text %03d", rng.IntN(177)))
					valid[row] = row%17 != 0
				}
				vector = store.MakeStringVector(a, texts, valid)
			} else {
				for row := range rows {
					value := rng.IntN(173) - 86
					switch dType.ID() {
					case dtype.INT32T, dtype.DATET:
						vector.I32s()[row] = int32(value)
					case dtype.INT64T, dtype.TIMESTAMPTZT:
						vector.I64s()[row] = int64(value)
					case dtype.BOOLT:
						if value > 0 {
							vector.Bools()[row>>3] |= 1 << (row & 7)
						}
					case dtype.FLOAT32T:
						vector.F32s()[row] = float32(value)
						if row%11 == 0 {
							vector.F32s()[row] = math.Float32frombits(0x7fc00000 | uint32(row))
						}
						if row%23 == 0 {
							vector.F32s()[row] = math.Float32frombits(1 << 31)
						}
						if row%31 == 0 {
							vector.F32s()[row] = float32(math.Inf(-1))
						}
					case dtype.FLOAT64T:
						vector.F64s()[row] = float64(value)
						if row%11 == 0 {
							vector.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(row))
						}
						if row%23 == 0 {
							vector.F64s()[row] = math.Copysign(0, -1)
						}
						if row%31 == 0 {
							vector.F64s()[row] = math.Inf(1)
						}
					}
					if row%17 == 0 {
						vector.Validity().Clear(row)
					}
				}
			}
			batch := store.MakeBatch([]store.Vector{vector})
			scope := mem.MakeAllocationScope(a, 8<<20)
			payload, keys := &distinctState{}, &distinctState{keyOnly: true}
			payload.setScope(scope)
			keys.setScope(scope)
			payload.index.stringKeys = dType.ID() == dtype.STRT
			keys.index.stringKeys = payload.index.stringKeys
			for row := range rows {
				var reference, actual int
				var referenceOK, actualOK bool
				if payload.index.stringKeys {
					reference, referenceOK = payload.addString(a, batch, row, scope.Limit())
					actual, actualOK = keys.addString(a, batch, row, scope.Limit())
				} else {
					reference, referenceOK = payload.add(a, batch, row, scope.Limit())
					actual, actualOK = keys.add(a, batch, row, scope.Limit())
				}
				if !referenceOK || !actualOK || reference != actual || payload.length() != keys.length() {
					t.Fatal("key-only equality differs", row, reference, actual, referenceOK, actualOK)
				}
			}
			if keys.rows.values != nil || keys.rows.strings != nil || keys.rows.types != nil || keys.rows.length != 0 || keys.rows.capacity != 0 {
				t.Fatal("key-only state allocated representative rows")
			}
			if keys.length() != keys.index.used || keys.charged >= payload.charged {
				t.Fatal("key-only accounting or index count changed", keys.length(), keys.index.used, keys.charged, payload.charged)
			}
			for row := range keys.length() {
				if string(keys.keys.key(row)) != string(payload.keys.key(row)) {
					t.Fatal("encoded key changed", row)
				}
			}
			before := keys.length()
			if _, ok := keys.addEncoded(a, payload.keys.key(0), nil, 0, nil, scope.Limit()); !ok || keys.length() != before {
				t.Fatal("duplicate encoded key changed the state")
			}
			keys.release()
			payload.release()
			batch.Release()
			if scope.Live() != 0 {
				t.Fatal("scoped state leaked", scope.Live())
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatal("allocator leaked", usage)
			}
		})
	}
}

func TestKeyOnlyGroupedDistinctPayloadDifferential(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		for _, input := range []string{"mixed", "all_null", "empty"} {
			t.Run(fmt.Sprintf("groups=%t/%s", grouped, input), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				rng := rand.New(rand.NewPCG(74, 123))
				batches := make([]*store.Batch, 4)
				for chunk := range batches {
					n := 2053
					if input == "empty" {
						n = 0
					}
					g, x, f := store.MakeVector(a, n, dtype.Int64T(), true), store.MakeVector(a, n, dtype.Int64T(), true), store.MakeVector(a, n, dtype.Float64T(), true)
					texts, valid := make([][]byte, n), make([]bool, n)
					offsets := make([]uint32, 0, n)
					bitmap := store.MakeBitMap(a, n)
					for row := range n {
						g.I64s()[row] = int64(rng.IntN(7))
						x.I64s()[row] = int64(rng.IntN(201) - 100)
						f.F64s()[row] = float64(x.I64s()[row])
						if row%19 == 0 {
							f.F64s()[row] = math.Float64frombits(0x7ff8000000000000 | uint64(row+chunk))
						}
						if row%23 == 0 {
							f.F64s()[row] = math.Copysign(0, -1)
						}
						if row%31 == 0 {
							f.F64s()[row] = math.Inf(1)
						}
						texts[row] = []byte(fmt.Sprintf("binary\x00 distinct long string %03d", rng.IntN(257)))
						valid[row] = input != "all_null" && row%17 != 0
						if !valid[row] {
							x.Validity().Clear(row)
							f.Validity().Clear(row)
						}
						if row%43 == 0 {
							g.Validity().Clear(row)
						}
						if row%11 != 0 {
							offsets = append(offsets, uint32(row))
							bitmap.Set(row)
						}
					}
					batches[chunk] = store.MakeBatch([]store.Vector{g, x, f, store.MakeStringVector(a, texts, valid)})
					if chunk%2 == 0 {
						bitmap.Release()
						batches[chunk].SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, n, offsets)))
					} else {
						batches[chunk].SetSelection(store.MakeRowSelectionFromBitMap(bitmap))
					}
				}
				table := store.MakeTable(batches)
				schema := MakeSchemaWithNullability([]string{"g", "x", "f", "s"}, []dtype.Type{dtype.Int64T(), dtype.Int64T(), dtype.Float64T(), dtype.StringT()}, []bool{true, true, true, true})
				keys := []Expr{MakeLiteral(MakeI64Scalar(1))}
				if grouped {
					keys = []Expr{MakeColumn("g")}
				}
				aggregates := []Aggregate{MakeCountDistinct(MakeColumn("f")), MakeMin(MakeColumn("f")).Distinct(), MakeMax(MakeColumn("f")).Distinct(), MakeCountDistinct(MakeColumn("f")), MakeCountDistinct(MakeColumn("s")), MakeMin(MakeColumn("s")).Distinct(), MakeMax(MakeColumn("s")), MakeCountDistinct(MakeColumn("x")), MakeSum(MakeColumn("x")).Distinct(), MakeAvg(MakeColumn("x")).Distinct(), MakeCountDistinct(MakeColumn("x").Add(MakeLiteral(MakeI64Scalar(1)))), MakeSum(MakeColumn("x")), MakeCountStar()}
				p, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy(keys, aggregates...))
				if err != nil {
					t.Fatal(err)
				}
				step := p.steps[len(p.steps)-1]
				metadata := makeGroupState(step, nil)
				if !metadata.uniqueKeyOnly[0] || !metadata.uniqueKeyOnly[3] || !metadata.uniqueKeyOnly[4] || metadata.uniqueKeyOnly[7] || !metadata.uniqueKeyOnly[10] {
					t.Fatal("shared representative requirements are incorrect", metadata.uniqueSources, metadata.uniqueKeyOnly)
				}
				metadata.release()
				reference := makeGroupState(step, nil)
				clear(reference.uniqueKeyOnly)
				for _, batch := range batches {
					input := batch
					if p.source.projection != nil {
						input = batch.Project(p.source.projection)
					}
					source := makeGroupState(step, nil)
					clear(source.uniqueKeyOnly)
					if !source.add(a, input, step, 64<<20) || !reference.merge(a, source, step, 64<<20) {
						t.Fatal("payload reference failed")
					}
					source.release()
					if input != batch {
						input.Release()
					}
				}
				expected := reference.finish(a, step)
				reference.release()
				want := map[string][]Scalar{}
				capture := func(batch *store.Batch, target map[string][]Scalar) {
					for row := range batch.Len() {
						key := scalarAt(batch.VectorAt(0), row)
						id := fmt.Sprint(key.bits, key.null)
						if _, duplicate := target[id]; duplicate {
							t.Fatal("duplicate output group", id)
						}
						values := make([]Scalar, batch.NVectors())
						for col := range values {
							values[col] = scalarAt(batch.VectorAt(col), row)
							if values[col].Type().ID() == dtype.STRT {
								values[col].text = string([]byte(values[col].text))
							}
						}
						target[id] = values
					}
				}
				capture(expected, want)
				expected.Release()
				var retained []*store.Batch
				for _, workers := range []int{1, 4} {
					scope := mem.MakeAllocationScope(a, 64<<20)
					actual := map[string][]Scalar{}
					result := p.executeWithScope(context.Background(), a, ExecutionOptions{Workers: workers, MemoryBudget: scope.Limit()}, func(batch *store.Batch) bool {
						capture(batch, actual)
						retained = append(retained, batch.Retain())
						return true
					}, scope)
					if result.Code() != ExecutionCompleted || len(actual) != len(want) {
						t.Fatal(workers, result.Code(), result.Err(), len(actual), len(want))
					}
					for key, values := range want {
						got, exists := actual[key]
						if !exists {
							t.Fatal("missing group", workers, key)
						}
						for col, expected := range values {
							value := got[col]
							if value.dType != expected.dType || value.null != expected.null || value.bits != expected.bits || value.text != expected.text {
								t.Fatalf("workers=%d group=%s col=%d got=%+v want=%+v", workers, key, col, value, expected)
							}
						}
					}
				}
				p.Release()
				table.Release()
				for _, batch := range batches {
					batch.Release()
				}
				for _, batch := range retained {
					for row := range batch.Len() {
						key := scalarAt(batch.VectorAt(0), row)
						values := want[fmt.Sprint(key.bits, key.null)]
						for col, expected := range values {
							value := scalarAt(batch.VectorAt(col), row)
							if value.bits != expected.bits || value.null != expected.null || value.text != expected.text {
								t.Fatal("retained result changed after input release", row, col)
							}
						}
					}
					batch.Release()
				}
				if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatal("allocator leaked", usage)
				}
			})
		}
	}
}

func TestKeyOnlyDistinctParentBudgetSavingsAndCompatibility(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	const rows = 4097
	texts := make([][]byte, rows)
	for row := range rows {
		texts[row] = []byte(fmt.Sprintf("distinct long owned value with enough payload to dominate representative duplication %08d", row))
	}
	batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, nil)})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"s"}, []dtype.Type{dtype.StringT()})).GroupBy([]Expr{MakeLiteral(MakeI64Scalar(1))}, MakeCountDistinct(MakeColumn("s")), MakeMin(MakeColumn("s")).Distinct(), MakeMax(MakeColumn("s")).Distinct(), MakeCountDistinct(MakeColumn("s"))))
	if err != nil {
		t.Fatal(err)
	}
	step := p.steps[len(p.steps)-1]
	var keyPeak, keyLive, fullPeak, fullLive int64
	for _, keysOnly := range []bool{false, true} {
		scope := mem.MakeAllocationScope(a, 32<<20)
		scoped := mem.MakeAllocatorWithScope(a, scope)
		state := makeGroupState(step, scope)
		if !keysOnly {
			clear(state.uniqueKeyOnly)
		}
		if !state.add(scoped, batch, step, scope.Limit()) {
			t.Fatal("large-budget state failed")
		}
		unique := state.uniques[0][0]
		if keysOnly {
			keyPeak, keyLive = scope.Peak(), scope.Live()
			if unique.rows.bytes() != 0 || unique.length() != rows {
				t.Fatal("key-only cardinality or representative payload changed")
			}
		} else {
			fullPeak, fullLive = scope.Peak(), scope.Live()
			if unique.rows.length != rows {
				t.Fatal("payload reference was not populated")
			}
		}
		state.release()
		if scope.Live() != 0 {
			t.Fatal("state reservation leaked", scope.Live())
		}
	}
	if keyPeak >= fullPeak || keyLive >= fullLive || fullLive-keyLive < int64(rows*24) {
		t.Fatal("key-only did not remove representative allocation", keyPeak, fullPeak, keyLive, fullLive)
	}
	t.Logf("key-only peak/live=%d/%d, payload peak/live=%d/%d", keyPeak, keyLive, fullPeak, fullLive)
	for _, keysOnly := range []bool{false, true} {
		scope := mem.MakeAllocationScope(a, keyPeak+1024)
		scoped := mem.MakeAllocatorWithScope(a, scope)
		resident, ok := scope.AllocSeg(127)
		if !ok {
			t.Fatal("parent reservation failed")
		}
		state := makeGroupState(step, scope)
		if !keysOnly {
			clear(state.uniqueKeyOnly)
		}
		if ok := state.add(scoped, batch, step, scope.Limit()); ok != keysOnly {
			t.Fatal("low parent budget outcome", keysOnly, ok, scope.Peak())
		}
		state.release()
		if scope.Live() != 127 {
			t.Fatal("failed/successful state leaked", scope.Live())
		}
		resident.Dec()
	}
	scope := mem.MakeAllocationScope(a, 32<<20)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	fullSource, keySource := makeGroupState(step, scope), makeGroupState(step, scope)
	clear(fullSource.uniqueKeyOnly)
	if !fullSource.add(scoped, batch, step, scope.Limit()) || !keySource.add(scoped, batch, step, scope.Limit()) {
		t.Fatal("compatibility source failed")
	}
	fullTarget, keyTarget := makeGroupState(step, scope), makeGroupState(step, scope)
	clear(fullTarget.uniqueKeyOnly)
	if fullTarget.merge(scoped, keySource, step, scope.Limit()) {
		t.Fatal("payload target accepted normalized keys without original representatives")
	}
	if !keyTarget.merge(scoped, fullSource, step, scope.Limit()) {
		t.Fatal("key-only target rejected payload source")
	}
	if !keyTarget.merge(scoped, keySource, step, scope.Limit()) {
		t.Fatal("key-only overlap merge failed")
	}
	output := keyTarget.finish(scoped, step)
	if output == nil || output.VectorAt(1).I64s()[0] != rows || output.VectorAt(4).I64s()[0] != rows {
		t.Fatal("compatibility count changed")
	}
	output.Release()
	fullSource.release()
	keySource.release()
	fullTarget.release()
	keyTarget.release()
	for _, sourceKeyOnly := range []bool{false, true} {
		source, target := makeGroupState(step, scope), makeGroupState(step, scope)
		if !sourceKeyOnly {
			clear(source.uniqueKeyOnly)
		} else {
			clear(target.uniqueKeyOnly)
		}
		if !source.add(scoped, batch, step, scope.Limit()) {
			t.Fatal("owned compatibility source failed")
		}
		merged := target.mergeOwnedRow(scoped, source, step, scope.Limit(), 0, make([]Scalar, 1))
		if merged == sourceKeyOnly {
			t.Fatal("owned compatibility outcome", sourceKeyOnly, merged)
		}
		if merged {
			if source.values[0] != nil || source.uniques[0] != nil || !target.uniques[0][0].keyOnly || target.uniques[0][0].rows.bytes() != 0 {
				t.Fatal("owned full source was not converted and consumed")
			}
			output := target.finish(scoped, step)
			if output == nil || output.VectorAt(1).I64s()[0] != rows || output.VectorAt(4).I64s()[0] != rows {
				t.Fatal("owned compatibility count changed")
			}
			output.Release()
		} else if source.values[0] == nil || source.uniques[0] == nil {
			t.Fatal("rejected owned source was consumed")
		}
		source.release()
		target.release()
	}
	if scope.Live() != 0 {
		t.Fatal("compatibility reservation leaked", scope.Live())
	}
	p.Release()
	table.Release()
	batch.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatal("allocator leaked", usage)
	}
}

func TestKeyOnlyDistinctGlobalMergeCancellation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	const rows = 8193
	vector := store.MakeVector(a, rows, dtype.Int64T(), false)
	for row := range rows {
		vector.I64s()[row] = int64(row)
	}
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()})).GroupBy([]Expr{MakeLiteral(MakeI64Scalar(1))}, MakeCountDistinct(MakeColumn("x")), MakeMin(MakeColumn("x")).Distinct(), MakeCountDistinct(MakeColumn("x").Add(MakeLiteral(MakeI64Scalar(1))))))
	if err != nil {
		t.Fatal(err)
	}
	step := p.steps[len(p.steps)-1]
	scope := mem.MakeAllocationScope(a, 32<<20)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	sources := make([]*groupState, 4)
	for i := range sources {
		sources[i] = makeGroupState(step, scope)
		if !sources[i].add(scoped, batch, step, scope.Limit()) {
			t.Fatal("local state failed")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	controlled := &parallelSortCancelContext{Context: ctx, cancel: cancel}
	result, handled := p.executeGlobalDistinctMerge(controlled, scoped, scope, step, sources, scope.Limit(), func(*store.Batch) bool { t.Error("cancelled merge reached sink"); return true })
	cancel()
	if !handled || result.Code() != ExecutionCancelled || controlled.calls.Load() < 20 {
		t.Fatal("merge did not observe mid-merge cancellation", handled, result.Code(), controlled.calls.Load())
	}
	for _, source := range sources {
		if source != nil {
			source.release()
		}
	}
	if scope.Live() != 0 {
		t.Fatal("cancelled merge leaked", scope.Live())
	}
	p.Release()
	table.Release()
	batch.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatal("allocator leaked", usage)
	}
}
