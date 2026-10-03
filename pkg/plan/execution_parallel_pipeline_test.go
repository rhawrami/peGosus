package plan

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func pipelineRows(a *mem.Allocator, target *[][]Scalar) func(*store.Batch) bool {
	return func(batch *store.Batch) bool {
		mask := batch.Selection().RetainBitMap(a)
		defer mask.Release()
		for row := range batch.Len() {
			if mask != nil && !mask.IsSet(row) {
				continue
			}
			values := make([]Scalar, batch.NVectors())
			for column := range values {
				values[column] = scalarAt(batch.VectorAt(column), row)
			}
			*target = append(*target, values)
		}
		return true
	}
}

func TestParallelMultipleJoinTopNAndAggregate(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	for _, first := range []JoinKind{JoinInner, JoinLeft, JoinFull, JoinSemi, JoinAnti} {
		for _, second := range []JoinKind{JoinInner, JoinLeft, JoinFull, JoinSemi, JoinAnti} {
			t.Run(fmt.Sprintf("first=%d/second=%d", first, second), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				left, ls, _ := makeJoinBatchTestSource(a, 5, 23, true)
				right, rs, _ := makeJoinBatchTestSource(a, 3, 29, false)
				third, ts, _ := makeJoinBatchTestSource(a, 2, 19, true)
				joined := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), first,
					[]Expr{MakeColumn("l.k"), MakeColumn("l.text"), MakeColumn("l.flag")},
					[]Expr{MakeColumn("r.k"), MakeColumn("r.text"), MakeColumn("r.flag")}, MakeColumn("l.id").Lt(MakeColumn("r.id")))
				joined = joined.Join(MakeScan(third, ts).As("t"), second,
					[]Expr{MakeColumn("l.k"), MakeColumn("l.text"), MakeColumn("l.flag")},
					[]Expr{MakeColumn("t.k"), MakeColumn("t.text"), MakeColumn("t.flag")}, MakeColumn("l.id").Gt(MakeColumn("t.id")))
				columns := []Expr{MakeColumn("l.id"), MakeColumn("l.k"), MakeColumn("l.flag")}
				if first != JoinSemi && first != JoinAnti {
					columns = append(columns, MakeColumn("r.id"))
				}
				if second != JoinSemi && second != JoinAnti {
					columns = append(columns, MakeColumn("t.id"))
				}
				for mode, query := range []LogicalPlan{
					joined.Project(columns...).OrderBy(MakeOrderKey(MakeLiteral(int64(0)))).Limit(157, 11),
					joined.Project(columns...).OrderBy(MakeOrderKey(MakeColumn("l.k")).Desc().NullsFirst(), MakeOrderKey(MakeLiteral(int64(0)))).Limit(83),
					joined.Aggregate(MakeCountStar(), MakeSum(MakeColumn("l.id"))),
				} {
					p, err := MakePhysicalPlan(query)
					if err != nil {
						t.Fatal(err)
					}
					var want [][]Scalar
					result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 1}, pipelineRows(a, &want))
					if result.Code() != ExecutionCompleted {
						t.Fatalf("serial mode=%d %v/%v", mode, result.Code(), result.Err())
					}
					for _, workers := range []int{2, 4} {
						var got [][]Scalar
						result, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: workers}, pipelineRows(a, &got))
						if !used || result.Code() != ExecutionCompleted {
							t.Fatalf("mode=%d workers=%d parallel=%t %v/%v", mode, workers, used, result.Code(), result.Err())
						}
						if len(got) != len(want) {
							t.Fatalf("mode=%d rows=%d want=%d", mode, len(got), len(want))
						}
						for row := range want {
							if !slices.Equal(got[row], want[row]) {
								t.Fatalf("mode=%d workers=%d row=%d got=%v want=%v", mode, workers, row, got[row], want[row])
							}
						}
					}
					p.Release()
				}
				left.Release()
				right.Release()
				third.Release()
				if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("leaked %+v", usage)
				}
			})
		}
	}
}

