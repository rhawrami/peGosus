package plan

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalComputedProjectionAndCheckedCast(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	x := store.MakeVector(a, 5, dtype.Int32T(), true)
	copy(x.I32s(), []int32{1, 4, 9, 16, 25})
	x.Validity().Clear(4)
	y := store.MakeVector(a, 5, dtype.Int32T(), false)
	copy(y.I32s(), []int32{2, 2, 3, 4, 5})
	f := store.MakeVector(a, 5, dtype.Float64T(), false)
	copy(f.F64s(), []float64{1.75, math.NaN(), math.Inf(1), 2147483647.5, -2.25})
	leftDate := store.MakeVector(a, 5, dtype.DateT(), false)
	rightDate := store.MakeVector(a, 5, dtype.DateT(), false)
	copy(leftDate.I32s(), []int32{10, 20, 30, 40, 50})
	copy(rightDate.I32s(), []int32{1, 2, 3, 4, 5})
	batch := store.MakeBatch([]store.Vector{x, y, f, leftDate, rightDate})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability(
		[]string{"x", "y", "f", "left_date", "right_date"},
		[]dtype.Type{dtype.Int32T(), dtype.Int32T(), dtype.Float64T(), dtype.DateT(), dtype.DateT()},
		[]bool{true, false, false, false, false},
	)
	logical := MakeScan(table, schema).Project(
		MakeColumn("x").Add(MakeColumn("y")).Alias("sum"),
		MakeColumn("x").Div(MakeColumn("y")).Alias("ratio"),
		MakeColumn("f").Cast(dtype.Int32T()).Alias("integer"),
		MakeColumn("f").Sqrt().Alias("root"),
		MakeColumn("left_date").Sub(MakeColumn("right_date")).Alias("days"),
		MakeLiteral("long constant string").Alias("constant"),
	)
	physical, err := MakePhysicalPlan(logical)
	if err != nil {
		t.Fatalf("lower computed projection: %v", err)
	}
	table.Release()
	var output *store.Batch
	if !physical.Execute(a, func(batch *store.Batch) bool {
		output = batch.Retain()
		return true
	}) {
		t.Fatal("computed projection execution failed")
	}
	physical.Release()
	if output == nil || output.NVectors() != 6 || output.Len() != 5 {
		t.Fatal("unexpected computed output shape")
	}
	if got := output.VectorAt(0).I32s(); got[0] != 3 || got[3] != 20 || output.VectorAt(0).Validity().IsSet(4) {
		t.Fatal("unexpected addition output")
	}
	if got := output.VectorAt(1).F32s(); got[0] != 0.5 || got[3] != 4 || output.VectorAt(1).Validity().IsSet(4) {
		t.Fatal("unexpected division output")
	}
	integer := output.VectorAt(2)
	if integer.I32s()[0] != 1 || integer.I32s()[3] != math.MaxInt32 || integer.I32s()[4] != -2 {
		t.Fatalf("unexpected checked cast payload: %v", integer.I32s())
	}
	if !integer.Validity().IsSet(0) || integer.Validity().IsSet(1) || integer.Validity().IsSet(2) || !integer.Validity().IsSet(3) {
		t.Fatal("unexpected checked cast validity")
	}
	if got := output.VectorAt(4).I32s(); got[0] != 9 || got[4] != 45 {
		t.Fatalf("unexpected date subtraction: %v", got)
	}
	if got := output.VectorAt(5).Strings()[4].View(); got != "long constant string" {
		t.Fatalf("retained literal string: got %q", got)
	}
	output.Release()
}

