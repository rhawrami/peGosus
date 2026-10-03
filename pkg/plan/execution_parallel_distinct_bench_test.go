package plan

import (
	"context"
	"fmt"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkParallelDistinct(b *testing.B) {
	const rows = 1 << 18
	for _, cardinality := range []int{64, rows} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
		batches := make([]*store.Batch, rows/4096)
		for chunk := range batches {
			keys := store.MakeVector(a, 4096, dtype.Int64T(), false)
			texts := make([][]byte, 4096)
			for row := range texts {
				id := (chunk*4096 + row) % cardinality
				keys.I64s()[row] = int64(id)
				texts[row] = []byte(fmt.Sprintf("long compound distinct key %08d", id))
			}
			batches[chunk] = store.MakeBatch([]store.Vector{keys, store.MakeStringVector(a, texts, nil)})
		}
		table := store.MakeTable(batches)
		for _, batch := range batches {
			batch.Release()
		}
		scan := MakeScan(table, MakeSchemaWithNullability([]string{"key", "text"}, []dtype.Type{dtype.Int64T(), dtype.StringT()}, []bool{false, false}))
		for _, mode := range []string{"compound", "count-string"} {
			query := scan.Distinct()
			if mode == "count-string" {
				query = scan.Aggregate(MakeCountDistinct(MakeColumn("text")))
			}
			p, err := MakePhysicalPlan(query)
			if err != nil {
				b.Fatal(err)
			}
			for _, workers := range []int{1, 4} {
				b.Run(fmt.Sprintf("groups=%d/%s/workers=%d", cardinality, mode, workers), func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						count := int64(0)
						result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 30, Workers: workers}, func(batch *store.Batch) bool {
							if mode == "compound" {
								count += int64(batch.Len())
							} else {
								count += batch.VectorAt(0).I64s()[0]
							}
							return true
						})
						if result.Code() != ExecutionCompleted || count != int64(cardinality) {
							b.Fatalf("result %v/%v count=%d", result.Code(), result.Err(), count)
						}
					}
					b.ReportMetric(float64(rows)*float64(b.N)/b.Elapsed().Seconds(), "rows/s")
				})
			}
			p.Release()
		}
		table.Release()
	}
}
