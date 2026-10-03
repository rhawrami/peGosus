package plan

import (
	"context"
	"fmt"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkSpilledSort(b *testing.B) {
	const rows = 16384
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	vector := store.MakeVector(a, rows, dtype.Int64T(), false)
	for row := range rows {
		vector.I64s()[row] = int64(rows - row)
	}
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	defer table.Release()
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"id"}, []dtype.Type{dtype.Int64T()})).OrderBy(MakeOrderKey(MakeColumn("id"))))
	if err != nil {
		b.Fatal(err)
	}
	defer p.Release()
	for _, budget := range []int64{128 << 10, 1 << 20} {
		b.Run(fmt.Sprintf("budget=%d", budget), func(b *testing.B) {
			directory := b.TempDir()
			var peak int64
			b.ResetTimer()
			for range b.N {
				scope := mem.MakeAllocationScope(a, budget)
				seen := int64(0)
				result := p.executeWithScope(context.Background(), a, ExecutionOptions{MemoryBudget: budget, Workers: 1, SpillDirectory: directory}, func(batch *store.Batch) bool {
					for _, value := range batch.VectorAt(0).I64s() {
						seen++
						if value != seen {
							b.Fatal("incorrect sort")
						}
					}
					return true
				}, scope)
				if result.Code() != ExecutionCompleted || seen != rows || scope.Live() != 0 {
					b.Fatal(result.Code(), result.Err(), seen, scope.Live())
				}
				peak = max(peak, scope.Peak())
			}
			b.StopTimer()
			b.ReportMetric(float64(peak), "peak-live-bytes")
			b.ReportMetric(rows, "rows/op")
		})
	}
}