func TestPhysicalThreeValuedBooleanProjection(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	left := makePhysicalBoolVector(a, []int8{1, 1, 1, 0, 0, 0, -1, -1, -1})
	right := makePhysicalBoolVector(a, []int8{1, 0, -1, 1, 0, -1, 1, 0, -1})
	batch := store.MakeBatch([]store.Vector{left, right})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchema([]string{"left", "right"}, []dtype.Type{dtype.BoolT(), dtype.BoolT()})
	logical := MakeScan(table, schema).Project(
		MakeColumn("left").And(MakeColumn("right")).Alias("and"),
		MakeColumn("left").Or(MakeColumn("right")).Alias("or"),
		MakeColumn("left").Not().Alias("not"),
		MakeColumn("left").IsNull().Alias("is_null"),
	)
	physical, err := MakePhysicalPlan(logical)
	if err != nil {
		t.Fatalf("lower boolean projection: %v", err)
	}
	table.Release()
	physical.Execute(a, func(output *store.Batch) bool {
		assertPhysicalBoolVector(t, output.VectorAt(0), []int8{1, 0, -1, 0, 0, 0, -1, 0, -1})
		assertPhysicalBoolVector(t, output.VectorAt(1), []int8{1, 1, 1, 1, 0, -1, 1, -1, -1})
		assertPhysicalBoolVector(t, output.VectorAt(2), []int8{0, 0, 0, 1, 1, 1, -1, -1, -1})
		assertPhysicalBoolVector(t, output.VectorAt(3), []int8{0, 0, 0, 0, 0, 0, 1, 1, 1})
		return true
	})
	physical.Release()
}

func TestPhysicalScalarLeftArithmetic(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	value := store.MakeVector(a, 4, dtype.Int32T(), false)
	copy(value.I32s(), []int32{1, 2, 4, 5})
	batch := store.MakeBatch([]store.Vector{value})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability([]string{"value"}, []dtype.Type{dtype.Int32T()}, []bool{false})
	logical := MakeScan(table, schema).Project(
		MakeLiteral(10).Sub(MakeColumn("value")),
		MakeLiteral(10).Div(MakeColumn("value")),
	)
	physical, err := MakePhysicalPlan(logical)
	if err != nil {
		t.Fatalf("lower scalar-left arithmetic: %v", err)
	}
	for i, node := range physical.steps[0].program.nodes {
		if node.kind == exprLiteral && physical.steps[0].program.materialize[i] {
			t.Fatal("numeric scalar operand was unnecessarily materialized")
		}
	}
	table.Release()
	physical.Execute(a, func(output *store.Batch) bool {
		wantSub := []int32{9, 8, 6, 5}
		wantDiv := []float32{10, 5, 2.5, 2}
		for i := range wantSub {
			if output.VectorAt(0).I32s()[i] != wantSub[i] || output.VectorAt(1).F32s()[i] != wantDiv[i] {
				t.Fatalf("row %d: got %d/%v, expected %d/%v", i, output.VectorAt(0).I32s()[i], output.VectorAt(1).F32s()[i], wantSub[i], wantDiv[i])
			}
		}
		return true
	})
	physical.Release()
}

func TestPhysicalArbitraryFilterComposesSelection(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	x := store.MakeVector(a, 4, dtype.Int64T(), false)
	y := store.MakeVector(a, 4, dtype.Int64T(), false)
	copy(x.I64s(), []int64{1, 5, 10, 20})
	copy(y.I64s(), []int64{1, 2, 6, 5})
	flag := makePhysicalBoolVector(a, []int8{1, 0, -1, 1})
	batch := store.MakeBatch([]store.Vector{x, y, flag})
	batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 4, []uint32{1, 2, 3})))
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchema([]string{"x", "y", "flag"}, []dtype.Type{dtype.Int64T(), dtype.Int64T(), dtype.BoolT()})
	predicate := MakeColumn("x").Add(1).
		Gt(MakeColumn("y").Mul(2)).
		And(MakeColumn("flag").Or(MakeColumn("x").IsNull()))
	logical := MakeScan(table, schema).Filter(predicate).Project(MakeColumn("x").Sub(MakeColumn("y")))
	physical, err := MakePhysicalPlan(logical)
	if err != nil {
		t.Fatalf("lower arbitrary filter: %v", err)
	}
	table.Release()
	physical.Execute(a, func(output *store.Batch) bool {
		if output.ActiveLen() != 1 || output.VectorAt(0).I64s()[3] != 15 {
			t.Fatal("arbitrary filter produced unexpected rows")
		}
		bitmap, ok := output.Selection().AsBitMap()
		if !ok || !bitmap.IsSet(3) {
			t.Fatal("filter did not compose the prior selection")
		}
		return true
	})
	physical.Release()
}