func TestParallelJoinTopNLargeFanoutStableTies(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	makeSource := func(parts, rows int) (*store.Table, Schema) {
		batches := make([]*store.Batch, parts)
		for part := range parts {
			key := store.MakeVector(a, rows, dtype.Int32T(), false)
			id := store.MakeVector(a, rows, dtype.Int64T(), false)
			for row := range rows {
				key.I32s()[row] = 1
				id.I64s()[row] = int64(part*rows + row)
			}
			batches[part] = store.MakeBatch([]store.Vector{key, id})
		}
		table := store.MakeTable(batches)
		for _, batch := range batches {
			batch.Release()
		}
		return table, MakeSchema([]string{"key", "id"}, []dtype.Type{dtype.Int32T(), dtype.Int64T()})
	}
	left, ls := makeSource(7, 17)
	right, rs := makeSource(2, 701)
	joined := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), JoinFull, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")})
	p, err := MakePhysicalPlan(joined.OrderBy(MakeOrderKey(MakeLiteral(int64(1)))).Limit(5000, 1023))
	if err != nil {
		t.Fatal(err)
	}
	var want [][]Scalar
	result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 1}, pipelineRows(a, &want))
	if result.Code() != ExecutionCompleted {
		t.Fatal(result.Code(), result.Err())
	}
	for round := range 3 {
		var got [][]Scalar
		result, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 4}, pipelineRows(a, &got))
		if !used || result.Code() != ExecutionCompleted || len(got) != 5000 {
			t.Fatalf("round=%d parallel=%t %v/%v rows=%d", round, used, result.Code(), result.Err(), len(got))
		}
		for row := range want {
			if !slices.Equal(got[row], want[row]) {
				t.Fatalf("unstable round=%d row=%d got=%v want=%v", round, row, got[row], want[row])
			}
		}
	}
	p.Release()
	left.Release()
	right.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestParallelMultipleFullJoinTailsProbeLaterBuilds(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	makeSource := func(keys []int32) (*store.Table, Schema) {
		batches := make([]*store.Batch, len(keys))
		for part, value := range keys {
			key := store.MakeVector(a, 1, dtype.Int32T(), false)
			key.I32s()[0] = value
			batches[part] = store.MakeBatch([]store.Vector{key})
		}
		table := store.MakeTable(batches)
		for _, batch := range batches {
			batch.Release()
		}
		return table, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int32T()})
	}
	left, ls := makeSource([]int32{1, 1})
	right, rs := makeSource([]int32{2, 2})
	third, ts := makeSource([]int32{2, 3})
	fourth, fs := makeSource([]int32{3, 4})
	query := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), JoinFull, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")})
	query = query.Join(MakeScan(third, ts).As("t"), JoinFull, []Expr{MakeColumn("r.key")}, []Expr{MakeColumn("t.key")})
	query = query.Join(MakeScan(fourth, fs).As("u"), JoinFull, []Expr{MakeColumn("t.key")}, []Expr{MakeColumn("u.key")})
	for _, query := range []LogicalPlan{query.OrderBy(MakeOrderKey(MakeLiteral(int64(0)))).Limit(100), query.Aggregate(MakeCountStar())} {
		p, err := MakePhysicalPlan(query)
		if err != nil {
			t.Fatal(err)
		}
		var want, got [][]Scalar
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 1}, pipelineRows(a, &want))
		if result.Code() != ExecutionCompleted {
			t.Fatal(result.Code(), result.Err())
		}
		result, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, pipelineRows(a, &got))
		if !used || result.Code() != ExecutionCompleted || len(got) != len(want) {
			t.Fatalf("parallel=%t %v/%v rows=%v want=%v", used, result.Code(), result.Err(), got, want)
		}
		for row := range want {
			if !slices.Equal(got[row], want[row]) {
				t.Fatalf("row=%d got=%v want=%v", row, got[row], want[row])
			}
		}
		p.Release()
	}
	left.Release()
	right.Release()
	third.Release()
	fourth.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestParallelMultipleJoinStopCancellationAndRetainedOutput(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	left, ls, _ := makeJoinBatchTestSource(a, 4, 17, false)
	right, rs, _ := makeJoinBatchTestSource(a, 2, 19, true)
	third, ts, _ := makeJoinBatchTestSource(a, 2, 23, false)
	query := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), JoinFull, []Expr{MakeColumn("l.k")}, []Expr{MakeColumn("r.k")})
	query = query.Join(MakeScan(third, ts).As("t"), JoinFull, []Expr{MakeColumn("r.k")}, []Expr{MakeColumn("t.k")})
	p, err := MakePhysicalPlan(query)
	if err != nil {
		t.Fatal(err)
	}
	var retained *store.Batch
	var before [][]Scalar
	result, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(batch *store.Batch) bool {
		retained = batch.Retain()
		pipelineRows(a, &before)(batch)
		return false
	})
	if !used || result.Code() != ExecutionStopped || retained == nil {
		t.Fatalf("parallel=%t %v/%v", used, result.Code(), result.Err())
	}
	ctx, cancel := context.WithCancel(context.Background())
	result, used = p.executeParallel(ctx, a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 4}, func(*store.Batch) bool { cancel(); return true })
	cancel()
	if !used || result.Code() != ExecutionCancelled {
		t.Fatalf("cancel parallel=%t %v/%v", used, result.Code(), result.Err())
	}
	p.Release()
	left.Release()
	right.Release()
	third.Release()
	var after [][]Scalar
	pipelineRows(a, &after)(retained)
	if len(after) != len(before) {
		t.Fatal("retained output changed length")
	}
	for row := range before {
		if !slices.Equal(before[row], after[row]) {
			t.Fatal("retained output changed")
		}
	}
	retained.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestParallelJoinSequenceRangeAdmission(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	vector := store.MakeVector(a, 2, dtype.Int64T(), false)
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch, batch})
	batch.Release()
	p := PhysicalPlan{source: &scanSource{table: table}, steps: []physicalStep{
		{operation: physicalJoin, join: &physicalJoinSpec{kind: JoinFull}},
		{operation: physicalJoin, join: &physicalJoinSpec{kind: JoinFull}},
	}}
	builds := []*joinState{{length: 3}, {length: 5}}
	tasks, tails, ok := p.parallelJoinSequences(2, nil, []int{0, 1}, builds)
	if !ok || !slices.Equal(tasks, []uint64{0, 30}) || !slices.Equal(tails, []uint64{60, 75}) {
		t.Fatalf("ranges tasks=%v tails=%v admitted=%t", tasks, tails, ok)
	}
	builds[0].length, builds[1].length = int(^uint(0)>>1), int(^uint(0)>>1)
	if _, _, ok := p.parallelJoinSequences(2, nil, []int{0, 1}, builds); ok {
		t.Fatal("admitted overflowing fanout ranges")
	}
	table.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}
