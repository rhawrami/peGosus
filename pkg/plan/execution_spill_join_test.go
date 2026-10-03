package plan

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestSpillJoinCompoundResidualReference(t *testing.T) {
	for _, kind := range []JoinKind{JoinInner, JoinLeft, JoinFull, JoinSemi, JoinAnti} {
		for _, residual := range []bool{false, true} {
			t.Run(fmt.Sprintf("kind=%d/residual=%t", kind, residual), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				left, ls, lrows := makeJoinBatchTestSource(a, 4, 31, true)
				right, rs, rrows := makeJoinBatchTestSource(a, 5, 43, false)
				var predicates []Expr
				if residual {
					predicates = []Expr{MakeColumn("l.id").Lt(MakeColumn("r.id"))}
				}
				p, err := MakePhysicalPlan(MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), kind, []Expr{MakeColumn("l.k"), MakeColumn("l.text"), MakeColumn("l.flag")}, []Expr{MakeColumn("r.k"), MakeColumn("r.text"), MakeColumn("r.flag")}, predicates...))
				if err != nil {
					t.Fatal(err)
				}
				directory := t.TempDir()
				scope := mem.MakeAllocationScope(a, 128<<10)
				seen := make(map[string]int)
				var retained *store.Batch
				result, used := p.executeSpilledJoin(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: directory, Workers: 4}, func(batch *store.Batch) bool {
					if batch.Len() > joinOutputBatchRows {
						t.Errorf("unbounded output=%d", batch.Len())
					}
					if retained == nil {
						retained = batch.Retain()
					}
					mask := batch.Selection().RetainBitMap(a)
					defer mask.Release()
					for row := range batch.Len() {
						if mask != nil && !mask.IsSet(row) {
							continue
						}
						lid := checkJoinBatchReferenceSide(t, batch, row, 0, lrows)
						rid := "-"
						if kind != JoinSemi && kind != JoinAnti {
							rid = checkJoinBatchReferenceSide(t, batch, row, 5, rrows)
						}
						seen[lid+"/"+rid]++
					}
					return true
				}, scope)
				if !used || result.Code() != ExecutionCompleted {
					t.Fatalf("eligible=%t %v/%v live=%d", used, result.Code(), result.Err(), scope.Live())
				}
				want := joinBatchReferencePairs(lrows, rrows, kind, residual)
				if len(seen) != len(want) {
					t.Fatalf("pairs=%d want=%d", len(seen), len(want))
				}
				for pair, count := range want {
					if seen[pair] != count {
						t.Fatalf("pair=%s count=%d want=%d", pair, seen[pair], count)
					}
				}
				p.Release()
				left.Release()
				right.Release()
				if retained != nil {
					for row := range retained.Len() {
						checkJoinBatchReferenceSide(t, retained, row, 0, lrows)
					}
					retained.Release()
				}
				if scope.Live() != 0 {
					t.Fatalf("live=%d", scope.Live())
				}
				if files, err := os.ReadDir(directory); err != nil || len(files) != 0 {
					t.Fatalf("temporary files=%v err=%v", files, err)
				}
				if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
					t.Fatalf("leaked %+v", usage)
				}
			})
		}
	}
}

func makeSpillHotJoinSource(a *mem.Allocator, rows int, first int64) (*store.Table, Schema) {
	key := store.MakeVector(a, rows, dtype.Int32T(), false)
	id := store.MakeVector(a, rows, dtype.Int64T(), false)
	texts := make([][]byte, rows)
	for row := range rows {
		key.I32s()[row] = 1
		id.I64s()[row] = first + int64(row)
		texts[row] = []byte(strings.Repeat("binary \x00\xff", 64))
	}
	text := store.MakeStringVector(a, texts, nil)
	batch := store.MakeBatch([]store.Vector{key, id, text})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	return table, MakeSchema([]string{"key", "id", "text"}, []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.StringT()})
}

