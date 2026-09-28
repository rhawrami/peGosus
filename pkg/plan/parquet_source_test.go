package plan

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

// DuckDB 1.5.6, uncompressed PLAIN, nullable UTF-8 string.
const parquetScanFixture = "UEFSMRUAFSQVJCwVBhUAFQYVBgAAAgAAAAYBAAAAAAEAAAACAAAAFQAVlgEVlgEsFQYVABUGFQYAACEAAABBBQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAPAAAAYWFhYWFhYWFhYWFhYWFhDwAAAGNjY2NjY2NjY2NjY2NjYxUCGTw1ABgNZHVja2RiX3NjaGVtYRUEABUCJQIYAmlkJSIAFQwlAhgDdHh0JQAAFgYZHBksJgAcFQIZFQAZGAJpZBUAFgYWRhZGJgg8GAQCAAAAGAQAAAAAFgAoBAIAAAAYBAAAAAAREQAAACYAHBUMGRUAGRgDdHh0FQAWBha8ARa8ASZOPBgPY2NjY2NjY2NjY2NjY2NjGA9hYWFhYWFhYWFhYWFhYWEWAigPY2NjY2NjY2NjY2NjY2NjGA9hYWFhYWFhYWFhYWFhYWEREQAAABaCAhYGJggWggIAKChEdWNrREIgdmVyc2lvbiB2MS41LjYgKGJ1aWxkIDA2OWNjOWY5YjUpGSwcAAAcAAAAEAEAAFBBUjE="

func TestParquetScanFilterProjectRetain(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(parquetScanFixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	schema := MakeSchemaWithNullability([]string{"id", "txt"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{true, true})
	query := MakeParquetScan(path, schema, parquet.ParquetOptions{BatchSize: 2}).Filter(MakeColumn("id").Ge(1)).Project(MakeColumn("txt"))
	bound, err := BindLogicalPlan(query)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := MakePhysicalPlanFromBound(bound)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Schema().FieldAt(0).ID() != bound.Schema().FieldAt(0).ID() {
		t.Fatal("scan lowering changed field ID")
	}
	if len(plan.source.filters) != 1 {
		t.Fatal("scan pushdown missing")
	}
	var batches []*store.Batch
	result := plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(b *store.Batch) bool {
		batches = append(batches, b.Retain())
		return true
	})
	plan.Release()
	if result.Code() != ExecutionCompleted || len(batches) != 2 {
		t.Fatalf("execution %v %v, batches=%d", result.Code(), result.Err(), len(batches))
	}
	defer func() {
		for _, b := range batches {
			b.Release()
		}
	}()
	if batches[0].ActiveLen() != 1 || batches[0].VectorAt(0).Validity().IsSet(1) || batches[1].VectorAt(0).Strings()[0].View() != "ccccccccccccccc" {
		t.Fatal("filtered or retained values changed")
	}
	projected, err := MakePhysicalPlan(MakeParquetScan(path, schema, parquet.ParquetOptions{}).Project(MakeColumn("id")))
	if err != nil {
		t.Fatal(err)
	}
	if len(projected.source.projection) != 1 || projected.source.projection[0] != 0 {
		t.Fatal("column projection did not reach scan")
	}
	result = projected.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(b *store.Batch) bool {
		if b.NVectors() != 1 || b.VectorAt(0).I32s()[2] != 2 {
			t.Fatal("projected scan changed values")
		}
		return true
	})
	projected.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("projected scan: %v/%v", result.Code(), result.Err())
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if batches[0].VectorAt(0).Strings()[0].View() != "aaaaaaaaaaaaaaa" {
		t.Fatal("retained backing lost")
	}
}

func TestParquetScanExecutionOutcomes(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(parquetScanFixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	schema := MakeSchemaWithNullability([]string{"id", "txt"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{true, true})
	plan, err := MakePhysicalPlan(MakeParquetScan(path, schema, parquet.ParquetOptions{BatchSize: 1}))
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := plan.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 8192}, func(*store.Batch) bool { t.Fatal("cancelled query reached sink"); return true })
	if result.Code() != ExecutionCancelled || !errors.Is(result.Err(), context.Canceled) {
		t.Fatalf("cancelled: %v/%v", result.Code(), result.Err())
	}
	calls := 0
	result = plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(*store.Batch) bool { calls++; return false })
	if result.Code() != ExecutionStopped || calls != 1 {
		t.Fatalf("sink stopped: %v/%d", result.Code(), calls)
	}
	result = plan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32}, func(*store.Batch) bool { t.Fatal("over-budget query reached sink"); return true })
	if result.Code() != ExecutionResourceExhausted {
		t.Fatalf("resource admission: %v/%v", result.Code(), result.Err())
	}
	wrong := MakeSchemaWithNullability([]string{"id", "other"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{true, true})
	badPlan, err := MakePhysicalPlan(MakeParquetScan(path, wrong, parquet.ParquetOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	result = badPlan.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(*store.Batch) bool { t.Fatal("wrong schema reached sink"); return true })
	badPlan.Release()
	var pqErr *parquet.ParquetError
	if result.Code() != ExecutionSourceFailure || !errors.As(result.Err(), &pqErr) || pqErr.Code() != parquet.ParquetInvalid {
		t.Fatalf("schema failure: %v/%v", result.Code(), result.Err())
	}
}

func TestParquetScanConcurrentReuse(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(parquetScanFixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	schema := MakeSchemaWithNullability([]string{"id", "txt"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{true, true})
	p, err := MakePhysicalPlan(MakeParquetScan(path, schema, parquet.ParquetOptions{BatchSize: 2}).Aggregate(MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	var wg sync.WaitGroup
	resultErrors := make(chan error, 16)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(batch *store.Batch) bool {
				if batch.VectorAt(0).I64s()[0] != 3 {
					resultErrors <- errors.New("wrong aggregate result")
				}
				return true
			})
			if result.Code() != ExecutionCompleted {
				resultErrors <- result.Err()
			}
		}()
	}
	wg.Wait()
	close(resultErrors)
	for err := range resultErrors {
		t.Error(err)
	}
}
