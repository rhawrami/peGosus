package plan

import (
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestCompactGroupStateMergeCanonicalKeys(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	var batches []*store.Batch
	for _, values := range [][]float64{{math.Float64frombits(0x7ff8000000000001), math.Copysign(0, -1), 0}, {math.NaN(), 0, 0}} {
		key := store.MakeVector(a, 3, dtype.Float64T(), true)
		copy(key.F64s(), values)
		key.Validity().Clear(2)
		batches = append(batches, store.MakeBatch([]store.Vector{key}))
	}
	table := store.MakeTable(batches)
	p, err := MakePhysicalPlan(MakeScan(table, MakeSchemaWithNullability([]string{"key"}, []dtype.Type{dtype.Float64T()}, []bool{true})).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar()))
	if err != nil {
		t.Fatal(err)
	}
	step := p.steps[0]
	first, second := makeGroupState(step, nil), makeGroupState(step, nil)
	if !first.add(a, batches[0], step, math.MaxInt64) || !second.add(a, batches[1], step, math.MaxInt64) || !first.merge(a, second, step, math.MaxInt64) {
		t.Fatal("compact group merge failed")
	}
	second.release()
	for _, batch := range batches {
		batch.Release()
	}
	table.Release()
	p.Release()
	output := first.finish(a, step)
	first.release()
	if output == nil || output.Len() != 3 {
		t.Fatal("wrong compact group count")
	}
	defer output.Release()
	seen := make(map[string]bool)
	for row := range output.Len() {
		name := "null"
		if v := output.VectorAt(0); v.Validity().IsSet(row) {
			if math.IsNaN(v.F64s()[row]) {
				name = "NaN"
			} else if v.F64s()[row] == 0 {
				name = "zero"
			}
		}
		if seen[name] || output.VectorAt(1).I64s()[row] != 2 {
			t.Fatalf("merged %q incorrectly", name)
		}
		seen[name] = true
	}
}

func TestGeneralGroupStateMergeRetainsStrings(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	var batches []*store.Batch
	for _, names := range [][]string{{"long shared group key", "long first group key"}, {"long shared group key", "long second group key"}} {
		keys := store.MakeStringVector(a, [][]byte{[]byte(names[0]), []byte(names[1])}, nil)
		values := store.MakeVector(a, 2, dtype.Int32T(), false)
		values.I32s()[0], values.I32s()[1] = 3, 2
		batches = append(batches, store.MakeBatch([]store.Vector{keys, values}))
	}
	table := store.MakeTable(batches)
	schema := MakeSchemaWithNullability([]string{"key", "value"}, []dtype.Type{dtype.StringT(), dtype.Int32T()}, []bool{false, false})
	p, err := MakePhysicalPlan(MakeScan(table, schema).GroupBy([]Expr{MakeColumn("key")}, MakeCountStar(), MakeSum(MakeColumn("value"))))
	if err != nil {
		t.Fatal(err)
	}
	step := p.steps[0]
	first, second := makeGroupState(step, nil), makeGroupState(step, nil)
	if !first.add(a, batches[0], step, math.MaxInt64) || !second.add(a, batches[1], step, math.MaxInt64) || !first.merge(a, second, step, math.MaxInt64) {
		t.Fatal("general group merge failed")
	}
	second.release()
	for _, batch := range batches {
		batch.Release()
	}
	table.Release()
	output := first.finish(a, step)
	first.release()
	p.Release()
	if output == nil || output.Len() != 3 {
		t.Fatal("wrong general group count")
	}
	defer output.Release()
	want := map[string][2]int64{"long shared group key": {2, 6}, "long first group key": {1, 2}, "long second group key": {1, 2}}
	for row := range output.Len() {
		key := output.VectorAt(0).Strings()[row].View()
		counts, ok := want[key]
		if !ok || output.VectorAt(1).I64s()[row] != counts[0] || output.VectorAt(2).I64s()[row] != counts[1] {
			t.Fatalf("merged string %q incorrectly", key)
		}
	}
}