func TestSpillJoinHotKeyBoundedChunksAndAggregate(t *testing.T) {
	for _, kind := range []JoinKind{JoinInner, JoinLeft, JoinFull, JoinSemi, JoinAnti} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			left, ls := makeSpillHotJoinSource(a, 7, 0)
			right, rs := makeSpillHotJoinSource(a, 400, 0)
			joined := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), kind, []Expr{MakeColumn("l.key"), MakeColumn("l.text")}, []Expr{MakeColumn("r.key"), MakeColumn("r.text")}, MakeColumn("l.id").Lt(MakeColumn("r.id")))
			p, err := MakePhysicalPlan(joined)
			if err != nil {
				t.Fatal(err)
			}
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, Workers: 1}, func(*store.Batch) bool { t.Error("in-memory oversized build reached sink"); return true })
			if result.Code() != ExecutionResourceExhausted {
				t.Fatalf("in-memory code=%v/%v", result.Code(), result.Err())
			}
			p.Release()
			p, err = MakePhysicalPlan(joined.Aggregate(MakeCountStar(), MakeSum(MakeColumn("l.id"))))
			if err != nil {
				t.Fatal(err)
			}
			var got [][]Scalar
			directory := t.TempDir()
			result, used := p.executeSpilledJoin(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: directory, Workers: 1}, pipelineRows(a, &got), nil)
			if !used || result.Code() != ExecutionCompleted {
				t.Fatalf("spill eligible=%t %v/%v", used, result.Code(), result.Err())
			}
			var count, sum int64
			for l := int64(0); l < 7; l++ {
				matches := int64(399) - l
				if kind == JoinSemi {
					matches = 1
				} else if kind == JoinAnti {
					matches = 0
				}
				count += matches
				sum += l * matches
			}
			if kind == JoinFull {
				count++
			}
			if len(got) != 1 || got[0][0].i64() != count || count != 0 && got[0][1].i64() != sum || count == 0 && !got[0][1].IsNull() {
				t.Fatalf("got=%v want count=%d sum=%d", got, count, sum)
			}
			p.Release()
			left.Release()
			right.Release()
			if files, err := os.ReadDir(directory); err != nil || len(files) != 0 {
				t.Fatalf("temporary files=%v err=%v", files, err)
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("leaked %+v", usage)
			}
		})
	}
}

func TestSpillJoinEmptyInputs(t *testing.T) {
	for _, kind := range []JoinKind{JoinInner, JoinLeft, JoinFull, JoinSemi, JoinAnti} {
		for _, leftEmpty := range []bool{false, true} {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			leftRows, rightRows := 3, 0
			if leftEmpty {
				leftRows, rightRows = 0, 3
			}
			left, ls := makeSpillHotJoinSource(a, leftRows, 0)
			right, rs := makeSpillHotJoinSource(a, rightRows, 0)
			p, err := MakePhysicalPlan(MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), kind, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")}))
			if err != nil {
				t.Fatal(err)
			}
			var rows int
			result, used := p.executeSpilledJoin(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: t.TempDir()}, func(batch *store.Batch) bool { rows += batch.ActiveLen(); return true }, nil)
			want := 0
			if kind == JoinFull {
				want = 3
			} else if !leftEmpty && (kind == JoinLeft || kind == JoinAnti) {
				want = 3
			}
			if !used || result.Code() != ExecutionCompleted || rows != want {
				t.Fatalf("kind=%d emptyLeft=%t eligible=%t result=%v/%v rows=%d want=%d", kind, leftEmpty, used, result.Code(), result.Err(), rows, want)
			}
			p.Release()
			left.Release()
			right.Release()
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("leaked %+v", usage)
			}
		}
	}
}

