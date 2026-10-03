package plan

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func BenchmarkParallelChainedJoinTopN(b *testing.B) {
	const rows = 131072
	const keys = 4096
	const budget = 128 << 20
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	schema := MakeSchema([]string{"key", "id", "score"}, []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Int64T()})
	makeSource := func(length, batchRows int, build bool) *store.Table {
		batches := make([]*store.Batch, 0, (length+batchRows-1)/batchRows)
		for start := 0; start < length; start += batchRows {
			count := min(batchRows, length-start)
			key := store.MakeVector(a, count, dtype.Int32T(), false)
			id := store.MakeVector(a, count, dtype.Int64T(), false)
			score := store.MakeVector(a, count, dtype.Int64T(), false)
			for row := range count {
				ordinal := start + row
				value, duplicate := ordinal%keys, 0
				if build {
					value, duplicate = ordinal/2, ordinal%2
				}
				key.I32s()[row] = int32(value)
				id.I64s()[row] = int64(ordinal)
				score.I64s()[row] = int64(value%16*2 + duplicate)
			}
			batches = append(batches, store.MakeBatch([]store.Vector{key, id, score}))
		}
		table := store.MakeTable(batches)
		for _, batch := range batches {
			batch.Release()
		}
		return table
	}
	left := makeSource(rows, 8192, false)
	right := makeSource(keys*2, 2048, true)
	third := makeSource(keys*2, 2048, true)
	defer left.Release()
	defer right.Release()
	defer third.Release()
	query := MakeScan(left, schema).As("l").Join(MakeScan(right, schema).As("r"), JoinInner, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")})
	query = query.Join(MakeScan(third, schema).As("t"), JoinInner, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("t.key")})
	query = query.OrderBy(MakeOrderKey(MakeColumn("r.score")).Desc(), MakeOrderKey(MakeColumn("t.score"))).Limit(50, 17)
	p, err := MakePhysicalPlan(query)
	if err != nil {
		b.Fatal(err)
	}
	defer p.Release()
	var expected [][]Scalar
	result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: budget, Workers: 1}, pipelineRows(a, &expected))
	if result.Code() != ExecutionCompleted || len(expected) != 50 {
		b.Fatalf("serial reference=%v/%v rows=%d", result.Code(), result.Err(), len(expected))
	}
	for _, workers := range []int{1, 4} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			if workers > 1 && runtime.GOMAXPROCS(0) < 2 {
				b.Skip("requires two workers")
			}
			var peak int64
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				scope := mem.MakeAllocationScope(a, budget)
				seen := 0
				result := p.executeWithScope(context.Background(), a, ExecutionOptions{MemoryBudget: budget, Workers: workers}, func(batch *store.Batch) bool {
					mask, bitmap := batch.Selection().AsBitMap()
					selected, offsets := batch.Selection().AsSelVec()
					cursor := 0
					for row := range batch.Len() {
						if bitmap && !mask.IsSet(row) {
							continue
						}
						if offsets {
							for cursor < len(selected.Offsets()) && int(selected.Offsets()[cursor]) < row {
								cursor++
							}
							if cursor == len(selected.Offsets()) || int(selected.Offsets()[cursor]) != row {
								continue
							}
						}
						if seen >= len(expected) || batch.NVectors() != len(expected[seen]) {
							b.Fatal("changed output dimensions")
						}
						for column := range batch.NVectors() {
							if scalarAt(batch.VectorAt(column), row) != expected[seen][column] {
								b.Fatalf("workers=%d row=%d column=%d differs from serial", workers, seen, column)
							}
						}
						seen++
					}
					return true
				}, scope)
				if result.Code() != ExecutionCompleted || seen != len(expected) || scope.Live() != 0 || scope.Peak() <= 0 || scope.Peak() > budget {
					b.Fatalf("workers=%d result=%v/%v rows=%d live=%d peak=%d", workers, result.Code(), result.Err(), seen, scope.Live(), scope.Peak())
				}
				peak = max(peak, scope.Peak())
			}
			b.StopTimer()
			b.ReportMetric(rows, "input-rows")
			b.ReportMetric(rows*4, "joined-rows")
			b.ReportMetric(float64(peak), "peak-query-bytes")
			b.ReportMetric(budget, "budget-bytes")
		})
	}
}
