package plan

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkParquetScanFilterProject(b *testing.B) {
	fixture, err := os.ReadFile("../io/parquet/testdata/duckdb-groups.b64")
	if err != nil {
		b.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(fixture)))
	if err != nil {
		b.Fatal(err)
	}
	path := filepath.Join(b.TempDir(), "bench.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		b.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	schema := MakeSchemaWithNullability([]string{"txt"}, []dtype.Type{dtype.StringT()}, []bool{true})
	p, err := MakePhysicalPlan(MakeParquetScan(path, schema, parquet.ParquetOptions{}).Filter(MakeColumn("txt").IsNotNull()).Project(MakeColumn("txt")))
	if err != nil {
		b.Fatal(err)
	}
	defer p.Release()
	b.SetBytes(2050)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		count := 0
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20}, func(batch *store.Batch) bool {
			count += batch.ActiveLen()
			return true
		})
		if result.Code() != ExecutionCompleted || count != 1367 {
			b.Fatalf("scan returned %d rows: %v/%v", count, result.Code(), result.Err())
		}
	}
}