func TestSpillJoinStopCancellationBudgetAndFallback(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	left, ls, _ := makeJoinBatchTestSource(a, 4, 19, true)
	right, rs, _ := makeJoinBatchTestSource(a, 3, 31, false)
	joined := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), JoinFull, []Expr{MakeColumn("l.k")}, []Expr{MakeColumn("r.k")})
	p, err := MakePhysicalPlan(joined)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	check := func() {
		t.Helper()
		if files, err := os.ReadDir(directory); err != nil || len(files) != 0 {
			t.Fatalf("temporary files=%v err=%v", files, err)
		}
	}
	calls := 0
	result, used := p.executeSpilledJoin(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: directory}, func(*store.Batch) bool { calls++; return false }, nil)
	if !used || result.Code() != ExecutionStopped || calls != 1 {
		t.Fatalf("stop=%t %v calls=%d", used, result.Code(), calls)
	}
	check()
	ctx, cancel := context.WithCancel(context.Background())
	result, used = p.executeSpilledJoin(ctx, a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: directory}, func(*store.Batch) bool { cancel(); return true }, nil)
	cancel()
	if !used || result.Code() != ExecutionCancelled {
		t.Fatalf("cancel=%t %v/%v", used, result.Code(), result.Err())
	}
	check()
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	result, used = p.executeSpilledJoin(ctx, a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: directory}, func(*store.Batch) bool { t.Error("cancelled sink"); return true }, nil)
	if !used || result.Code() != ExecutionCancelled {
		t.Fatalf("pre-cancel=%t %v", used, result.Code())
	}
	check()
	result, used = p.executeSpilledJoin(context.Background(), a, ExecutionOptions{MemoryBudget: 1, SpillDirectory: directory}, func(*store.Batch) bool { t.Error("over-budget sink"); return true }, nil)
	if !used || result.Code() != ExecutionResourceExhausted {
		t.Fatalf("budget=%t %v/%v", used, result.Code(), result.Err())
	}
	check()
	result, used = p.executeSpilledJoin(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: directory + "/missing"}, func(*store.Batch) bool { t.Error("invalid-directory sink"); return true }, nil)
	if !used || result.Code() != ExecutionFailed {
		t.Fatalf("disk=%t %v/%v", used, result.Code(), result.Err())
	}
	check()
	p.Release()
	for _, query := range []LogicalPlan{joined.OrderBy(MakeOrderKey(MakeColumn("l.id"))).Limit(10), joined.Limit(10), joined.Join(MakeScan(right, rs).As("t"), JoinInner, []Expr{MakeColumn("l.k")}, []Expr{MakeColumn("t.k")})} {
		p, err := MakePhysicalPlan(query)
		if err != nil {
			t.Fatal(err)
		}
		if _, used := p.executeSpilledJoin(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: directory}, func(*store.Batch) bool { t.Error("unsupported sink"); return true }, nil); used {
			t.Fatal("unsupported ordered/multiple-join tail accepted")
		}
		p.Release()
		check()
	}
	left.Release()
	right.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestSpillJoinGroupedAndComputedFilterProjectTail(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	left, ls, _ := makeJoinBatchTestSource(a, 4, 19, true)
	right, rs, _ := makeJoinBatchTestSource(a, 3, 31, false)
	joined := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), JoinFull, []Expr{MakeColumn("l.k")}, []Expr{MakeColumn("r.k")})
	for _, query := range []LogicalPlan{
		joined.Filter(MakeColumn("l.id").IsNotNull()).Project(MakeColumn("l.id").Add(1).Alias("computed"), MakeColumn("l.flag").Alias("flag")).GroupBy([]Expr{MakeColumn("flag")}, MakeCountStar(), MakeSum(MakeColumn("computed"))),
		joined.Aggregate(MakeCountStar(), MakeSum(MakeColumn("l.id")), MakeMin(MakeColumn("l.k")), MakeMax(MakeColumn("l.k"))),
	} {
		p, err := MakePhysicalPlan(query)
		if err != nil {
			t.Fatal(err)
		}
		var want, got [][]Scalar
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 32 << 20, Workers: 1}, pipelineRows(a, &want))
		if result.Code() != ExecutionCompleted {
			t.Fatal(result.Code(), result.Err())
		}
		result, used := p.executeSpilledJoin(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: t.TempDir()}, pipelineRows(a, &got), nil)
		if !used || result.Code() != ExecutionCompleted {
			t.Fatalf("eligible=%t %v/%v", used, result.Code(), result.Err())
		}
		signature := func(rows [][]Scalar) map[string]int {
			out := make(map[string]int)
			for _, row := range rows {
				out[fmt.Sprint(row)]++
			}
			return out
		}
		expected, actual := signature(want), signature(got)
		if len(expected) != len(actual) {
			t.Fatalf("got=%v want=%v", got, want)
		}
		for key, count := range expected {
			if actual[key] != count {
				t.Fatalf("row=%s got=%d want=%d", key, actual[key], count)
			}
		}
		p.Release()
	}
	left.Release()
	right.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestSpillJoinPublicExecutionDispatch(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	left, ls := makeSpillHotJoinSource(a, 5, 0)
	right, rs := makeSpillHotJoinSource(a, 400, 0)
	query := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), JoinFull, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")})
	p, err := MakePhysicalPlan(query)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	rows := 0
	result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: directory, Workers: 4}, func(batch *store.Batch) bool { rows += batch.ActiveLen(); return true })
	if result.Code() != ExecutionCompleted || rows != 2000 {
		t.Fatalf("public spill=%v/%v rows=%d", result.Code(), result.Err(), rows)
	}
	p.Release()
	left.Release()
	right.Release()
	if files, err := os.ReadDir(directory); err != nil || len(files) != 0 {
		t.Fatalf("temporary files=%v err=%v", files, err)
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func BenchmarkSpillJoinHotKey(b *testing.B) {
	for _, kind := range []JoinKind{JoinInner, JoinFull, JoinSemi, JoinAnti} {
		b.Run(fmt.Sprintf("kind=%d", kind), func(b *testing.B) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			left, ls := makeSpillHotJoinSource(a, 7, 0)
			right, rs := makeSpillHotJoinSource(a, 400, 0)
			query := MakeScan(left, ls).As("l").Join(MakeScan(right, rs).As("r"), kind, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")}, MakeColumn("l.id").Lt(MakeColumn("r.id"))).Aggregate(MakeCountStar())
			p, err := MakePhysicalPlan(query)
			if err != nil {
				b.Fatal(err)
			}
			defer p.Release()
			defer left.Release()
			defer right.Release()
			directory := b.TempDir()
			b.ReportAllocs()
			b.ReportMetric(128<<10, "budget-bytes")
			b.ResetTimer()
			for range b.N {
				result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 10, SpillDirectory: directory, Workers: 1}, func(*store.Batch) bool { return true })
				if result.Code() != ExecutionCompleted {
					b.Fatal(result.Code(), result.Err())
				}
			}
		})
	}
}

