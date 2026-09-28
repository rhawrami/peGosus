package plan

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/csv"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkCSVScanBatchSize(b *testing.B) {
	const rows = 65536
	input := make([]byte, 0, rows*12)
	want := 0
	for row := range rows {
		score := row % 100
		if score > 49 {
			want++
		}
		input = strconv.AppendInt(input, int64(score), 10)
		input = append(input, ',')
		input = strconv.AppendInt(input, int64(row), 10)
		input = append(input, '\n')
	}
	path := filepath.Join(b.TempDir(), "rows.csv")
	if err := os.WriteFile(path, input, 0600); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	schema := MakeSchema([]string{"score", "value"}, []dtype.Type{dtype.Int32T(), dtype.Int64T()})
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	for _, batchSize := range []int{1024, 4096, 8192, 16384} {
		b.Run(fmt.Sprintf("rows=%d", batchSize), func(b *testing.B) {
			query := MakeCSVScan(path, schema, csv.CSVOptions{BatchSize: batchSize}).Filter(MakeColumn("score").Gt(49)).Project(MakeColumn("value").Add(1))
			plan, err := MakePhysicalPlan(query)
			if err != nil {
				b.Fatal(err)
			}
			defer plan.Release()
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			for range b.N {
				count := 0
				result := plan.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: math.MaxInt64}, func(batch *store.Batch) bool {
					count += batch.ActiveLen()
					return true
				})
				if result.Code() != ExecutionCompleted || count != want {
					b.Fatalf("CSV scan result %v/%v, got %d rows, want %d", result.Code(), result.Err(), count, want)
				}
			}
		})
	}
}
