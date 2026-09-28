package plan

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rhawrami/peGosus/pkg/io/csv"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkExternalIOScans(b *testing.B) {
	expected := pairedScanData(b)
	names := []string{"id", "seq", "measure", "score", "active", "day", "observed", "category", "message"}
	nullable := make([]bool, len(names))
	for i := range nullable {
		nullable[i] = true
	}
	schema := MakeSchemaWithNullability(names, pairedScanTypes(), nullable)
	for _, shape := range []string{"full", "projected", "filter-project", "selective-filter"} {
		for _, name := range []string{"scan_snappy.parquet", "scan_plain.parquet", "scan.csv"} {
			b.Run(shape+"/"+name, func(b *testing.B) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				path := filepath.Join(externalIOData, "paired", name)
				var scan LogicalPlan
				if filepath.Ext(path) == ".csv" {
					scan = MakeCSVScan(path, schema, csv.CSVOptions{HasHeader: true})
				} else {
					scan = MakeParquetScan(path, schema, parquet.ParquetOptions{})
				}
				switch shape {
				case "projected":
					scan = scan.Project(MakeColumn("id"), MakeColumn("score"))
				case "filter-project":
					scan = scan.Filter(MakeColumn("id").Ge(1000)).Filter(MakeColumn("active")).Project(MakeColumn("id"), MakeColumn("message"))
				case "selective-filter":
					scan = scan.Filter(MakeColumn("id").Ge(16000)).Filter(MakeColumn("active")).Project(MakeColumn("id"), MakeColumn("message"))
				}
				p, err := MakePhysicalPlan(scan)
				if err != nil {
					b.Fatal(err)
				}
				defer p.Release()
				want := expected.Rows
				if shape == "filter-project" {
					want = expected.Filter.Rows
				} else if shape == "selective-filter" {
					want = expected.Selective.Rows
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					count := 0
					result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20}, func(batch *store.Batch) bool {
						count += batch.ActiveLen()
						return true
					})
					if result.Code() != ExecutionCompleted || count != want {
						b.Fatalf("scan %v/%v: %d rows, want %d", result.Code(), result.Err(), count, want)
					}
				}
				b.ReportMetric(float64(expected.Rows)*float64(b.N)/b.Elapsed().Seconds(), "input-rows/s")
			})
		}
	}
}
