package plan

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestGlobalDistinctMergeReusesSetsUnderParentBudget(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(fmt.Sprint(reuse), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			const rows = 2049
			x := store.MakeVector(a, rows, dtype.Int64T(), false)
			texts := make([][]byte, rows)
			for row := range rows {
				x.I64s()[row] = int64(row)
				texts[row] = []byte(fmt.Sprintf("distinct owned long string %08d", row))
			}
			batch := store.MakeBatch([]store.Vector{x, store.MakeStringVector(a, texts, nil)})
			table := store.MakeTable([]*store.Batch{batch})
			p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x", "s"}, []dtype.Type{dtype.Int64T(), dtype.StringT()})).GroupBy([]Expr{MakeLiteral(MakeI64Scalar(1))}, MakeCountDistinct(MakeColumn("x")), MakeSum(MakeColumn("x")).Distinct(), MakeCountDistinct(MakeColumn("s")), MakeMin(MakeColumn("s")).Distinct(), MakeMax(MakeColumn("s")), MakeCountStar()))
			if err != nil {
				t.Fatal(err)
			}
			step := p.steps[len(p.steps)-1]
			scope := mem.MakeAllocationScope(a, 8<<20)
			scoped := mem.MakeAllocatorWithScope(a, scope)
			sources := make([]*groupState, 4)
			defer func() {
				for _, source := range sources {
					if source != nil {
						source.release()
					}
				}
			}()
			for i := range sources {
				sources[i] = makeGroupState(step, scope)
				if !sources[i].add(scoped, batch, step, scope.Limit()) {
					t.Fatal("local state allocation failed")
				}
			}
			resident, ok := scope.AllocSeg(int(scope.Limit() - scope.Live() - 1024))
			if !ok {
				t.Fatal("parent reservation failed")
			}
			peakBefore := scope.Peak()
			if !reuse {
				copied := makeGroupState(step, scope)
				if copied.merge(scoped, sources[0], step, scope.Limit()) || !scope.Exhausted() {
					t.Fatal("copying a fifth set unexpectedly fit the constrained parent budget")
				}
				copied.release()
				for i, source := range sources {
					source.release()
					sources[i] = nil
				}
			} else {
				var retained *store.Batch
				result, handled := p.executeGlobalDistinctMerge(context.Background(), scoped, scope, step, sources, scope.Limit(), func(output *store.Batch) bool { retained = output.Retain(); return true })
				if !handled || result.Code() != ExecutionCompleted || retained == nil {
					t.Fatal(handled, result.Code(), result.Err())
				}
				if scope.Exhausted() || scope.Peak() != peakBefore {
					t.Fatal("merge allocated an additional distinct set", peakBefore, scope.Peak(), scope.Exhausted())
				}
				if retained.VectorAt(1).I64s()[0] != rows || retained.VectorAt(2).I64s()[0] != rows*(rows-1)/2 || retained.VectorAt(3).I64s()[0] != rows || retained.VectorAt(6).I64s()[0] != rows*4 {
					t.Fatal("mixed aggregate values changed")
				}
				for _, source := range sources {
					if source != nil {
						t.Fatal("source ownership was not consumed")
					}
				}
				if scope.Live() <= int64(resident.Len()) {
					t.Fatal("retained result escaped parent accounting")
				}
				p.Release()
				p = nil
				table.Release()
				table = nil
				batch.Release()
				batch = nil
				if retained.VectorAt(4).StringAt(0).View() != "distinct owned long string 00000000" || retained.VectorAt(5).StringAt(0).View() != "distinct owned long string 00002048" {
					t.Fatal("retained string aggregate lost ownership")
				}
				retained.Release()
			}
			if scope.Live() != int64(resident.Len()) {
				t.Fatal("merge payload leaked", scope.Live(), resident.Len())
			}
			resident.Dec()
			if scope.Live() != 0 {
				t.Fatal("parent scope leaked", scope.Live())
			}
			if p != nil {
				p.Release()
			}
			if table != nil {
				table.Release()
			}
			if batch != nil {
				batch.Release()
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatal("allocator leaked", usage)
			}
		})
	}
}

