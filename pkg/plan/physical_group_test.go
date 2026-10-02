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

func TestPhysicalGroupedAggregation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	groups := [][]float64{{0, math.NaN(), 3}, {math.Copysign(0, -1), math.Float64frombits(0x7ff8000000000001), 3}}
	var batches []*store.Batch
	for i, keys := range groups {
		key := store.MakeVector(a, 3, dtype.Float64T(), true)
		copy(key.F64s(), keys)
		key.Validity().Clear(2)
		value := store.MakeVector(a, 3, dtype.Int32T(), true)
		copy(value.I32s(), []int32{1, 2, 3})
		if i == 1 {
			value.Validity().Clear(2)
		}
		batches = append(batches, store.MakeBatch([]store.Vector{key, value}))
	}
	table := store.MakeTable(batches)
	for _, b := range batches {
		b.Release()
	}
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "value"}, []dtype.Type{dtype.Float64T(), dtype.Int32T()})).GroupBy(
		[]Expr{MakeColumn("key").Alias("group")}, MakeCountStar().Alias("n"), MakeCount(MakeColumn("value")), MakeSum(MakeColumn("value")),
	).OrderBy(MakeOrderKey(MakeColumn("n")).Desc()))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	var output *store.Batch
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 16384}, func(b *store.Batch) bool { output = b.Retain(); return true })
	physical.Release()
	if result.Code() != ExecutionCompleted || output == nil {
		t.Fatalf("grouped aggregation failed: %v", result.Code())
	}
	defer output.Release()
	if output.Len() != 3 {
		t.Fatalf("group count: got %d", output.Len())
	}
	if output.VectorAt(0).F64s()[0] != 0 || !math.IsNaN(output.VectorAt(0).F64s()[1]) || output.VectorAt(0).Validity().IsSet(2) {
		t.Fatal("grouping did not canonicalize signed zero, NaN or NULL")
	}
	for i, counts := range [][3]int64{{2, 2, 2}, {2, 2, 4}, {2, 1, 3}} {
		if output.VectorAt(1).I64s()[i] != counts[0] || output.VectorAt(2).I64s()[i] != counts[1] || output.VectorAt(3).I64s()[i] != counts[2] {
			t.Fatalf("group %d: count/sum mismatch", i)
		}
	}
}

func TestPhysicalGroupedCountDistinct(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	key := store.MakeVector(a, 6, dtype.Int32T(), false)
	copy(key.I32s(), []int32{1, 1, 1, 2, 2, 2})
	value := store.MakeVector(a, 6, dtype.Int32T(), true)
	copy(value.I32s(), []int32{5, 5, 6, 5, 7, 9})
	value.Validity().Clear(5)
	batch := store.MakeBatch([]store.Vector{key, value})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "value"}, []dtype.Type{dtype.Int32T(), dtype.Int32T()})).GroupBy(
		[]Expr{MakeColumn("key")}, MakeCountDistinct(MakeColumn("value")), MakeSum(MakeColumn("value")).Distinct(),
	))
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	result := physical.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8192}, func(b *store.Batch) bool {
		if b.Len() != 2 || b.VectorAt(1).I64s()[0] != 2 || b.VectorAt(2).I64s()[0] != 11 || b.VectorAt(1).I64s()[1] != 2 || b.VectorAt(2).I64s()[1] != 12 {
			t.Fatal("grouped DISTINCT results differ from scalar reference")
		}
		return true
	})
	physical.Release()
	if result.Code() != ExecutionCompleted {
		t.Fatalf("grouped DISTINCT result: %v", result.Code())
	}
}