func TestPhysicalBetweenClipCoalesceAndStringComparison(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	value := store.MakeVector(a, 4, dtype.Int32T(), true)
	copy(value.I32s(), []int32{-1, 99, 5, 10})
	value.Validity().Clear(1)
	fallback := store.MakeVector(a, 4, dtype.Int32T(), false)
	copy(fallback.I32s(), []int32{2, 2, 2, 2})
	strings := store.MakeStringVector(a, [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("b")}, nil)
	batch := store.MakeBatch([]store.Vector{value, fallback, strings})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability(
		[]string{"value", "fallback", "text"},
		[]dtype.Type{dtype.Int32T(), dtype.Int32T(), dtype.StringT()},
		[]bool{true, false, false},
	)
	logical := MakeScan(table, schema).Project(
		MakeColumn("value").Coalesce(MakeColumn("fallback")),
		MakeColumn("value").Clip(0, 6),
		MakeColumn("value").Between(0, 6),
		MakeColumn("value").NotBetween(0, 6),
		MakeColumn("text").Ge("b"),
	)
	physical, err := MakePhysicalPlan(logical)
	if err != nil {
		t.Fatalf("lower ternary expressions: %v", err)
	}
	table.Release()
	physical.Execute(a, func(output *store.Batch) bool {
		if got := output.VectorAt(0).I32s(); got[0] != -1 || got[1] != 2 || got[2] != 5 || got[3] != 10 || output.VectorAt(0).Validity() != nil {
			t.Fatal("unexpected coalesce output")
		}
		if got := output.VectorAt(1).I32s(); got[0] != 0 || got[2] != 5 || got[3] != 6 || output.VectorAt(1).Validity().IsSet(1) {
			t.Fatal("unexpected clip output")
		}
		assertPhysicalBoolVector(t, output.VectorAt(2), []int8{0, -1, 1, 0})
		assertPhysicalBoolVector(t, output.VectorAt(3), []int8{1, -1, 0, 1})
		assertPhysicalBoolVector(t, output.VectorAt(4), []int8{0, 1, 1, 1})
		return true
	})
	physical.Release()
}

