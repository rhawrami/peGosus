package plan

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParallelTableStreamingAndEarlyStop(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 12)
	for chunk := range batches {
		ids := store.MakeVector(a, 257, dtype.Int32T(), false)
		stringsIn := make([][]byte, 257)
		for row := range 257 {
			id := chunk*257 + row
			ids.I32s()[row] = int32(id)
			stringsIn[row] = []byte(fmt.Sprintf("independent owned string %06d", id))
		}
		text := store.MakeStringVector(a, stringsIn, nil)
		batches[chunk] = store.MakeBatch([]store.Vector{ids, text})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	schema := MakeSchemaWithNullability([]string{"id", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{false, false})
	p, err := MakePhysicalPlan(MakeScan(table, schema).Filter(MakeColumn("id").Ge(500)).Project(MakeColumn("id").Add(1), MakeColumn("text")))
	if err != nil {
		t.Fatal(err)
	}
	var retained []*store.Batch
	var active atomic.Int32
	opts := ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}
	result, parallel := p.executeParallel(context.Background(), a, opts, func(batch *store.Batch) bool {
		if active.Add(1) != 1 {
			t.Error("sink calls overlapped")
		}
		defer active.Add(-1)
		retained = append(retained, batch.Retain())
		return true
	})
	if !parallel || result.Code() != ExecutionCompleted || len(retained) != len(batches) {
		t.Fatalf("parallel streaming %t, %v/%v, %d batches", parallel, result.Code(), result.Err(), len(retained))
	}
	count := 0
	for _, batch := range retained {
		count += batch.ActiveLen()
	}
	if count != 12*257-500 {
		t.Fatalf("selected %d rows", count)
	}
	stopCalls := 0
	result = p.ExecuteWithOptions(context.Background(), a, opts, func(*store.Batch) bool { stopCalls++; return false })
	if result.Code() != ExecutionStopped || stopCalls != 1 {
		t.Fatalf("early stop %v/%v calls %d", result.Code(), result.Err(), stopCalls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result = p.ExecuteWithOptions(ctx, a, opts, func(*store.Batch) bool { cancel(); return true })
	if result.Code() != ExecutionCancelled {
		t.Fatalf("sink cancellation %v/%v", result.Code(), result.Err())
	}
	small, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20}, func(*store.Batch) bool { t.Fatal("small auto query unexpectedly used parallel path"); return true })
	if used || small.Code() != ExecutionCompleted {
		t.Fatalf("auto worker gate %t/%v", used, small.Code())
	}
	if result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 512, Workers: 4}, func(*store.Batch) bool { t.Fatal("over-budget query reached sink"); return true }); result.Code() != ExecutionResourceExhausted {
		t.Fatalf("budget result %v/%v", result.Code(), result.Err())
	}
	p.Release()
	table.Release()
	for _, batch := range retained {
		if batch.VectorAt(1).Strings()[0].View() == "" {
			t.Fatal("retained string backing was lost")
		}
		batch.Release()
	}
}

