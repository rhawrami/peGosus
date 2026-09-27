package plan

import (
	"context"
	"errors"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalHashJoinsAndQualifiedNames(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	var leftBatches []*store.Batch
	for _, keys := range [][]int32{{1, 2}, {9}} {
		v := store.MakeVector(a, len(keys), dtype.Int32T(), true)
		copy(v.I32s(), keys)
		if len(keys) == 1 {
			v.Validity().Clear(0)
		}
		leftBatches = append(leftBatches, store.MakeBatch([]store.Vector{v}))
	}
	leftTable := store.MakeTable(leftBatches)
	for _, b := range leftBatches {
		b.Release()
	}
	rightKey := store.MakeVector(a, 3, dtype.Int32T(), false)
	copy(rightKey.I32s(), []int32{2, 2, 3})
	rightValue := store.MakeStringVector(a, [][]byte{[]byte("small"), []byte("long matching string value"), []byte("other unmatched string value")}, nil)
	rightScore := store.MakeVector(a, 3, dtype.Int32T(), false)
	copy(rightScore.I32s(), []int32{5, 20, 30})
	rightBatch := store.MakeBatch([]store.Vector{rightKey, rightValue, rightScore})
	rightTable := store.MakeTable([]*store.Batch{rightBatch})
	rightBatch.Release()
	left := MakeScan(leftTable, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int32T()})).As("l")
	right := MakeScan(rightTable, MakeSchema([]string{"key", "value", "score"}, []dtype.Type{dtype.Int32T(), dtype.StringT(), dtype.Int32T()})).As("r")
	for _, test := range []struct {
		kind JoinKind
		want int
	}{{JoinInner, 1}, {JoinLeft, 3}, {JoinFull, 5}, {JoinSemi, 1}, {JoinAnti, 2}} {
		query := left.Join(right, test.kind, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("r.key")}, MakeColumn("r.score").Gt(10))
		physical, err := MakePhysicalPlan(query)
		if err != nil {
			t.Fatalf("join %d: %v", test.kind, err)
		}
		var output []*store.Batch
		result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(batch *store.Batch) bool {
			output = append(output, batch.Retain())
			return true
		})
		physical.Release()
		if result.Code() != ExecutionCompleted {
			t.Fatalf("join %d: result %v", test.kind, result.Code())
		}
		count := 0
		for _, b := range output {
			count += b.ActiveLen()
			if test.kind == JoinInner && b.VectorAt(2).Strings()[0].View() != "long matching string value" {
				t.Fatal("join string payload lost")
			}
			b.Release()
		}
		if count != test.want {
			t.Fatalf("join %d: got %d rows, want %d", test.kind, count, test.want)
		}
	}
	if _, err := MakePhysicalPlan(left.Join(right, JoinInner, []Expr{MakeColumn("key")}, []Expr{MakeColumn("key")}, MakeColumn("key").Gt(1))); err == nil {
		t.Fatal("ambiguous unqualified join residual bound")
	} else {
		var e *PlanError
		if !errors.As(err, &e) || e.Code() != ErrorAmbiguousColumn {
			t.Fatalf("got %v, want ambiguous column", err)
		}
	}
	if _, err := MakePhysicalPlan(left.Join(right, JoinInner, []Expr{MakeColumn("l.missing")}, []Expr{MakeColumn("r.key")})); err == nil {
		t.Fatal("missing qualified join key bound")
	} else {
		var e *PlanError
		if !errors.As(err, &e) || e.Code() != ErrorMissingColumn {
			t.Fatalf("got %v, want missing column", err)
		}
	}
	if _, err := MakePhysicalPlan(left.Join(right.As("l"), JoinInner, []Expr{MakeColumn("l.key")}, []Expr{MakeColumn("l.key")})); err == nil {
		t.Fatal("join sources with identical aliases bound")
	} else {
		var e *PlanError
		if !errors.As(err, &e) || e.Code() != ErrorAmbiguousColumn {
			t.Fatalf("got %v, want ambiguous column", err)
		}
	}
	leftTable.Release()
	rightTable.Release()
}

func TestPhysicalJoinEmptyAndNumericCoercion(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	leftTable := store.MakeEmptyTable([]dtype.Type{dtype.Int32T()})
	rightVector := store.MakeVector(a, 2, dtype.Int64T(), false)
	copy(rightVector.I64s(), []int64{1, 2})
	rightBatch := store.MakeBatch([]store.Vector{rightVector})
	rightTable := store.MakeTable([]*store.Batch{rightBatch})
	rightBatch.Release()
	left := MakeScan(leftTable, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int32T()})).As("left")
	right := MakeScan(rightTable, MakeSchema([]string{"key"}, []dtype.Type{dtype.Int64T()})).As("right")
	physical, err := MakePhysicalPlan(left.Join(right, JoinFull, []Expr{MakeColumn("left.key")}, []Expr{MakeColumn("right.key")}))
	if err != nil {
		t.Fatal(err)
	}
	var output *store.Batch
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 4096}, func(b *store.Batch) bool { output = b.Retain(); return true })
	physical.Release()
	leftTable.Release()
	rightTable.Release()
	if result.Code() != ExecutionCompleted || output == nil || output.Len() != 2 {
		t.Fatalf("empty left FULL join: result %v", result.Code())
	}
	defer output.Release()
	for i, want := range []int64{1, 2} {
		if output.VectorAt(0).Validity() == nil || output.VectorAt(0).Validity().IsSet(i) || output.VectorAt(1).I64s()[i] != want {
			t.Fatalf("unmatched row %d wrong", i)
		}
	}
}

func TestPhysicalJoinFanoutBudgetAndCancellation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	leftVector := store.MakeVector(a, 1, dtype.Int32T(), false)
	leftVector.I32s()[0] = 1
	leftBatch := store.MakeBatch([]store.Vector{leftVector})
	leftTable := store.MakeTable([]*store.Batch{leftBatch})
	leftBatch.Release()
	rightVector := store.MakeVector(a, 20, dtype.Int32T(), false)
	for i := range rightVector.I32s() {
		rightVector.I32s()[i] = 1
	}
	rightBatch := store.MakeBatch([]store.Vector{rightVector})
	rightTable := store.MakeTable([]*store.Batch{rightBatch})
	rightBatch.Release()
	physical, err := MakePhysicalPlan(MakeScan(leftTable, MakeSchema([]string{"k"}, []dtype.Type{dtype.Int32T()})).As("l").Join(
		MakeScan(rightTable, MakeSchema([]string{"k"}, []dtype.Type{dtype.Int32T()})).As("r"), JoinInner,
		[]Expr{MakeColumn("l.k")}, []Expr{MakeColumn("r.k")},
	))
	if err != nil {
		t.Fatal(err)
	}
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 2500}, func(*store.Batch) bool { t.Fatal("over-budget join reached sink"); return true })
	if result.Code() != ExecutionResourceExhausted {
		t.Fatalf("fanout budget result: %v", result.Code())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = physical.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: 8192}, func(*store.Batch) bool { t.Fatal("cancelled join reached sink"); return true })
	if result.Code() != ExecutionCancelled || !errors.Is(result.Err(), context.Canceled) {
		t.Fatalf("cancelled join result: %v", result.Code())
	}
	physical.Release()
	leftTable.Release()
	rightTable.Release()
}
