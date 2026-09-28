package plan

import (
	"context"
	"fmt"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkParallelGroupedAggregation(b *testing.B) {
	const rows = 32768
	const batchSize = 1024
	for _, groups := range []int{16, 4096, rows} {
		for _, kind := range []string{"compact-count", "general-sum-string"} {
			b.Run(fmt.Sprintf("groups=%d/%s", groups, kind), func(b *testing.B) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				batches := make([]*store.Batch, rows/batchSize)
				for chunk := range batches {
					key := store.MakeVector(a, batchSize, dtype.Int32T(), true)
					value := store.MakeVector(a, batchSize, dtype.Int32T(), true)
					var stringsIn [][]byte
					if kind == "general-sum-string" {
						stringsIn = make([][]byte, batchSize)
					}
					for row := range batchSize {
						id := chunk*batchSize + row
						key.I32s()[row] = int32(id % groups)
						value.I32s()[row] = int32(id%17 - 8)
						if id%113 == 0 {
							key.Validity().Clear(row)
						}
						if id%17 == 0 {
							value.Validity().Clear(row)
						}
						if stringsIn != nil {
							stringsIn[row] = []byte(fmt.Sprintf("long group text %06d", id%groups))
						}
					}
					vectors := []store.Vector{key, value}
					if stringsIn != nil {
						vectors = append(vectors, store.MakeStringVector(a, stringsIn, nil))
					}
					batches[chunk] = store.MakeBatch(vectors)
				}
				table := store.MakeTable(batches)
				for _, batch := range batches {
					batch.Release()
				}
				defer table.Release()
				names := []string{"key", "value"}
				types := []dtype.Type{dtype.Int32T(), dtype.Int32T()}
				nullable := []bool{true, true}
				if kind == "general-sum-string" {
					names = append(names, "text")
					types = append(types, dtype.StringT())
					nullable = append(nullable, false)
				}
				scan := MakeScan(table, MakeSchemaWithNullability(names, types, nullable))
				var query LogicalPlan
				countColumn := 1
				if kind == "compact-count" {
					query = scan.GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeCount(MakeColumn("value")))
				} else {
					query = scan.GroupBy([]Expr{MakeColumn("key"), MakeColumn("text")}, MakeCountStar(), MakeSum(MakeColumn("value")), MakeMin(MakeColumn("text")))
					countColumn = 2
				}
				p, err := MakePhysicalPlan(query)
				if err != nil {
					b.Fatal(err)
				}
				defer p.Release()
				for _, mode := range []struct {
					name    string
					workers int
				}{{"serial", 1}, {"auto", 0}, {"four", 4}, {"eight", 8}} {
					b.Run(mode.name, func(b *testing.B) {
						b.ReportAllocs()
						b.ResetTimer()
						for range b.N {
							count := int64(0)
							result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: mode.workers}, func(batch *store.Batch) bool {
								for _, n := range batch.VectorAt(countColumn).I64s() {
									count += n
								}
								return true
							})
							if result.Code() != ExecutionCompleted || count != rows {
								b.Fatalf("grouped query %v/%v count %d", result.Code(), result.Err(), count)
							}
						}
						b.ReportMetric(float64(rows)*float64(b.N)/b.Elapsed().Seconds(), "rows/s")
					})
				}
			})
		}
	}
}

func BenchmarkParallelParquetGrouping(b *testing.B) {
	expected := pairedScanData(b)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	names := []string{"id", "seq", "measure", "score", "active", "day", "observed", "category", "message"}
	nullable := make([]bool, len(names))
	for i := range nullable {
		nullable[i] = true
	}
	scan := MakeParquetScan("../../testdata/paired/scan_snappy.parquet", MakeSchemaWithNullability(names, pairedScanTypes(), nullable), parquet.ParquetOptions{})
	for _, setup := range []struct {
		name       string
		key        string
		aggregates []Aggregate
	}{
		{"compact-boolean", "active", []Aggregate{MakeCountStar(), MakeCount(MakeColumn("message"))}},
		{"general-category", "category", []Aggregate{MakeCountStar(), MakeSum(MakeColumn("id"))}},
		{"compact-unique", "id", []Aggregate{MakeCountStar()}},
		{"general-message", "message", []Aggregate{MakeCountStar(), MakeMin(MakeColumn("score"))}},
	} {
		p, err := MakePhysicalPlan(scan.GroupBy([]Expr{MakeColumn(setup.key)}, setup.aggregates...))
		if err != nil {
			b.Fatal(err)
		}
		for _, mode := range []struct {
			name    string
			workers int
		}{{"serial", 1}, {"auto", 0}, {"four", 4}} {
			b.Run(setup.name+"/"+mode.name, func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					count := int64(0)
					result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: mode.workers}, func(batch *store.Batch) bool {
						for _, n := range batch.VectorAt(1).I64s() {
							count += n
						}
						return true
					})
					if result.Code() != ExecutionCompleted || count != int64(expected.Rows) {
						b.Fatalf("grouped file %v/%v count %d", result.Code(), result.Err(), count)
					}
				}
				b.ReportMetric(float64(expected.Rows)*float64(b.N)/b.Elapsed().Seconds(), "rows/s")
			})
		}
		p.Release()
	}
}