func TestPhysicalLazyCase(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	condition := makePhysicalBoolVector(a, []int8{1, 0, -1, 1, 0})
	value := store.MakeVector(a, 5, dtype.Float64T(), false)
	copy(value.F64s(), []float64{math.Inf(1), 12.75, -2.5, math.NaN(), math.Inf(-1)})
	strings := store.MakeStringVector(a, [][]byte{
		[]byte("first long string value"), []byte("second long string value"),
		[]byte("third long string value"), []byte("fourth long string value"), []byte("fifth long string value"),
	}, nil)
	batch := store.MakeBatch([]store.Vector{condition, value, strings})
	batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 5, []uint32{0, 2, 4})))
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchema([]string{"condition", "value", "text"}, []dtype.Type{dtype.BoolT(), dtype.Float64T(), dtype.StringT()})
	logical := MakeScan(table, schema).Project(
		MakeCase(MakeColumn("condition"), 7, MakeColumn("value").Cast(dtype.Int32T())),
		MakeCase(MakeColumn("condition"), MakeColumn("text"), "fallback long string value"),
		MakeCase(MakeColumn("condition"), MakeColumn("condition"), MakeColumn("value").Gt(0)),
		MakeCase(MakeColumn("condition"), MakeCase(MakeColumn("condition"), 1, 2), 3),
	)
	physical, err := MakePhysicalPlan(logical)
	if err != nil {
		t.Fatalf("lower CASE: %v", err)
	}
	table.Release()
	var output *store.Batch
	if !physical.Execute(a, func(batch *store.Batch) bool { output = batch.Retain(); return true }) {
		t.Fatal("execute CASE")
	}
	physical.Release()
	defer output.Release()
	if output.ActiveLen() != 3 || output.Len() != 5 {
		t.Fatal("CASE changed the physical row domain or selection")
	}
	want := []int32{7, 12, -2, 7, 0}
	for row, expected := range want {
		v := output.VectorAt(0)
		if row == 4 {
			if v.Validity() == nil || v.Validity().IsSet(row) {
				t.Fatal("invalid selected branch cast must yield NULL")
			}
		} else if v.I32s()[row] != expected || (v.Validity() != nil && !v.Validity().IsSet(row)) {
			t.Fatalf("CASE row %d: got %d, expected %d", row, v.I32s()[row], expected)
		}
	}
	for row, expected := range []string{"first long string value", "fallback long string value", "fallback long string value", "fourth long string value", "fallback long string value"} {
		if got := output.VectorAt(1).Strings()[row].View(); got != expected {
			t.Fatalf("string CASE row %d: got %q, expected %q", row, got, expected)
		}
	}
	assertPhysicalBoolVector(t, output.VectorAt(2), []int8{1, 1, 0, 1, 0})
	for row, expected := range []int64{1, 3, 3, 1, 3} {
		if got := output.VectorAt(3).I64s()[row]; got != expected {
			t.Fatalf("nested CASE row %d: got %d, expected %d", row, got, expected)
		}
	}
}

func TestPhysicalStringCoalesceOwnsBacking(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	left := store.MakeStringVector(
		a,
		[][]byte{[]byte("left long string value"), []byte("unused"), []byte("short")},
		[]bool{true, false, true},
	)
	right := store.MakeStringVector(
		a,
		[][]byte{[]byte("unused"), []byte("right long string value"), []byte("unused")},
		nil,
	)
	batch := store.MakeBatch([]store.Vector{left, right})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability(
		[]string{"left", "right"}, []dtype.Type{dtype.StringT(), dtype.StringT()}, []bool{true, false},
	)
	logical := MakeScan(table, schema).Project(MakeColumn("left").Coalesce(MakeColumn("right")))
	physical, err := MakePhysicalPlan(logical)
	if err != nil {
		t.Fatalf("lower string coalesce: %v", err)
	}
	table.Release()
	var output *store.Batch
	physical.Execute(a, func(batch *store.Batch) bool {
		output = batch.Retain()
		return true
	})
	physical.Release()
	want := []string{"left long string value", "right long string value", "short"}
	for i, expected := range want {
		if got := output.VectorAt(0).Strings()[i].View(); got != expected {
			t.Fatalf("row %d: got %q, expected %q", i, got, expected)
		}
	}
	output.Release()
}

func TestPhysicalNumericExpressionsRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(41, 42))
	for _, length := range []int{0, 1, 7, 8, 9, 17, 257} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
		x := store.MakeVector(a, length, dtype.Int64T(), true)
		y := store.MakeVector(a, length, dtype.Int64T(), true)
		selection := store.MakeBitMap(a, length)
		xValues, yValues := make([]int64, length), make([]int64, length)
		xValid, yValid := make([]bool, length), make([]bool, length)
		var selected int
		for i := range length {
			x.I64s()[i] = int64(rng.Uint64())
			y.I64s()[i] = int64(rng.Uint64())
			xValid[i], yValid[i] = true, true
			if rng.Uint32N(5) == 0 {
				x.Validity().Clear(i)
				xValid[i] = false
			}
			if rng.Uint32N(4) == 0 {
				y.Validity().Clear(i)
				yValid[i] = false
			}
			if rng.Uint32N(3) != 0 {
				selection.Set(i)
				selected++
			}
			xValues[i], yValues[i] = x.I64s()[i], y.I64s()[i]
		}
		batch := store.MakeBatch([]store.Vector{x, y})
		batch.SetSelection(store.MakeRowSelectionFromBitMap(selection))
		table := store.MakeTable([]*store.Batch{batch})
		batch.Release()
		schema := MakeSchema([]string{"x", "y"}, []dtype.Type{dtype.Int64T(), dtype.Int64T()})
		logical := MakeScan(table, schema).Project(
			MakeColumn("x").Add(MakeColumn("y")),
			MakeLiteral(11).Sub(MakeColumn("x")),
			MakeColumn("x").Gt(MakeColumn("y")),
			MakeColumn("x").Between(-100, 100),
			MakeColumn("x").Coalesce(7),
		)
		physical, err := MakePhysicalPlan(logical)
		if err != nil {
			t.Fatalf("length %d: lower randomized projection: %v", length, err)
		}
		table.Release()
		physical.Execute(a, func(output *store.Batch) bool {
			if output.Len() != length || output.ActiveLen() != selected {
				t.Fatalf("length %d: selection was not preserved", length)
			}
			for i := range length {
				if output.VectorAt(0).I64s()[i] != xValues[i]+yValues[i] || output.VectorAt(0).Validity().IsSet(i) != (xValid[i] && yValid[i]) {
					t.Fatalf("length %d row %d: incorrect addition", length, i)
				}
				if output.VectorAt(1).I64s()[i] != 11-xValues[i] || output.VectorAt(1).Validity().IsSet(i) != xValid[i] {
					t.Fatalf("length %d row %d: incorrect scalar-left subtraction", length, i)
				}
				assertPhysicalBoolAt(t, output.VectorAt(2), i, xValues[i] > yValues[i], xValid[i] && yValid[i])
				assertPhysicalBoolAt(t, output.VectorAt(3), i, xValues[i] >= -100 && xValues[i] <= 100, xValid[i])
				wantCoalesce := xValues[i]
				if !xValid[i] {
					wantCoalesce = 7
				}
				if output.VectorAt(4).I64s()[i] != wantCoalesce || output.VectorAt(4).Validity() != nil {
					t.Fatalf("length %d row %d: incorrect coalesce", length, i)
				}
			}
			return true
		})
		physical.Release()
	}
}

func makePhysicalBoolVector(a *mem.Allocator, values []int8) store.Vector {
	vector := store.MakeVector(a, len(values), dtype.BoolT(), true)
	for i, value := range values {
		if value < 0 {
			vector.Validity().Clear(i)
			continue
		}
		if value != 0 {
			vector.Bools()[i>>3] |= 1 << (i & 7)
		}
	}
	return vector
}

func assertPhysicalBoolVector(t *testing.T, vector *store.Vector, want []int8) {
	t.Helper()
	for i, expected := range want {
		valid := vector.Validity() == nil || vector.Validity().IsSet(i)
		if expected < 0 {
			if valid {
				t.Fatalf("row %d: got valid boolean, expected NULL", i)
			}
			continue
		}
		got := vector.Bools()[i>>3]>>(i&7)&1 != 0
		if !valid || got != (expected != 0) {
			t.Fatalf("row %d: got value=%t valid=%t, expected %d", i, got, valid, expected)
		}
	}
}

func assertPhysicalBoolAt(t *testing.T, vector *store.Vector, row int, want, valid bool) {
	t.Helper()
	gotValid := vector.Validity() == nil || vector.Validity().IsSet(row)
	got := vector.Bools()[row>>3]>>(row&7)&1 != 0
	if gotValid != valid || (valid && got != want) {
		t.Fatalf("row %d: got value=%t valid=%t, expected value=%t valid=%t", row, got, gotValid, want, valid)
	}
}