func TestSpillJoinAllNullInputs(t *testing.T) {
	for _, kind := range []JoinKind{JoinInner, JoinLeft, JoinFull, JoinSemi, JoinAnti} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			makeSource := func(rows int) *store.Table {
				floating := store.MakeVector(a, rows, dtype.Float64T(), true)
				floating.Validity().ClearAll()
				text := store.MakeStringVector(a, make([][]byte, rows), make([]bool, rows))
				batch := store.MakeBatch([]store.Vector{floating, text})
				table := store.MakeTable([]*store.Batch{batch})
				batch.Release()
				return table
			}
			left, right := makeSource(3), makeSource(5)
			schema := MakeSchemaWithNullability([]string{"f", "s"}, []dtype.Type{dtype.Float64T(), dtype.StringT()}, []bool{true, true})
			query := MakeScan(left, schema).As("l").Join(MakeScan(right, schema).As("r"), kind, []Expr{MakeColumn("l.f"), MakeColumn("l.s")}, []Expr{MakeColumn("r.f"), MakeColumn("r.s")})
			p, err := MakePhysicalPlan(query)
			if err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			scope := mem.MakeAllocationScope(a, 128<<10)
			rows := 0
			var retained *store.Batch
			check := func(batch *store.Batch) {
				t.Helper()
				for column := range batch.NVectors() {
					for row := range batch.Len() {
						if !scalarAt(batch.VectorAt(column), row).IsNull() {
							t.Fatalf("column=%d row=%d is non-null", column, row)
						}
					}
				}
			}
			result, used := p.executeSpilledJoin(context.Background(), a, ExecutionOptions{MemoryBudget: scope.Limit(), SpillDirectory: directory}, func(batch *store.Batch) bool {
				check(batch)
				rows += batch.ActiveLen()
				if retained == nil {
					retained = batch.Retain()
				}
				return true
			}, scope)
			want := 0
			if kind == JoinLeft || kind == JoinAnti {
				want = 3
			} else if kind == JoinFull {
				want = 8
			}
			if !used || result.Code() != ExecutionCompleted || rows != want {
				t.Fatalf("eligible=%t code=%v/%v rows=%d want=%d", used, result.Code(), result.Err(), rows, want)
			}
			p.Release()
			left.Release()
			right.Release()
			if retained != nil {
				check(retained)
				if scope.Live() == 0 {
					t.Fatal("retained output uncharged")
				}
				retained.Release()
			}
			if scope.Live() != 0 {
				t.Fatalf("scope live=%d", scope.Live())
			}
			if files, err := os.ReadDir(directory); err != nil || len(files) != 0 {
				t.Fatalf("files=%v cause=%v", files, err)
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("leaked %+v", usage)
			}
		})
	}
}
