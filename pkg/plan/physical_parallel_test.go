package plan

import (
	"context"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalBlockingPlanConcurrentReuse(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	leftVector := store.MakeVector(a, 3, dtype.Int32T(), false)
	copy(leftVector.I32s(), []int32{1, 1, 2})
	leftBatch := store.MakeBatch([]store.Vector{leftVector})
	leftTable := store.MakeTable([]*store.Batch{leftBatch})
	leftBatch.Release()
	rightVector := store.MakeVector(a, 2, dtype.Int32T(), false)
	copy(rightVector.I32s(), []int32{1, 2})
	rightBatch := store.MakeBatch([]store.Vector{rightVector})
	rightTable := store.MakeTable([]*store.Batch{rightBatch})
	rightBatch.Release()
	left := MakeScan(leftTable, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int32T()})).As("l")
	right := MakeScan(rightTable, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int32T()})).As("r")
	physical, err := MakePhysicalPlan(left.Join(right, JoinInner, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")}).Aggregate(MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	leftTable.Release()
	rightTable.Release()
	const workers = 8
	var wait sync.WaitGroup
	failures := make(chan string, workers)
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(batch *store.Batch) bool {
				if batch.Len() != 1 || batch.VectorAt(0).I64s()[0] != 3 {
					failures <- "incorrect concurrent aggregation"
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
	physical.Release()
	for failure := range failures {
		t.Error(failure)
	}
}
