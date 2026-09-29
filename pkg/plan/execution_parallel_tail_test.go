package plan

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParallelGroupedOrderAndLimit(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 8)
	for part := range batches {
		ids := store.MakeVector(a, 257, dtype.Int32T(), false)
		values := store.MakeVector(a, 257, dtype.Int32T(), true)
		stringsIn := make([][]byte, 257)
		valid := make([]bool, 257)
		var offsets []uint32
		for row := range 257 {
			id := part*257 + row
			ids.I32s()[row] = int32(id)
			values.I32s()[row] = int32(id%17 - 8)
			if id%19 == 0 {
				values.Validity().Clear(row)
			}
			if id%13 != 0 {
				stringsIn[row], valid[row] = []byte(fmt.Sprintf("retained grouping key %d", id%7)), true
			}
			if id%5 != 0 {
				offsets = append(offsets, uint32(row))
			}
		}
		batches[part] = store.MakeBatch([]store.Vector{ids, store.MakeStringVector(a, stringsIn, valid), values})
		batches[part].SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 257, offsets)))
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	scan := MakeScan(table, MakeSchemaWithNullability([]string{"id", "key", "value"}, []dtype.Type{dtype.Int32T(), dtype.StringT(), dtype.Int32T()}, []bool{false, true, true}))
	query := scan.Filter(MakeColumn("id").Ge(100)).GroupBy([]Expr{MakeColumn("key").Alias("key")}, MakeCountStar().Alias("n"), MakeSum(MakeColumn("value")).Alias("total")).OrderBy(MakeOrderKey(MakeColumn("n")).Desc(), MakeOrderKey(MakeColumn("key")).NullsLast()).Limit(3, 1)
	p, err := MakePhysicalPlan(query)
	if err != nil {
		t.Fatal(err)
	}
	collect := func(batch *store.Batch) []string {
		var got []string
		mask := batch.Selection().MakeBitMapTemp(a)
		if mask != nil {
			defer mask.Release()
		}
		for row := range batch.Len() {
			if mask != nil && !mask.IsSet(row) {
				continue
			}
			key := "<null>"
			if v := batch.VectorAt(0); v.Validity() == nil || v.Validity().IsSet(row) {
				key = strings.Clone(v.Strings()[row].View())
			}
			got = append(got, fmt.Sprintf("%s:%d:%d", key, batch.VectorAt(1).I64s()[row], batch.VectorAt(2).I64s()[row]))
		}
		return got
	}
	var expected []string
	serial := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 1}, func(batch *store.Batch) bool { expected = append(expected, collect(batch)...); return true })
	if serial.Code() != ExecutionCompleted || len(expected) != 3 {
		t.Fatalf("serial reference: %v, %v", serial.Code(), expected)
	}
	var retained *store.Batch
	parallel, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(batch *store.Batch) bool { retained = batch.Retain(); return true })
	if !used || parallel.Code() != ExecutionCompleted || retained == nil {
		t.Fatalf("parallel ordered group: %t %v/%v", used, parallel.Code(), parallel.Err())
	}
	stopped := 0
	result, used := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(*store.Batch) bool { stopped++; return false })
	if !used || result.Code() != ExecutionStopped || stopped != 1 {
		t.Fatalf("ordered group early stop: %t %v calls %d", used, result.Code(), stopped)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result, used = p.executeParallel(ctx, a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(*store.Batch) bool { cancel(); return true })
	if !used || result.Code() != ExecutionCancelled {
		t.Fatalf("ordered group cancellation: %t %v/%v", used, result.Code(), result.Err())
	}
	zero, err := MakePhysicalPlan(scan.Filter(MakeColumn("id").Lt(0)).GroupBy([]Expr{MakeColumn("key").Alias("key")}, MakeCountStar()).OrderBy(MakeOrderKey(MakeColumn("key"))).Limit(0))
	if err != nil {
		t.Fatal(err)
	}
	result, used = zero.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: 4}, func(*store.Batch) bool { t.Error("empty group reached sink"); return true })
	zero.Release()
	if !used || result.Code() != ExecutionCompleted {
		t.Fatalf("empty grouped sort: %t %v/%v", used, result.Code(), result.Err())
	}
	p.Release()
	table.Release()
	got := collect(retained)
	retained.Release()
	if len(got) != len(expected) {
		t.Fatalf("retained ordered rows %v, want %v", got, expected)
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("retained ordered row %d: %s want %s", i, got[i], expected[i])
		}
	}
}

func TestParallelShardedGroupsWithOrderedLimit(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("requires at least two workers")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	batches := make([]*store.Batch, 8)
	for part := range batches {
		keys := store.MakeVector(a, 512, dtype.Int32T(), false)
		values := store.MakeVector(a, 512, dtype.Int32T(), false)
		for row := range 512 {
			keys.I32s()[row], values.I32s()[row] = int32(part*512+row), 1
		}
		batches[part] = store.MakeBatch([]store.Vector{keys, values})
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	scan := MakeScan(table, MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{dtype.Int32T(), dtype.Int32T()}, []bool{false, false}))
	p, err := MakePhysicalPlan(scan.GroupBy([]Expr{MakeColumn("key").Alias("key")}, MakeCountStar().Alias("rows"), MakeSum(MakeColumn("value")).Alias("total")).OrderBy(MakeOrderKey(MakeColumn("rows")).Desc(), MakeOrderKey(MakeColumn("key"))).Limit(10, 5))
	if err != nil {
		t.Fatal(err)
	}
	if makeCompactTopNState(p.steps[1]) == nil {
		t.Fatal("grouped ordered limit missed compact TopN")
	}
	var got []int32
	result, parallel := p.executeParallel(context.Background(), a, ExecutionOptions{MemoryBudget: 128 << 20, Workers: 4}, func(batch *store.Batch) bool {
		mask := batch.Selection().MakeBitMapTemp(a)
		for row := range batch.Len() {
			if mask != nil && !mask.IsSet(row) {
				continue
			}
			got = append(got, batch.VectorAt(0).I32s()[row])
			if batch.VectorAt(1).I64s()[row] != 1 || batch.VectorAt(2).I64s()[row] != 1 {
				t.Errorf("incorrect grouped count or sum")
			}
		}
		if mask != nil {
			mask.Release()
		}
		return true
	})
	p.Release()
	table.Release()
	if !parallel || result.Code() != ExecutionCompleted || len(got) != 10 {
		t.Fatalf("sharded grouped limit: %t %v/%v, %v", parallel, result.Code(), result.Err(), got)
	}
	for i, key := range got {
		if key != int32(i+5) {
			t.Fatalf("ordered key %d: %d want %d", i, key, i+5)
		}
	}
}
