package plan

import (
	"context"
	"math"
	"reflect"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestSharedGlobalAggregatesDifferential(t *testing.T) {
	for _, rows := range []int{0, 1, 9, 129} {
		for _, workers := range []int{1, 4} {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
			var batches []*store.Batch
			for part := range 4 {
				x := store.MakeVector(a, rows, dtype.Float64T(), true)
				flag := store.MakeVector(a, rows, dtype.BoolT(), false)
				text := make([][]byte, rows)
				valid := make([]bool, rows)
				for row := range rows {
					x.F64s()[row] = float64((row+part)%7 - 3)
					if row%13 == 0 {
						x.F64s()[row] = math.NaN()
					}
					if row%17 == 0 {
						x.F64s()[row] = math.Inf(1)
					}
					if row%19 == 0 {
						x.F64s()[row] = math.Copysign(0, -1)
					}
					if row%11 == 0 {
						x.Validity().Clear(row)
					}
					if row%3 == 0 {
						flag.Bools()[row>>3] |= 1 << (row & 7)
					}
					text[row] = []byte("owned aggregate string payload " + string(rune('a'+(row+part)%5)))
					valid[row] = row%11 != 0
				}
				batch := store.MakeBatch([]store.Vector{x, flag, store.MakeStringVector(a, text, valid)})
				var selected []uint32
				for row := range rows {
					if row%5 != 0 {
						selected = append(selected, uint32(row))
					}
				}
				batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, rows, selected)))
				batches = append(batches, batch)
			}
			table := store.MakeTable(batches)
			for _, batch := range batches {
				batch.Release()
			}
			schema := MakeSchema([]string{"x", "flag", "text"}, []dtype.Type{dtype.Float64T(), dtype.BoolT(), dtype.StringT()})
			input := MakeCase(MakeColumn("flag"), MakeColumn("x"), MakeLiteral(1.0).Div(MakeColumn("x")))
			aggs := []Aggregate{MakeSum(input), MakeSum(input), MakeAvg(input), MakeAvg(input), MakeCountStar(), MakeCountStar(), MakeCountDistinct(MakeColumn("x")), MakeCountDistinct(MakeColumn("x")), MakeCount(MakeColumn("x")), MakeMin(MakeColumn("text")), MakeMin(MakeColumn("text")), MakeMax(MakeColumn("text"))}
			// DISTINCT runs serially; the second variant verifies local-state merging.
			if workers == 4 {
				aggs = append(aggs[:6], aggs[8:]...)
			}
			p, err := MakePhysicalPlan(MakeScan(table, schema).Aggregate(aggs...))
			if err != nil {
				t.Fatal(err)
			}
			if len(p.steps[0].aggregateSources) == 0 {
				t.Fatal("identical aggregates not shared")
			}
			var outputs []*store.Batch
			for _, shared := range []bool{true, false} {
				sources := p.steps[0].aggregateSources
				if !shared {
					p.steps[0].aggregateSources = nil
				}
				result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{Workers: workers, MemoryBudget: 1 << 20}, func(batch *store.Batch) bool { outputs = append(outputs, batch.Retain()); return true })
				p.steps[0].aggregateSources = sources
				if result.Code() != ExecutionCompleted {
					t.Fatalf("rows %d workers %d shared %t: %v", rows, workers, shared, result.Code())
				}
			}
			p.Release()
			table.Release()
			for column := range len(aggs) {
				l, r := scalarAt(outputs[0].VectorAt(column), 0), scalarAt(outputs[1].VectorAt(column), 0)
				if l != r {
					t.Fatalf("rows %d workers %d column %d: shared %+v reference %+v", rows, workers, column, l, r)
				}
			}
			for _, out := range outputs {
				out.Release()
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("shared aggregates leaked: %+v", usage)
			}
		}
	}
}

func TestSharedAggregateIdentity(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	x := store.MakeVector(a, 1, dtype.Float64T(), false)
	y := store.MakeVector(a, 1, dtype.Float64T(), false)
	batch := store.MakeBatch([]store.Vector{x, y})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x", "y"}, []dtype.Type{dtype.Float64T(), dtype.Float64T()})).Aggregate(
		MakeSum(MakeColumn("x").Add(0.0)), MakeSum(MakeColumn("x").Add(math.Copysign(0, -1))), MakeSum(MakeColumn("y").Add(0.0)), MakeAvg(MakeColumn("x").Add(0.0)), MakeSum(MakeColumn("x").Add(0.0)).Distinct(), MakeSum(MakeColumn("x").Add(0.0)).Alias("other"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.steps[0].aggregateSources, []int{0, 1, 2, 3, 4, 0}) {
		t.Fatalf("incorrect identity: %v", p.steps[0].aggregateSources)
	}
	p.Release()
	table.Release()
}
