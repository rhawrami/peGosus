package plan

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestBatchReservationIndependentAndNestedCases(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	x := store.MakeVector(a, 512, dtype.Int64T(), true)
	for row := range x.I64s() {
		x.I64s()[row] = int64(row)
		if row%7 == 0 {
			x.Validity().Clear(row)
		}
	}
	batch := store.MakeBatch([]store.Vector{x})
	table := store.MakeTable([]*store.Batch{batch})
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()}))
	var singleReservation int64
	for _, width := range []int{1, 128} {
		aggs := make([]Aggregate, width)
		for i := range aggs {
			aggs[i] = MakeSum(MakeCase(MakeColumn("x").Gt(i), MakeColumn("x").Add(i), 0)).Alias(fmt.Sprintf("s%d", i))
		}
		p, err := MakePhysicalPlan(scan.Aggregate(aggs...))
		if err != nil {
			t.Fatal(err)
		}
		reservation := p.batchReservation(batch)
		if width == 1 {
			singleReservation = reservation
		} else if reservation != singleReservation {
			t.Fatalf("sequential program width inflated reservation: %d vs %d", reservation, singleReservation)
		}
		called := false
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 1 << 20}, func(out *store.Batch) bool {
			called = true
			for col := range width {
				var sum int64
				for row := range 512 {
					if row%7 != 0 && row > col {
						sum += int64(row + col)
					}
				}
				if out.VectorAt(col).I64s()[0] != sum {
					t.Fatalf("sum %d: got %d want %d", col, out.VectorAt(col).I64s()[0], sum)
				}
			}
			return true
		})
		if result.Code() != ExecutionCompleted || !called {
			t.Fatalf("width %d result %v", width, result.Code())
		}
		result = p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 128}, func(*store.Batch) bool { t.Fatal("tiny budget reached sink"); return true })
		if result.Code() != ExecutionResourceExhausted {
			t.Fatalf("tiny budget width %d: %v", width, result.Code())
		}
		p.Release()
	}
	expr := MakeColumn("x")
	var previous int64
	for depth := range 5 {
		expr = MakeCase(MakeColumn("x").Gt(0), expr, 0)
		p, err := MakePhysicalPlan(scan.Aggregate(MakeSum(expr)))
		if err != nil {
			t.Fatal(err)
		}
		got := p.batchReservation(batch)
		if got <= previous {
			t.Fatalf("depth %d reservation failed to grow: %d <= %d", depth+1, got, previous)
		}
		previous = got
		p.Release()
	}
	table.Release()
	batch.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("reservation tests leaked: %+v", usage)
	}
	if saturatingMultiply(math.MaxInt64, 2) != math.MaxInt64 {
		t.Fatal("reservation overflow")
	}
}

func TestGroupedReservationIncludesSimultaneousAggregatePrograms(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	v := store.MakeVector(a, 32, dtype.Int64T(), false)
	batch := store.MakeBatch([]store.Vector{v})
	table := store.MakeTable([]*store.Batch{batch})
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()}))
	one, err := MakePhysicalPlan(scan.GroupBy([]Expr{MakeColumn("x")}, MakeSum(MakeColumn("x").Add(1))))
	if err != nil {
		t.Fatal(err)
	}
	two, err := MakePhysicalPlan(scan.GroupBy([]Expr{MakeColumn("x")}, MakeSum(MakeColumn("x").Add(1)), MakeSum(MakeColumn("x").Add(2))))
	if err != nil {
		t.Fatal(err)
	}
	if two.batchReservation(batch) <= one.batchReservation(batch) {
		t.Fatal("simultaneously live group inputs were not counted")
	}
	one.Release()
	two.Release()
	table.Release()
	batch.Release()
}
