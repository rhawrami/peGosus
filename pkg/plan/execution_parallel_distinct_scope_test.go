package plan

import (
	"context"
	"runtime"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParallelDistinctPreservesParentScope(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 4)
	for chunk := range batches {
		v := store.MakeVector(a, 2049, dtype.Int64T(), true)
		for row := range v.Len() {
			v.I64s()[row] = int64((chunk*v.Len() + row) % 17)
			if row%29 == 0 {
				v.Validity().Clear(row)
			}
		}
		batches[chunk] = store.MakeBatch([]store.Vector{v})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()}))
	queries := []struct {
		name    string
		logical LogicalPlan
	}{
		{"standalone", scan.Distinct()},
		{"distinct_reduce", scan.Distinct().Aggregate(MakeCountStar(), MakeSum(MakeColumn("x")))},
		{"single_global", scan.Aggregate(MakeCountDistinct(MakeColumn("x")))},
		{"multiple_global", scan.Aggregate(MakeCountDistinct(MakeColumn("x")), MakeSum(MakeColumn("x")).Distinct(), MakeCountStar())},
		{"grouped", scan.GroupBy([]Expr{MakeColumn("x")}, MakeCountDistinct(MakeColumn("x")))},
		{"min", scan.Aggregate(MakeMin(MakeColumn("x")).Distinct())},
		{"max", scan.Aggregate(MakeMax(MakeColumn("x")).Distinct())},
	}
	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			p, err := MakePhysicalPlan(query.logical)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Release()
			baseline := a.Usage()
			options := ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}
			for _, dispatch := range []bool{false, true} {
				scope := mem.MakeAllocationScope(a, options.MemoryBudget)
				resident, ok := scope.AllocSeg(127)
				if !ok {
					t.Fatal("parent reservation failed")
				}
				var retained []*store.Batch
				var result ExecutionResult
				parallel := true
				sink := func(batch *store.Batch) bool { retained = append(retained, batch.Retain()); return true }
				if dispatch {
					result = p.executeWithScope(context.Background(), a, options, sink, scope)
				} else {
					result, parallel = p.executeParallelDistinctWithScope(context.Background(), a, options, sink, scope)
				}
				if !parallel || result.Code() != ExecutionCompleted || len(retained) == 0 {
					t.Fatal(dispatch, parallel, result.Code(), result.Err())
				}
				if scope.Peak() <= 127 || scope.Live() <= 127 {
					t.Fatal("query allocation escaped parent scope", dispatch, scope.Peak(), scope.Live())
				}
				for _, batch := range retained {
					batch.Release()
				}
				if scope.Live() != 127 {
					t.Fatal("query retained payload leaked", dispatch, scope.Live())
				}
				resident.Dec()
				if scope.Live() != 0 {
					t.Fatal("parent reservation leaked", dispatch, scope.Live())
				}
				constrained := mem.MakeAllocationScope(a, 2<<20)
				resident, ok = constrained.AllocSeg(int(constrained.Limit()))
				if !ok {
					t.Fatal("parent budget reservation failed")
				}
				if dispatch {
					result = p.executeWithScope(context.Background(), a, options, func(*store.Batch) bool { t.Error("exhausted parent scope reached sink"); return true }, constrained)
				} else {
					result, parallel = p.executeParallelDistinctWithScope(context.Background(), a, options, func(*store.Batch) bool { t.Error("exhausted parent scope reached sink"); return true }, constrained)
				}
				if !parallel || result.Code() != ExecutionResourceExhausted || !constrained.Exhausted() {
					t.Fatal("parent limit not enforced", dispatch, parallel, result.Code(), result.Err())
				}
				if constrained.Live() != constrained.Limit() {
					t.Fatal("failed query escaped/modified resident reservation", dispatch, constrained.Live())
				}
				resident.Dec()
				if constrained.Live() != 0 {
					t.Fatal("failed query leaked", dispatch, constrained.Live())
				}
			}
			if usage := a.Usage(); usage.GeneralUsed != baseline.GeneralUsed || usage.ScratchUsed != baseline.ScratchUsed {
				t.Fatal("query payload leaked", usage, baseline)
			}
		})
	}
	table.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatal("source payload leaked", usage)
	}
}
