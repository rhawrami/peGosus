package plan

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkParallelTablePipeline(b *testing.B) {
	for _, shape := range []struct{ rows, batchSize int }{{1024, 256}, {16384, 1024}, {65536, 4096}, {262144, 4096}} {
		b.Run(fmt.Sprintf("rows=%d", shape.rows), func(b *testing.B) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			batches := make([]*store.Batch, 0, (shape.rows+shape.batchSize-1)/shape.batchSize)
			for at := 0; at < shape.rows; at += shape.batchSize {
				n := min(shape.batchSize, shape.rows-at)
				v := store.MakeVector(a, n, dtype.Int32T(), false)
				for row := range n {
					v.I32s()[row] = int32(at + row)
				}
				batches = append(batches, store.MakeBatch([]store.Vector{v}))
			}
			table := store.MakeTable(batches)
			for _, batch := range batches {
				batch.Release()
			}
			defer table.Release()
			for _, kind := range []string{"stream", "aggregate"} {
				scan := MakeScan(table, MakeSchema([]string{"id"}, []dtype.Type{dtype.Int32T()}))
				if kind == "stream" {
					scan = scan.Filter(MakeColumn("id").Ge(0)).Project(MakeColumn("id").Add(1))
				} else {
					scan = scan.Aggregate(MakeCountStar(), MakeSum(MakeColumn("id")))
				}
				p, err := MakePhysicalPlan(scan)
				if err != nil {
					b.Fatal(err)
				}
				for _, mode := range []struct {
					name    string
					workers int
				}{{"serial", 1}, {"auto", 0}, {"four", 4}, {"eight", 8}} {
					b.Run(kind+"/"+mode.name, func(b *testing.B) {
						b.ReportAllocs()
						b.ResetTimer()
						for range b.N {
							count := 0
							result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: mode.workers}, func(batch *store.Batch) bool {
								if kind == "stream" {
									count += batch.ActiveLen()
								} else {
									count += int(batch.VectorAt(0).I64s()[0])
									if got, want := batch.VectorAt(1).I64s()[0], int64(shape.rows)*(int64(shape.rows)-1)/2; got != want {
										b.Fatalf("aggregate sum %d, want %d", got, want)
									}
								}
								return true
							})
							if result.Code() != ExecutionCompleted || count != shape.rows {
								b.Fatalf("scan %v/%v: %d rows", result.Code(), result.Err(), count)
							}
						}
						b.ReportMetric(float64(shape.rows)*float64(b.N)/b.Elapsed().Seconds(), "rows/s")
					})
				}
				p.Release()
			}
		})
	}
}

func BenchmarkParallelParquetPipeline(b *testing.B) {
	expected := pairedScanData(b)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	names := []string{"id", "seq", "measure", "score", "active", "day", "observed", "category", "message"}
	nullable := make([]bool, len(names))
	for i := range nullable {
		nullable[i] = true
	}
	path := filepath.Join(externalIOData, "paired", "scan_snappy.parquet")
	scan := MakeParquetScan(path, MakeSchemaWithNullability(names, pairedScanTypes(), nullable), parquet.ParquetOptions{})
	for _, kind := range []string{"stream", "global-aggregate"} {
		var query LogicalPlan
		if kind == "stream" {
			query = scan.Filter(MakeColumn("id").Ge(0)).Project(MakeColumn("id"), MakeColumn("message"))
		} else {
			query = scan.Filter(MakeColumn("id").Ge(0)).Aggregate(MakeCountStar())
		}
		p, err := MakePhysicalPlan(query)
		if err != nil {
			b.Fatal(err)
		}
		for _, mode := range []struct {
			name    string
			workers int
		}{{"serial", 1}, {"auto", 0}, {"four", 4}, {"eight", 8}} {
			b.Run(kind+"/"+mode.name, func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					count := 0
					result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: mode.workers}, func(batch *store.Batch) bool {
						if kind == "stream" {
							count += batch.ActiveLen()
						} else {
							count += int(batch.VectorAt(0).I64s()[0])
						}
						return true
					})
					if result.Code() != ExecutionCompleted || count != expected.Rows {
						b.Fatalf("scan %v/%v: %d rows", result.Code(), result.Err(), count)
					}
				}
				b.ReportMetric(float64(expected.Rows)*float64(b.N)/b.Elapsed().Seconds(), "rows/s")
			})
		}
		p.Release()
	}
}
