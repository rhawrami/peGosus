package plan

import (
	"context"
	"crypto/sha256"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParallelJoinReferenceOwnershipAndFullMatches(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	for _, kind := range []JoinKind{JoinInner, JoinLeft, JoinFull, JoinSemi, JoinAnti} {
		for _, residual := range []bool{false, true} {
			t.Run(fmt.Sprintf("kind=%d/residual=%t", kind, residual), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				left, ls, lrows := makeJoinBatchTestSource(a, 8, 129, true)
				right, rs, rrows := makeJoinBatchTestSource(a, 5, 73, false)
				var predicates []Expr
				if residual {
					predicates = []Expr{MakeColumn("l.id").Lt(MakeColumn("r.id"))}
				}
				p, err := MakePhysicalPlan(MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), kind, []Expr{MakeColumn("l.k"), MakeColumn("l.text"), MakeColumn("l.flag")}, []Expr{MakeColumn("r.k"), MakeColumn("r.text"), MakeColumn("r.flag")}, predicates...))
				if err != nil {
					t.Fatal(err)
				}
				var outputs []*store.Batch
				result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 4}, func(batch *store.Batch) bool {
					if batch.Len() > joinOutputBatchRows {
						t.Errorf("unbounded output %d", batch.Len())
					}
					outputs = append(outputs, batch.Retain())
					return true
				})
				p.Release()
				left.Release()
				right.Release()
				if !parallel || result.Code() != ExecutionCompleted {
					t.Fatalf("parallel=%t result=%v/%v", parallel, result.Code(), result.Err())
				}
				want := joinBatchReferencePairs(lrows, rrows, kind, residual)
				seen := make(map[string]int)
				for _, batch := range outputs {
					bitmap := batch.Selection().RetainBitMap(a)
					for row := range batch.Len() {
						if bitmap != nil && !bitmap.IsSet(row) {
							continue
						}
						lid := checkJoinBatchReferenceSide(t, batch, row, 0, lrows)
						rid := "-"
						if kind != JoinSemi && kind != JoinAnti {
							rid = checkJoinBatchReferenceSide(t, batch, row, 5, rrows)
						}
						seen[lid+"/"+rid]++
					}
					bitmap.Release()
					batch.Release()
				}
				if len(seen) != len(want) {
					t.Fatalf("pairs %d want %d", len(seen), len(want))
				}
				for pair, count := range want {
					if seen[pair] != count {
						t.Errorf("pair %s count=%d want=%d", pair, seen[pair], count)
					}
				}
				usage := a.Usage()
				if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("leaked %+v", usage)
				}
			})
		}
	}
}

func TestParallelJoinDownstreamAggregateAndGroupedSort(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	for _, kind := range []JoinKind{JoinInner, JoinFull} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
		left, ls, _ := makeJoinBatchTestSource(a, 4, 67, false)
		right, rs, _ := makeJoinBatchTestSource(a, 3, 71, true)
		joined := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), kind, []Expr{MakeColumn("l.k")}, []Expr{MakeColumn("r.k")}, MakeColumn("l.id").Lt(MakeColumn("r.id")))
		for _, query := range []LogicalPlan{
			joined.Aggregate(MakeCountStar(), MakeSum(MakeColumn("l.id")), MakeSum(MakeColumn("r.id"))),
			joined.GroupBy([]Expr{MakeColumn("l.flag")}, MakeCountStar().Alias("count")).OrderBy(MakeOrderKey(MakeColumn("count"))).Limit(1),
		} {
			p, err := MakePhysicalPlan(query)
			if err != nil {
				t.Fatal(err)
			}
			var serial, parallelRows [][]Scalar
			consume := func(target *[][]Scalar) func(*store.Batch) bool {
				return func(batch *store.Batch) bool {
					bitmap := batch.Selection().RetainBitMap(a)
					defer bitmap.Release()
					for row := range batch.Len() {
						if bitmap != nil && !bitmap.IsSet(row) {
							continue
						}
						values := make([]Scalar, batch.NVectors())
						for c := range values {
							values[c] = scalarAt(batch.VectorAt(c), row)
						}
						*target = append(*target, values)
					}
					return true
				}
			}
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 1}, consume(&serial))
			if result.Code() != ExecutionCompleted {
				t.Fatal(result.Code())
			}
			result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 4}, consume(&parallelRows))
			if !parallel || result.Code() != ExecutionCompleted {
				t.Fatalf("downstream kind=%d parallel=%t result=%v/%v", kind, parallel, result.Code(), result.Err())
			}
			// Unordered aggregate rows are compared as multisets.
			signature := func(rows [][]Scalar) map[string]int {
				result := make(map[string]int)
				for _, row := range rows {
					text := ""
					for _, value := range row {
						text += fmt.Sprintf("%d:%t:%x:%s/", value.Type().ID(), value.IsNull(), value.bits, value.text)
					}
					result[text]++
				}
				return result
			}
			expected, actual := signature(serial), signature(parallelRows)
			if len(expected) != len(actual) {
				t.Errorf("downstream groups %d want %d", len(actual), len(expected))
			}
			for key, count := range expected {
				if actual[key] != count {
					t.Errorf("downstream row %q count=%d want=%d", key, actual[key], count)
				}
			}
			p.Release()
		}
		left.Release()
		right.Release()
		usage := a.Usage()
		if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
			t.Fatalf("leaked %+v", usage)
		}
	}
}

