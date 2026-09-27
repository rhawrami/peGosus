package plan

import (
	"context"
	"errors"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestExecutionOutcomes(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	batches := make([]*store.Batch, 3)
	for i := range batches {
		v := store.MakeVector(a, 2, dtype.Int32T(), false)
		v.I32s()[0] = int32(i)
		batches[i] = store.MakeBatch([]store.Vector{v})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int32T()})).Project(MakeColumn("x").Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	defer physical.Release()
	options := ExecutionOptions{MemoryBudget: 1024}
	var count int
	result := physical.ExecuteWithOptions(context.Background(), a, options, func(*store.Batch) bool {
		count++
		return count < 2
	})
	if result.Code() != ExecutionStopped || count != 2 {
		t.Fatalf("sink termination: got %v after %d batches", result.Code(), count)
	}
	if result = physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1}, func(*store.Batch) bool {
		t.Fatal("budget exceeded before sink")
		return true
	}); result.Code() != ExecutionResourceExhausted {
		t.Fatalf("budget: got %v", result.Code())
	}
	if result = physical.ExecuteWithOptions(nil, a, options, func(*store.Batch) bool { return true }); result.Code() != ExecutionInvalidInvocation {
		t.Fatalf("invalid invocation: got %v", result.Code())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = physical.ExecuteWithOptions(ctx, a, options, func(*store.Batch) bool {
		t.Fatal("cancelled query reached sink")
		return true
	})
	if result.Code() != ExecutionCancelled || !errors.Is(result.Err(), context.Canceled) {
		t.Fatalf("pre-cancelled query: got %v / %v", result.Code(), result.Err())
	}
	ctx, cancel = context.WithCancel(context.Background())
	count = 0
	result = physical.ExecuteWithOptions(ctx, a, options, func(*store.Batch) bool {
		count++
		cancel()
		return true
	})
	if result.Code() != ExecutionCancelled || count != 1 {
		t.Fatalf("mid-query cancellation: got %v after %d batches", result.Code(), count)
	}
	count = 0
	result = physical.ExecuteWithOptions(context.Background(), a, options, func(*store.Batch) bool { count++; return true })
	if result.Code() != ExecutionCompleted || count != 3 {
		t.Fatalf("normal completion: got %v after %d batches", result.Code(), count)
	}
}
