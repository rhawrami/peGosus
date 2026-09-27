package plan

import (
	"context"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalBlockingEmptyAndEarlySink(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	table := store.MakeEmptyTable([]dtype.Type{dtype.Int32T()})
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int32T()}))
	for _, query := range []LogicalPlan{
		scan.Distinct(),
		scan.OrderBy(MakeOrderKey(MakeColumn("x"))),
		scan.GroupBy([]Expr{MakeColumn("x")}, MakeCountStar()),
	} {
		physical, err := MakePhysicalPlan(query)
		if err != nil {
			t.Fatal(err)
		}
		called := false
		result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096}, func(*store.Batch) bool { called = true; return true })
		physical.Release()
		if result.Code() != ExecutionCompleted || called {
			t.Fatalf("empty blocking query: %v, sink called %t", result.Code(), called)
		}
	}
	table.Release()
	v := store.MakeVector(a, 2, dtype.Int32T(), false)
	copy(v.I32s(), []int32{2, 1})
	b := store.MakeBatch([]store.Vector{v})
	table = store.MakeTable([]*store.Batch{b})
	b.Release()
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int32T()})).OrderBy(MakeOrderKey(MakeColumn("x"))))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	called := 0
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096}, func(*store.Batch) bool { called++; return false })
	physical.Release()
	if result.Code() != ExecutionStopped || called != 1 {
		t.Fatalf("blocking early sink: %v after %d calls", result.Code(), called)
	}
}