func TestGlobalDistinctMergeTransferredSeedsPreserveReductions(t *testing.T) {
	for _, input := range []string{"mixed", "all_null", "empty"} {
		t.Run(input, func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			batches := make([]*store.Batch, 4)
			payloads := [][]float64{{1e20, -1e20, 3}, {4, 1e20, 5, -1e20, 6, math.NaN(), math.Copysign(0, -1), 0}, {-1e20, 1e20, 7, math.NaN(), 0}, {8, -1e20, 9, 1e20}}
			for i, payload := range payloads {
				n := len(payload)
				if input == "empty" {
					n = 0
				}
				f := store.MakeVector(a, n, dtype.Float64T(), true)
				x := store.MakeVector(a, n, dtype.Int64T(), true)
				for row := range n {
					f.F64s()[row] = payload[row]
					x.I64s()[row] = int64(row)
					if input == "all_null" {
						f.Validity().Clear(row)
						x.Validity().Clear(row)
					}
				}
				batches[i] = store.MakeBatch([]store.Vector{f, x})
			}
			table := store.MakeTable(batches)
			p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"f", "x"}, []dtype.Type{dtype.Float64T(), dtype.Int64T()}, []bool{true, true})).GroupBy([]Expr{MakeLiteral(MakeI64Scalar(1))}, MakeCountDistinct(MakeColumn("f")), MakeSum(MakeColumn("f")).Distinct(), MakeAvg(MakeColumn("f")).Distinct(), MakeMin(MakeColumn("f")).Distinct(), MakeMax(MakeColumn("f")).Distinct(), MakeCountDistinct(MakeColumn("x")), MakeSum(MakeColumn("x")).Distinct(), MakeAvg(MakeColumn("x")).Distinct(), MakeCountDistinct(MakeColumn("x").Add(MakeLiteral(MakeI64Scalar(1)))), MakeCountStar(), MakeSum(MakeColumn("f"))))
			if err != nil {
				t.Fatal(err)
			}
			step := p.steps[len(p.steps)-1]
			reference := makeGroupState(step, nil)
			for _, batch := range batches {
				source := makeGroupState(step, nil)
				if !source.add(a, batch, step, 64<<20) || !reference.merge(a, source, step, 64<<20) {
					t.Fatal("reference merge failed")
				}
				source.release()
			}
			want := reference.finish(a, step)
			reference.release()
			for _, terminal := range []string{"complete", "stop", "cancel", "sink_cancel"} {
				scope := mem.MakeAllocationScope(a, 64<<20)
				scoped := mem.MakeAllocatorWithScope(a, scope)
				sources := make([]*groupState, len(batches))
				for i, batch := range batches {
					sources[i] = makeGroupState(step, scope)
					if !sources[i].add(scoped, batch, step, 64<<20) {
						t.Fatal("local state allocation failed")
					}
				}
				ctx, cancel := context.WithCancel(context.Background())
				if terminal == "cancel" {
					cancel()
				}
				calls := 0
				result, handled := p.executeGlobalDistinctMerge(ctx, scoped, scope, step, sources, 64<<20, func(got *store.Batch) bool {
					calls++
					if terminal == "cancel" {
						t.Fatal("cancelled merge reached sink")
					}
					if got.Len() != want.Len() || got.NVectors() != want.NVectors() {
						t.Fatal("output shape changed")
					}
					for row := range got.Len() {
						for col := range got.NVectors() {
							actual, expected := scalarAt(got.VectorAt(col), row), scalarAt(want.VectorAt(col), row)
							if actual.dType != expected.dType || actual.null != expected.null || actual.bits != expected.bits {
								t.Fatalf("terminal=%s row=%d col=%d actual=%+v expected=%+v", terminal, row, col, actual, expected)
							}
						}
					}
					if terminal == "sink_cancel" {
						cancel()
					}
					return terminal != "stop"
				})
				cancel()
				if input == "empty" {
					if handled || calls != 0 {
						t.Fatal("empty merge should defer to global empty finalization")
					}
				} else {
					expected := ExecutionCompleted
					if terminal == "stop" {
						expected = ExecutionStopped
					}
					if terminal == "cancel" || terminal == "sink_cancel" {
						expected = ExecutionCancelled
					}
					if !handled || result.Code() != expected {
						t.Fatal(terminal, handled, result.Code(), result.Err())
					}
				}
				for _, source := range sources {
					if source != nil {
						source.release()
					}
				}
				if scope.Live() != 0 {
					t.Fatal("terminal merge leaked", terminal, scope.Live())
				}
			}
			want.Release()
			p.Release()
			table.Release()
			for _, batch := range batches {
				batch.Release()
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatal("allocator leaked", usage)
			}
		})
	}
}

