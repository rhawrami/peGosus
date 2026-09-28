package plan

import (
	"context"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestExecutionScopedMappingFailureAndReuse(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	text := store.MakeStringVector(a, [][]byte{[]byte("a")}, nil)
	batch := store.MakeBatch([]store.Vector{text})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	expr := MakeColumn("text")
	for range 5 {
		expr = expr.Replace("a", "aaaa")
	}
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"text"}, []dtype.Type{dtype.StringT()})).Project(expr))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	called := false
	result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1200}, func(*store.Batch) bool { called = true; return true })
	if result.Code() != ExecutionResourceExhausted || called {
		t.Fatalf("budgeted expression: result=%v called=%t", result.Code(), called)
	}
	var output *store.Batch
	result = plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20}, func(batch *store.Batch) bool { output = batch.Retain(); return true })
	plan.Release()
	if result.Code() != ExecutionCompleted || output == nil || output.VectorAt(0).Strings()[0].View() != strings.Repeat("a", 1024) {
		t.Fatalf("reused expression after resource error: result=%v", result.Code())
	}
	output.Release()
}

func TestExecutionScopedEmptyBoolean(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	vector := store.MakeVector(a, 0, dtype.BoolT(), true)
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"flag"}, []dtype.Type{dtype.BoolT()})).Project(MakeColumn("flag").Not(), MakeColumn("flag").And(MakeLiteral(true))))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128}, func(b *store.Batch) bool {
		if b.Len() != 0 || b.NVectors() != 2 {
			t.Fatal("unexpected empty boolean output")
		}
		return true
	})
	plan.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("empty boolean result: %v", result.Code())
	}
}