func TestParallelGlobalAggregateMerge(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 8)
	var active, valid, sum int64
	minText, maxText := "", ""
	for chunk := range batches {
		ids := store.MakeVector(a, 256, dtype.Int32T(), true)
		stringsIn := make([][]byte, 256)
		selection := store.MakeBitMap(a, 256)
		selection.SetAll()
		for row := range 256 {
			id := chunk*256 + row
			ids.I32s()[row] = int32(id)
			value := fmt.Sprintf("retained aggregate long string %06d", id)
			stringsIn[row] = []byte(value)
			if id%13 == 0 {
				ids.Validity().Clear(row)
			}
			if id%7 == 0 {
				selection.Clear(row)
				continue
			}
			active++
			if id%13 != 0 {
				valid++
				sum += int64(id)
			}
			if minText == "" || value < minText {
				minText = value
			}
			if value > maxText {
				maxText = value
			}
		}
		text := store.MakeStringVector(a, stringsIn, nil)
		batch := store.MakeBatch([]store.Vector{ids, text})
		batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
		batches[chunk] = batch
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	schema := MakeSchemaWithNullability([]string{"id", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{true, false})
	p, err := MakePhysicalPlan(MakeScan(table, schema).Aggregate(MakeCountStar(), MakeCount(MakeColumn("id")), MakeSum(MakeColumn("id")), MakeMin(MakeColumn("text")), MakeMax(MakeColumn("text"))))
	if err != nil {
		t.Fatal(err)
	}
	var output *store.Batch
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(batch *store.Batch) bool { output = batch.Retain(); return true })
	if !parallel || result.Code() != ExecutionCompleted || output == nil {
		t.Fatalf("aggregate %t %v/%v", parallel, result.Code(), result.Err())
	}
	p.Release()
	table.Release()
	defer output.Release()
	if output.VectorAt(0).I64s()[0] != active || output.VectorAt(1).I64s()[0] != valid || output.VectorAt(2).I64s()[0] != sum || output.VectorAt(3).Strings()[0].View() != minText || output.VectorAt(4).Strings()[0].View() != maxText {
		t.Fatalf("incorrect merged aggregates: rows %d valid %d sum %d min %q max %q", output.VectorAt(0).I64s()[0], output.VectorAt(1).I64s()[0], output.VectorAt(2).I64s()[0], output.VectorAt(3).Strings()[0].View(), output.VectorAt(4).Strings()[0].View())
	}
}

func TestParallelAutoAggregateAndSerialFallback(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 16)
	for chunk := range batches {
		vector := store.MakeVector(a, 1024, dtype.Int32T(), false)
		for row := range 1024 {
			vector.I32s()[row] = int32(chunk*1024 + row)
		}
		batches[chunk] = store.MakeBatch([]store.Vector{vector})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	defer table.Release()
	scan := MakeScan(table, MakeSchema([]string{"id"}, []dtype.Type{dtype.Int32T()}))
	p, err := MakePhysicalPlan(scan.Aggregate(MakeCountStar(), MakeSum(MakeColumn("id"))))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	outputCalls := 0
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20}, func(batch *store.Batch) bool {
		outputCalls++
		if batch.VectorAt(0).I64s()[0] != 16*1024 || batch.VectorAt(1).I64s()[0] != 16384*16383/2 {
			t.Error("auto-merged aggregate mismatch")
		}
		return true
	})
	if !parallel || result.Code() != ExecutionCompleted || outputCalls != 1 {
		t.Fatalf("auto aggregate %t/%v, calls %d", parallel, result.Code(), outputCalls)
	}
	limit, err := MakePhysicalPlan(scan.Limit(5))
	if err != nil {
		t.Fatal(err)
	}
	defer limit.Release()
	if _, parallel := limit.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(*store.Batch) bool { t.Fatal("parallel limit reached sink"); return true }); parallel {
		t.Fatal("LIMIT incorrectly entered parallel pipeline")
	}
	count := 0
	result = limit.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(batch *store.Batch) bool { count += batch.ActiveLen(); return true })
	if result.Code() != ExecutionCompleted || count != 5 {
		t.Fatalf("LIMIT serial fallback %v/%v, %d rows", result.Code(), result.Err(), count)
	}
	for _, query := range []LogicalPlan{scan.OrderBy(MakeOrderKey(MakeColumn("id"))), scan.Distinct(), scan.GroupBy([]Expr{MakeColumn("id")}, MakeCountDistinct(MakeColumn("id")))} {
		blocking, err := MakePhysicalPlan(query)
		if err != nil {
			t.Fatal(err)
		}
		if _, parallel := blocking.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(*store.Batch) bool { t.Fatal("blocking operator reached parallel sink"); return true }); parallel {
			t.Fatal("blocking operator entered parallel pipeline")
		}
		blocking.Release()
	}
	distinctAggregate, err := MakePhysicalPlan(scan.Aggregate(MakeCountDistinct(MakeColumn("id"))))
	if err != nil {
		t.Fatal(err)
	}
	if _, parallel := distinctAggregate.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(*store.Batch) bool { t.Fatal("aggregate DISTINCT reached parallel sink"); return true }); parallel {
		t.Fatal("aggregate DISTINCT entered parallel pipeline")
	}
	distinctAggregate.Release()
	if result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: -1}, func(*store.Batch) bool { return true }); result.Code() != ExecutionInvalidInvocation {
		t.Fatalf("negative worker count: %v", result.Code())
	}
}