func TestGroupedDistinctOwnedOverlapReleasesSourceRow(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, 8<<20)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	keys := store.MakeVector(a, 4, dtype.Int64T(), false)
	for row := range 4 {
		keys.I64s()[row] = int64(row / 2)
	}
	texts := [][]byte{[]byte("alpha owned aggregate"), []byte("beta owned aggregate"), []byte("charlie owned aggregate"), []byte("delta owned aggregate")}
	batch := store.MakeBatch([]store.Vector{keys, store.MakeStringVector(a, texts, nil)})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"g", "s"}, []dtype.Type{dtype.Int64T(), dtype.StringT()})).GroupBy([]Expr{MakeColumn("g")}, MakeCountDistinct(MakeColumn("s")), MakeMin(MakeColumn("s")).Distinct(), MakeMax(MakeColumn("s")), MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	step := p.steps[len(p.steps)-1]
	target, source := makeGroupState(step, scope), makeGroupState(step, scope)
	if !target.add(scoped, batch, step, scope.Limit()) || !source.add(scoped, batch, step, scope.Limit()) {
		t.Fatal("local state failed")
	}
	before := scope.Live()
	if !target.mergeOwnedRow(scoped, source, step, scope.Limit(), 0, make([]Scalar, 1)) {
		t.Fatal("owned overlap failed")
	}
	if source.values[0] != nil || source.uniques[0] != nil || source.values[1] == nil || source.uniques[1] == nil || scope.Live() >= before {
		t.Fatal("overlap did not release only the consumed source row", before, scope.Live())
	}
	if !target.mergeOwnedRow(scoped, source, step, scope.Limit(), 1, make([]Scalar, 1)) {
		t.Fatal("second overlap failed")
	}
	source.release()
	output := target.finish(scoped, step)
	target.release()
	if output == nil || output.Len() != 2 {
		t.Fatal("output missing")
	}
	for row := range 2 {
		if output.VectorAt(1).I64s()[row] != 2 || output.VectorAt(4).I64s()[row] != 4 {
			t.Fatal("shared dedup or ordinary reduction changed")
		}
	}
	output.Release()
	p.Release()
	table.Release()
	batch.Release()
	if scope.Live() != 0 {
		t.Fatal("owned overlap scope leaked", scope.Live())
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatal("allocator leaked", usage)
	}
}

func TestGlobalDistinctMergeLargestSeedNaNAndZero(t *testing.T) {
	for _, dType := range []dtype.Type{dtype.Float32T(), dtype.Float64T()} {
		t.Run(dType.String(), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			batches := make([]*store.Batch, 3)
			for i, n := range []int{1, 3, 2} {
				vector := store.MakeVector(a, n, dType, false)
				if dType.ID() == dtype.FLOAT32T {
					vector.F32s()[0] = math.Float32frombits(1 << 31)
					if n > 1 {
						vector.F32s()[1] = math.Float32frombits(0x7fc00000 | uint32(i+1))
					}
					if n > 2 {
						vector.F32s()[2] = 0
					}
				} else {
					vector.F64s()[0] = math.Copysign(0, -1)
					if n > 1 {
						vector.F64s()[1] = math.Float64frombits(0x7ff8000000000000 | uint64(i+1))
					}
					if n > 2 {
						vector.F64s()[2] = 0
					}
				}
				batches[i] = store.MakeBatch([]store.Vector{vector})
			}
			table := store.MakeTable(batches)
			p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"f"}, []dtype.Type{dType})).GroupBy([]Expr{MakeLiteral(MakeI64Scalar(1))}, MakeCountDistinct(MakeColumn("f")), MakeMin(MakeColumn("f")).Distinct(), MakeMax(MakeColumn("f")).Distinct(), MakeCountDistinct(MakeColumn("f"))))
			if err != nil {
				t.Fatal(err)
			}
			step := p.steps[len(p.steps)-1]
			scope := mem.MakeAllocationScope(a, 8<<20)
			scoped := mem.MakeAllocatorWithScope(a, scope)
			sources := make([]*groupState, len(batches))
			for i, batch := range batches {
				sources[i] = makeGroupState(step, scope)
				if !sources[i].add(scoped, batch, step, scope.Limit()) {
					t.Fatal("local state allocation failed")
				}
			}
			if sources[1].uniques[0][0].length() <= sources[0].uniques[0][0].length() {
				t.Fatal("fixture requires a later largest seed")
			}
			result, handled := p.executeGlobalDistinctMerge(context.Background(), scoped, scope, step, sources, scope.Limit(), func(batch *store.Batch) bool {
				if batch.VectorAt(1).I64s()[0] != 2 || batch.VectorAt(4).I64s()[0] != 2 {
					t.Fatal("NaN or signed-zero DISTINCT equality changed")
				}
				minValue, maxValue := scalarAt(batch.VectorAt(2), 0), scalarAt(batch.VectorAt(3), 0)
				negativeZero := uint64(1) << 63
				if dType.ID() == dtype.FLOAT32T {
					negativeZero = 1 << 31
				}
				if minValue.bits != negativeZero || maxValue.bits != 0 || minValue.IsNull() || maxValue.IsNull() {
					t.Fatal("largest seed lost signed-zero MIN/MAX", minValue, maxValue)
				}
				return true
			})
			if !handled || result.Code() != ExecutionCompleted {
				t.Fatal(handled, result.Code(), result.Err())
			}
			if scope.Live() != 0 {
				t.Fatal("merge leaked", scope.Live())
			}
			p.Release()
			table.Release()
			for _, batch := range batches {
				batch.Release()
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatal("allocator leaked", usage)
			}
		})
	}
}
