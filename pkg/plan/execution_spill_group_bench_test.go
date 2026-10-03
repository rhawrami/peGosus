package plan

import (
	"context"
	"fmt"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkSpilledGroup(b *testing.B) {
	for _, global := range []bool{false, true} {
		b.Run(fmt.Sprintf("global=%t", global), func(b *testing.B) {
			const rows = 8192
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			keys := store.MakeVector(a, rows, dtype.Int64T(), false)
			nums := store.MakeVector(a, rows, dtype.Int64T(), false)
			texts := make([][]byte, rows)
			for row := range rows {
				keys.I64s()[row] = int64(row % 2048)
				nums.I64s()[row] = int64(row)
				texts[row] = []byte(fmt.Sprintf("unique\x00 allocator-backed string %05d", row))
			}
			batch := store.MakeBatch([]store.Vector{keys, nums, store.MakeStringVector(a, texts, nil)})
			table := store.MakeTable([]*store.Batch{batch})
			batch.Release()
			schema := MakeSchema([]string{"g", "x", "s"}, []dtype.Type{dtype.Int64T(), dtype.Int64T(), dtype.StringT()})
			scan := MakeScan(table, schema)
			aggregates := []Aggregate{MakeCountStar(), MakeSum(MakeColumn("x")), MakeCountDistinct(MakeColumn("x")), MakeSum(MakeColumn("x")).Distinct(), MakeCountDistinct(MakeColumn("s"))}
			logical := scan.GroupBy([]Expr{MakeColumn("g")}, aggregates...)
			expected := 2048
			if global {
				logical = scan.Aggregate(aggregates...)
				expected = 1
			}
			p, err := MakePhysicalPlan(logical)
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				p.Release()
				table.Release()
				if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					b.Fatal("payload leaked", usage)
				}
			}()
			options := ExecutionOptions{MemoryBudget: 192 << 10, Workers: 1, SpillDirectory: b.TempDir()}
			b.ReportAllocs()
			b.SetBytes(rows * 52)
			var peak int64
			b.ResetTimer()
			for range b.N {
				scope := mem.MakeAllocationScope(a, options.MemoryBudget)
				count := 0
				result := p.executeWithScope(context.Background(), a, options, func(batch *store.Batch) bool { count += batch.ActiveLen(); return true }, scope)
				if result.Code() != ExecutionCompleted || count != expected || scope.Live() != 0 {
					b.Fatal(result.Code(), result.Err(), count, scope.Live())
				}
				peak = max(peak, scope.Peak())
			}
			b.StopTimer()
			b.ReportMetric(float64(peak), "scope_peak_B")
			b.ReportMetric(float64(expected), "output_rows/op")
			b.ReportMetric(float64(options.MemoryBudget), "budget_B")
		})
	}
}