func TestStringGroupLookupAcrossBatchesAndNulls(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	var batches []*store.Batch
	type totals struct{ count, sum int64 }
	want := make(map[string]totals)
	for part := range 8 {
		stringsIn := make([][]byte, 257)
		valid := make([]bool, 257)
		values := store.MakeVector(a, 257, dtype.Int32T(), true)
		selection := store.MakeBitMap(a, 257)
		var offsets []uint32
		for row := range 257 {
			id := part*257 + row
			name := []string{"north", "south", "ø", "", "long grouping key with storage"}[id%5]
			if id%17 == 0 {
				name = "<NULL>"
			} else {
				stringsIn[row], valid[row] = []byte(name), true
			}
			values.I32s()[row] = int32(id%23 - 11)
			if id%11 == 0 {
				values.Validity().Clear(row)
			}
			if id%7 == 0 {
				continue
			}
			selection.Set(row)
			offsets = append(offsets, uint32(row))
			total := want[name]
			total.count++
			if values.Validity().IsSet(row) {
				total.sum += int64(values.I32s()[row])
			}
			want[name] = total
		}
		batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, stringsIn, valid), values})
		if part%2 == 0 {
			batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
		} else {
			selection.Release()
			batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 257, offsets)))
		}
		batches = append(batches, batch)
	}
	table := store.MakeTable(batches)
	for _, batch := range batches {
		batch.Release()
	}
	scan := MakeScan(table, MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{dtype.StringT(), dtype.Int32T()}, []bool{true, true}))
	p, err := MakePhysicalPlan(scan.GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeSum(MakeColumn("value"))))
	if err != nil {
		t.Fatal(err)
	}
	for _, workers := range []int{1, 4} {
		seen := make(map[string]bool)
		result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 64 << 20, Workers: workers}, func(output *store.Batch) bool {
			for row := range output.Len() {
				key := "<NULL>"
				if v := output.VectorAt(0); v.Validity() == nil || v.Validity().IsSet(row) {
					key = v.Strings()[row].View()
				}
				value, ok := want[key]
				if seen[key] || !ok || output.VectorAt(1).I64s()[row] != value.count || output.VectorAt(2).I64s()[row] != value.sum {
					t.Errorf("workers %d group %q: got %d/%d, expected %+v", workers, key, output.VectorAt(1).I64s()[row], output.VectorAt(2).I64s()[row], value)
				}
				seen[key] = true
			}
			return true
		})
		if result.Code() != ExecutionCompleted || len(seen) != len(want) {
			t.Fatalf("workers %d grouped %v/%v, %d/%d groups", workers, result.Code(), result.Err(), len(seen), len(want))
		}
	}
	p.Release()
	table.Release()
}

func TestStringGroupLookupHighCardinality(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	stringsIn := make([][]byte, 301)
	for row := range stringsIn {
		stringsIn[row] = []byte(fmt.Sprintf("group-%04d", row%300))
	}
	batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, stringsIn, nil)})
	table := store.MakeTable([]*store.Batch{batch})
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key"}, []dtype.Type{dtype.StringT()}, []bool{false})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	step := p.steps[0]
	g := makeGroupState(step, nil)
	if !g.add(a, batch, step, math.MaxInt64) {
		t.Fatal("string group accumulation failed")
	}
	if !g.keys.index.stringKeys || len(g.values) != 300 || g.values[0][0].count != 2 {
		t.Fatal("high-cardinality lookup lost a key or count")
	}
	output := g.finish(a, step)
	g.release()
	batch.Release()
	table.Release()
	p.Release()
	if output == nil || output.VectorAt(0).Strings()[0].View() != "group-0000" || output.VectorAt(1).I64s()[0] != 2 {
		t.Fatal("high-cardinality result lost retained strings")
	}
	output.Release()
}

func TestPhysicalGroupCachedChargeTracksState(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	var batches []*store.Batch
	for _, value := range []string{"zebra long string value", "a", "zzzzzz longer string value", "b"} {
		key := store.MakeVector(a, 1, dtype.Int32T(), false)
		key.I32s()[0] = 1
		text := store.MakeStringVector(a, [][]byte{[]byte(value)}, nil)
		batches = append(batches, store.MakeBatch([]store.Vector{key, text}))
	}
	table := store.MakeTable(batches)
	plan, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"key", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()})).GroupBy(
		[]Expr{MakeColumn("key")}, MakeMin(MakeColumn("text")), MakeMax(MakeColumn("text")), MakeCountDistinct(MakeColumn("text")),
	))
	if err != nil {
		t.Fatal(err)
	}
	state := &groupState{}
	for _, batch := range batches {
		if !state.add(a, batch, plan.steps[0], math.MaxInt64) {
			t.Fatal("grouping failed")
		}
		expected := saturatingAdd(state.keys.charged, int64(len(state.values)*len(plan.steps[0].aggregates))*64)
		for _, group := range state.values {
			for _, value := range group {
				if value.text != nil {
					expected = saturatingAdd(expected, int64(value.text.Len()))
				}
			}
		}
		for _, group := range state.uniques {
			for _, unique := range group {
				if unique != nil {
					expected = saturatingAdd(expected, unique.charged)
				}
			}
		}
		if got := state.charged(len(plan.steps[0].aggregates)); got != expected {
			t.Fatalf("cached group charge: got %d, want %d", got, expected)
		}
	}
	state.release()
	plan.Release()
	table.Release()
	for _, batch := range batches {
		batch.Release()
	}
}
