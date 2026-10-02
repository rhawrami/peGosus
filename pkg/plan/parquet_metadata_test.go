package plan

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParquetPreparedMetadataChecksSourceAgain(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(parquetScanFixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	schema := MakeSchemaWithNullability([]string{"id", "txt"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}, []bool{true, true})
	p, bindErr := MakePhysicalPlan(MakeParquetScan(path, schema, parquet.ParquetOptions{}).Project(MakeColumn("id")))
	if bindErr != nil {
		t.Fatal(bindErr)
	}
	defer p.Release()
	retained := p.source.Retain()
	if p.source.parquetMetadata == nil || retained.parquetMetadata != p.source.parquetMetadata {
		t.Fatal("retained scan did not share metadata cache")
	}
	retained.Release()
	for _, value := range []int32{0, 17, 0} {
		changed := bytes.Clone(data)
		binary.LittleEndian.PutUint32(changed[27:31], uint32(value))
		if err := os.WriteFile(path, changed, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
			t.Fatal(err)
		}
		calls := 0
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(batch *store.Batch) bool {
			calls++
			if batch.VectorAt(0).I32s()[0] != value {
				t.Fatalf("same-size overwritten file returned stale payload: %v", batch.VectorAt(0).I32s())
			}
			return true
		})
		if result.Code() != ExecutionCompleted || calls != 1 {
			t.Fatalf("execution %v, calls %d, error %v", result.Code(), calls, result.Err())
		}
	}
	footerStart := len(data) - 8 - int(binary.LittleEndian.Uint32(data[len(data)-8:]))
	changed := bytes.Clone(data)
	copy(changed[footerStart:], bytes.ReplaceAll(changed[footerStart:len(changed)-8], []byte("id"), []byte("ix")))
	if err := os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(*store.Batch) bool {
		t.Fatal("changed schema reached sink")
		return true
	})
	var failure *parquet.ParquetError
	if result.Code() != ExecutionSourceFailure || !errors.As(result.Err(), &failure) || failure.Code() != parquet.ParquetInvalid {
		t.Fatalf("same-size changed footer skipped schema validation: %v, %v", result.Code(), result.Err())
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	result = p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(*store.Batch) bool { return true })
	if result.Code() != ExecutionCompleted {
		t.Fatalf("source restored after schema change: %v", result.Err())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = p.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 16384}, func(*store.Batch) bool { t.Fatal("warm cancelled scan reached sink"); return true })
	if result.Code() != ExecutionCancelled || !errors.Is(result.Err(), context.Canceled) {
		t.Fatalf("warm cancellation: %v, %v", result.Code(), result.Err())
	}
}

func TestParquetPreparedMetadataParallelSourceChecks(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("parallel execution requires two workers")
	}
	encoded, err := os.ReadFile("../io/parquet/testdata/duckdb-groups.b64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	schema := MakeSchemaWithNullability([]string{"txt"}, []dtype.Type{dtype.StringT()}, []bool{true})
	p, bindErr := MakePhysicalPlan(MakeParquetScan(path, schema, parquet.ParquetOptions{}).Aggregate(MakeCountStar()))
	if bindErr != nil {
		t.Fatal(bindErr)
	}
	defer p.Release()
	options := ExecutionOptions{Workers: 2, MemoryBudget: 2 << 20}
	for range 2 {
		calls := 0
		result, handled := p.executeParallel(context.Background(), a, options, func(batch *store.Batch) bool {
			calls++
			if batch.VectorAt(0).I64s()[0] != 2050 {
				t.Fatal("wrong parallel count")
			}
			return true
		})
		if !handled || result.Code() != ExecutionCompleted || calls != 1 {
			t.Fatalf("parallel source: handled=%t, code=%v, error=%v, calls=%d", handled, result.Code(), result.Err(), calls)
		}
	}
	footerStart := len(data) - 8 - int(binary.LittleEndian.Uint32(data[len(data)-8:]))
	changed := bytes.Clone(data)
	copy(changed[footerStart:], bytes.ReplaceAll(changed[footerStart:len(changed)-8], []byte("txt"), []byte("bad")))
	if err := os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	result, handled := p.executeParallel(context.Background(), a, options, func(*store.Batch) bool {
		t.Fatal("changed schema reached parallel sink")
		return true
	})
	if !handled || result.Code() != ExecutionSourceFailure {
		t.Fatalf("warm parallel schema validation: handled=%t, code=%v, error=%v", handled, result.Code(), result.Err())
	}
}
