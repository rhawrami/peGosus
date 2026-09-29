package plan

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/csv"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestCSVScanFilterProjectAcrossBatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.csv")
	data := "name,amount\nlong name first value,4\nlong name second value,12\nlong name third value,19\nshort,\\N\nlong name fifth value,20\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	schema := MakeSchemaWithNullability([]string{"name", "amount"}, []dtype.Type{dtype.StringT(), dtype.Int32T()}, []bool{false, true})
	scan := MakeCSVScan(path, schema, csv.CSVOptions{HasHeader: true, BatchSize: 2})
	plan, err := MakePhysicalPlan(scan.Filter(MakeColumn("amount").Gt(10)).Project(MakeColumn("name"), MakeColumn("amount").Add(1)).Limit(2))
	if err != nil {
		t.Fatal(err)
	}
	var outputs []*store.Batch
	result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(batch *store.Batch) bool { outputs = append(outputs, batch.Retain()); return true })
	plan.Release()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if result.Code() != ExecutionCompleted || len(outputs) != 2 || outputs[0].ActiveLen() != 1 || outputs[1].ActiveLen() != 1 {
		t.Fatalf("CSV pipeline result %v error=%v, batches=%d", result.Code(), result.Err(), len(outputs))
	}
	for _, batch := range outputs {
		defer batch.Release()
	}
	if outputs[0].Len() != 2 || outputs[0].VectorAt(0).Strings()[1].View() != "long name second value" || outputs[0].VectorAt(1).I32s()[1] != 13 || outputs[1].VectorAt(0).Strings()[0].View() != "long name third value" || outputs[1].VectorAt(1).I32s()[0] != 20 {
		t.Fatal("retained CSV payload, filter selection, or projection was corrupted")
	}
}

func TestCSVScanStopsBeforeMalformedTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.csv")
	if err := os.WriteFile(path, []byte("name,amount\na,1\nb,2\nc,broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	scan := MakeCSVScan(path, MakeSchema([]string{"name", "amount"}, []dtype.Type{dtype.StringT(), dtype.Int32T()}), csv.CSVOptions{HasHeader: true, BatchSize: 2})
	plan, err := MakePhysicalPlan(scan)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(*store.Batch) bool { count++; return false })
	if result.Code() != ExecutionStopped || count != 1 {
		t.Fatalf("early sink result %v, calls %d", result.Code(), count)
	}
	result = plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(*store.Batch) bool { return true })
	var csvErr *csv.CSVError
	if result.Code() != ExecutionSourceFailure || !errors.As(result.Err(), &csvErr) || csvErr.Code() != csv.CSVInvalidValue || csvErr.Record() != 3 || csvErr.Column() != 1 {
		t.Fatalf("source error %v: %v", result.Code(), result.Err())
	}
	limited, err := MakePhysicalPlan(scan.Limit(1))
	if err != nil {
		t.Fatal(err)
	}
	result = limited.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(batch *store.Batch) bool {
		if batch.ActiveLen() != 1 {
			t.Fatal("LIMIT was not applied")
		}
		return true
	})
	limited.Release()
	plan.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("limit read malformed tail: %v", result.Code())
	}
}

func TestCSVScanCancellationAndConcurrentReuse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "parallel.csv")
	if err := os.WriteFile(path, []byte("k\n1\n2\n3\n4\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	plan, err := MakePhysicalPlan(MakeCSVScan(path, MakeSchema([]string{"k"}, []dtype.Type{dtype.Int32T()}), csv.CSVOptions{HasHeader: true, BatchSize: 2}).Aggregate(MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := plan.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 8192}, func(*store.Batch) bool { t.Fatal("cancelled scan called sink"); return true })
	if result.Code() != ExecutionCancelled || !errors.Is(result.Err(), context.Canceled) {
		t.Fatalf("cancelled scan: %v", result.Code())
	}
	var wait sync.WaitGroup
	failures := make(chan string, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(batch *store.Batch) bool {
				if batch.Len() != 1 || batch.VectorAt(0).I64s()[0] != 4 {
					failures <- "wrong count"
				}
				return true
			})
			if result.Code() != ExecutionCompleted {
				failures <- "query failed"
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

func TestCSVScanEmptyAndMissingSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.csv")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	scan := MakeCSVScan(path, MakeSchema([]string{"k"}, []dtype.Type{dtype.Int32T()}), csv.CSVOptions{})
	plan, err := MakePhysicalPlan(scan.Aggregate(MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096}, func(batch *store.Batch) bool {
		if batch.Len() != 1 || batch.VectorAt(0).I64s()[0] != 0 {
			t.Fatal("empty CSV global count")
		}
		return true
	})
	plan.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("empty CSV: %v", result.Code())
	}
	missing, err := MakePhysicalPlan(MakeCSVScan(path+"-missing", scan.root.schema, csv.CSVOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	result = missing.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096}, func(*store.Batch) bool { t.Fatal("missing file called sink"); return true })
	missing.Release()
	if result.Code() != ExecutionSourceFailure || !errors.Is(result.Err(), os.ErrNotExist) {
		t.Fatalf("missing CSV source: %v/%v", result.Code(), result.Err())
	}
}

func TestCSVScanProjectionPushdownPreservesFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wide.csv")
	if err := os.WriteFile(path, []byte("name,bad,score,unused\nlong source name,not-an-integer,10,irrelevant\nshort,also-bad,2,ignored\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	schema := MakeSchema([]string{"name", "bad", "score", "unused"}, []dtype.Type{dtype.StringT(), dtype.Int32T(), dtype.Int32T(), dtype.StringT()})
	query := MakeCSVScan(path, schema, csv.CSVOptions{HasHeader: true, BatchSize: 2}).Filter(MakeColumn("score").Gt(5)).Project(MakeColumn("name"), MakeColumn("score"))
	bound, err := BindLogicalPlan(query)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := MakePhysicalPlanFromBound(bound)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.source.projection; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("source columns %v, want [0 2]", got)
	}
	if len(plan.source.filters) != 1 || plan.steps[0].operation != physicalPushedFilter {
		t.Fatal("eligible bound filter was not placed at the scan")
	}
	for i := range bound.Schema().Len() {
		if bound.Schema().FieldAt(i).ID() != plan.Schema().FieldAt(i).ID() {
			t.Fatal("pushdown changed bound field IDs")
		}
	}
	result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(output *store.Batch) bool {
		if output.Len() != 2 || output.NVectors() != 2 || output.ActiveLen() != 1 || output.VectorAt(0).Strings()[0].View() != "long source name" || output.VectorAt(1).I32s()[0] != 10 {
			t.Fatal("projected CSV values or selection changed")
		}
		return true
	})
	plan.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("projected CSV: %v/%v", result.Code(), result.Err())
	}
}

func TestCSVScanAggregatePrunesUnusedColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "aggregate.csv")
	if err := os.WriteFile(path, []byte("id,bad,category\n1,not-an-integer,east\n2,still-invalid,east\n3,also-invalid,west\n"), 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	schema := MakeSchema([]string{"id", "bad", "category"}, []dtype.Type{dtype.Int32T(), dtype.Int32T(), dtype.StringT()})
	scan := MakeCSVScan(path, schema, csv.CSVOptions{HasHeader: true, BatchSize: 2})
	query := scan.Filter(MakeColumn("id").Ge(2)).GroupBy([]Expr{MakeColumn("category")}, MakeCountStar(), MakeSum(MakeColumn("id")))
	bound, err := BindLogicalPlan(query)
	if err != nil {
		t.Fatal(err)
	}
	physical, err := MakePhysicalPlanFromBound(bound)
	if err != nil {
		t.Fatal(err)
	}
	if got := physical.source.projection; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("aggregate needed source columns %v, want [0 2]", got)
	}
	for i := range bound.Schema().Len() {
		if bound.Schema().FieldAt(i).ID() != physical.Schema().FieldAt(i).ID() {
			t.Fatal("aggregate pruning changed field IDs")
		}
	}
	seen := make(map[string][2]int64)
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(batch *store.Batch) bool {
		for row := range batch.Len() {
			seen[batch.VectorAt(0).Strings()[row].View()] = [2]int64{batch.VectorAt(1).I64s()[row], batch.VectorAt(2).I64s()[row]}
		}
		return true
	})
	physical.Release()
	if result.Code() != ExecutionCompleted || len(seen) != 2 || seen["east"] != [2]int64{1, 2} || seen["west"] != [2]int64{1, 3} {
		t.Fatalf("aggregate after pruning: %v/%v, results %v", result.Code(), result.Err(), seen)
	}
	count, err := MakePhysicalPlan(scan.Aggregate(MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	if got := count.source.projection; len(got) != 1 || got[0] != 0 {
		t.Fatalf("COUNT(*) source projection %v, want [0]", got)
	}
	result = count.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(batch *store.Batch) bool {
		if batch.VectorAt(0).I64s()[0] != 3 {
			t.Fatal("COUNT(*) after pruning returned the wrong row count")
		}
		return true
	})
	count.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("COUNT(*) after pruning: %v/%v", result.Code(), result.Err())
	}
}

func TestTableProjectionPushdownRetainsPayload(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	x := store.MakeVector(a, 2, dtype.Int32T(), false)
	y := store.MakeVector(a, 2, dtype.Int32T(), false)
	z := store.MakeStringVector(a, [][]byte{[]byte("long retained string"), []byte("second long value")}, nil)
	copy(x.I32s(), []int32{1, 2})
	copy(y.I32s(), []int32{3, 4})
	batch := store.MakeBatch([]store.Vector{x, y, z})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x", "y", "z"}, []dtype.Type{dtype.Int32T(), dtype.Int32T(), dtype.StringT()})).Filter(MakeColumn("x").Gt(1)).Project(MakeColumn("z")))
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.source.projection; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("source columns %v, want [0 2]", got)
	}
	table.Release()
	var output *store.Batch
	if !plan.Execute(a, func(b *store.Batch) bool { output = b.Retain(); return true }) {
		t.Fatal("table scan failed")
	}
	plan.Release()
	if output == nil || output.ActiveLen() != 1 || output.VectorAt(0).Strings()[1].View() != "second long value" {
		t.Fatal("projected payload or filter selection was lost")
	}
	output.Release()
}

func TestScanFilterDoesNotCrossLimit(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	v := store.MakeVector(a, 2, dtype.Int32T(), false)
	copy(v.I32s(), []int32{1, 9})
	batch := store.MakeBatch([]store.Vector{v})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	query := MakeScan(table, MakeSchema([]string{"v"}, []dtype.Type{dtype.Int32T()})).Limit(1).Filter(MakeColumn("v").Gt(5))
	plan, err := MakePhysicalPlan(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.source.filters) != 0 || plan.steps[1].operation != physicalFilter {
		t.Fatal("moved filter across LIMIT")
	}
	table.Release()
	if !plan.Execute(a, func(output *store.Batch) bool {
		if output.ActiveLen() != 0 {
			t.Fatal("filter observed a row excluded by LIMIT")
		}
		return true
	}) {
		t.Fatal("LIMIT/filter execution failed")
	}
	plan.Release()
}