func TestParallelAllNullAggregate(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 4)
	for chunk := range batches {
		vector := store.MakeVector(a, 257, dtype.Int32T(), true)
		vector.Validity().ClearAll()
		batches[chunk] = store.MakeBatch([]store.Vector{vector})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	defer table.Release()
	schema := MakeSchemaWithNullability([]string{"id"}, []dtype.Type{dtype.Int32T()}, []bool{true})
	p, err := MakePhysicalPlan(MakeScan(table, schema).Aggregate(MakeCountStar(), MakeCount(MakeColumn("id")), MakeSum(MakeColumn("id"))))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 16 << 20, Workers: 4}, func(batch *store.Batch) bool {
		if batch.VectorAt(0).I64s()[0] != 4*257 || batch.VectorAt(1).I64s()[0] != 0 || batch.VectorAt(2).Validity().IsSet(0) {
			t.Error("all-null aggregate result changed")
		}
		return true
	})
	if !parallel || result.Code() != ExecutionCompleted {
		t.Fatalf("all-null merge %t %v/%v", parallel, result.Code(), result.Err())
	}
}

func TestParallelFloatAggregateMerge(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 4)
	for i := range batches {
		vector := store.MakeVector(a, 256, dtype.Float64T(), true)
		for row := range 256 {
			if row%37 == 0 {
				vector.Validity().Clear(row)
			}
			if i%2 == 0 {
				vector.F64s()[row] = math.Copysign(0, -1)
			} else {
				vector.F64s()[row] = 0
			}
		}
		vector.F64s()[1] = math.NaN()
		batches[i] = store.MakeBatch([]store.Vector{vector})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	defer table.Release()
	schema := MakeSchemaWithNullability([]string{"v"}, []dtype.Type{dtype.Float64T()}, []bool{true})
	p, err := MakePhysicalPlan(MakeScan(table, schema).Aggregate(MakeMin(MakeColumn("v")), MakeMax(MakeColumn("v"))))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 16 << 20, Workers: 4}, func(batch *store.Batch) bool {
		minValue, maxValue := batch.VectorAt(0).F64s()[0], batch.VectorAt(1).F64s()[0]
		if !math.Signbit(minValue) || math.Signbit(maxValue) || math.IsNaN(minValue) || math.IsNaN(maxValue) {
			t.Errorf("signed-zero preference lost: min %v, max %v", minValue, maxValue)
		}
		return true
	})
	if !parallel || result.Code() != ExecutionCompleted {
		t.Fatalf("float merge %t %v/%v", parallel, result.Code(), result.Err())
	}
}

func TestParallelQueryBudgetFailure(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 2)
	for i := range batches {
		v := store.MakeVector(a, 200000, dtype.Int32T(), false)
		batches[i] = store.MakeBatch([]store.Vector{v})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	defer table.Release()
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"id"}, []dtype.Type{dtype.Int32T()})).Project(MakeColumn("id").Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 2 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("over-budget query reached sink"); return true })
	if !parallel || result.Code() != ExecutionResourceExhausted {
		t.Fatalf("parallel resource failure %t %v/%v", parallel, result.Code(), result.Err())
	}
}

func TestParallelPlanConcurrentReuse(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two GOMAXPROCS")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 8)
	for i := range batches {
		v := store.MakeVector(a, 512, dtype.Int32T(), false)
		batches[i] = store.MakeBatch([]store.Vector{v})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"id"}, []dtype.Type{dtype.Int32T()})).Aggregate(MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	var wg sync.WaitGroup
	failures := make(chan string, 16)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(batch *store.Batch) bool {
				if batch.Len() != 1 || batch.VectorAt(0).I64s()[0] != 8*512 {
					failures <- "wrong count"
				}
				return true
			})
			if result.Code() != ExecutionCompleted {
				failures <- "parallel query failed"
			}
		}()
	}
	wg.Wait()
	close(failures)
	p.Release()
	for failure := range failures {
		t.Error(failure)
	}
}