func TestParallelJoinBuildRemainsImmutable(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	left, ls, _ := makeJoinBatchTestSource(a, 6, 97, true)
	right, rs, _ := makeJoinBatchTestSource(a, 5, 113, false)
	p, err := MakePhysicalPlan(MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), JoinInner, []Expr{MakeColumn("l.k"), MakeColumn("l.text")}, []Expr{MakeColumn("r.k"), MakeColumn("r.text")}))
	if err != nil {
		t.Fatal(err)
	}
	scope := mem.MakeAllocationScope(a, 128<<20)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	state := joinState{scope: scope}
	state.keys.setScope(scope)
	step := p.steps[0]
	if result := state.build(context.Background(), scoped, step.join, 128<<20); result.Code() != ExecutionCompleted {
		t.Fatal(result.Code())
	}
	snapshot := func() [32]byte {
		hash := sha256.New()
		for _, segment := range []*mem.Segment{state.rows, state.head, state.next, state.keys.index.slots, state.keys.index.controls, state.keys.keys.data, state.keys.keys.refs, state.keys.rows.values, state.keys.rows.strings} {
			if segment != nil {
				hash.Write(segment.AsBytes())
			}
		}
		var result [32]byte
		copy(result[:], hash.Sum(nil))
		return result
	}
	before := snapshot()
	var wg sync.WaitGroup
	for part := range left.NBatches() {
		wg.Add(1)
		go func(part int) {
			defer wg.Done()
			local := state
			result := local.probe(context.Background(), scoped, left.BatchAt(part), step, 128<<20, func(batch *store.Batch) ExecutionResult {
				batch.Release()
				return ExecutionResult{code: ExecutionCompleted}
			})
			if result.Code() != ExecutionCompleted {
				t.Errorf("probe %v", result.Code())
			}
		}(part)
	}
	wg.Wait()
	if after := snapshot(); after != before {
		t.Fatal("shared build index or retained rows mutated")
	}
	state.release()
	if scope.Live() != 0 {
		t.Fatalf("scope leaked %d", scope.Live())
	}
	p.Release()
	left.Release()
	right.Release()
	usage := a.Usage()
	if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestParallelJoinStopCancellationAndTightBudget(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	left, ls, _ := makeJoinBatchTestSource(a, 8, 129, false)
	right, rs, _ := makeJoinBatchTestSource(a, 4, 257, true)
	query := func(right *store.Table) LogicalPlan {
		return MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), JoinFull, []Expr{MakeColumn("l.k")}, []Expr{MakeColumn("r.k")})
	}
	p, err := MakePhysicalPlan(query(right))
	if err != nil {
		t.Fatal(err)
	}
	baseline := a.Usage()
	calls := 0
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 4}, func(*store.Batch) bool { calls++; return false })
	if !parallel || result.Code() != ExecutionStopped || calls != 1 {
		t.Fatalf("stop %t/%v calls=%d", parallel, result.Code(), calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls = 0
	result, parallel = p.executeParallel(ctx, a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 4}, func(*store.Batch) bool { calls++; cancel(); return true })
	if !parallel || result.Code() != ExecutionCancelled || calls != 1 {
		t.Fatalf("sink cancellation %t/%v calls=%d", parallel, result.Code(), calls)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	result, parallel = p.executeParallel(ctx, a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("cancelled join reached sink"); return true })
	if !parallel || result.Code() != ExecutionCancelled {
		t.Fatalf("initial cancellation %t/%v", parallel, result.Code())
	}
	usage := a.Usage()
	if usage.GeneralUsed != baseline.GeneralUsed || usage.ScratchUsed != baseline.ScratchUsed {
		t.Fatalf("temporary bytes leaked %+v baseline %+v", usage, baseline)
	}
	p.Release()
	right.Release()
	largeRight, _, _ := makeJoinBatchTestSource(a, 4, 8193, false)
	p, err = MakePhysicalPlan(query(largeRight))
	if err != nil {
		t.Fatal(err)
	}
	result, parallel = p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 2 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("over-budget join reached sink"); return true })
	if !parallel || result.Code() != ExecutionResourceExhausted {
		t.Fatalf("tight budget %t/%v", parallel, result.Code())
	}
	p.Release()
	left.Release()
	largeRight.Release()
	usage = a.Usage()
	if usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}
