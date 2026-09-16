package plan

import (
	"errors"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestBindReportsStructuredNameErrors(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	x := store.MakeVector(a, 1, dtype.Int32T(), false)
	y := store.MakeVector(a, 1, dtype.Int32T(), false)
	batch := store.MakeBatch([]store.Vector{x, y})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()

	tests := []struct {
		name string
		plan LogicalPlan
		code ErrorCode
	}{
		{
			"missing",
			MakeScan(table, MakeSchema([]string{"x", "y"}, []dtype.Type{dtype.Int32T(), dtype.Int32T()})).Filter(MakeColumn("z").Eq(1)),
			ErrorMissingColumn,
		},
		{
			"ambiguous",
			MakeScan(table, MakeSchema([]string{"x", "x"}, []dtype.Type{dtype.Int32T(), dtype.Int32T()})).Filter(MakeColumn("x").Eq(1)),
			ErrorAmbiguousColumn,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bound, err := BindLogicalPlan(test.plan)
			if err == nil || bound != nil {
				t.Fatal("invalid name was bound")
			}
			var planErr *PlanError
			if !errors.As(err, &planErr) || planErr.Code() != test.code {
				t.Fatalf("error: got %v, expected code %d", err, test.code)
			}
		})
	}
	table.Release()
}

func TestBindDirectLiteralAndInsertedCast(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	batch := store.MakeBatch([]store.Vector{store.MakeVector(a, 1, dtype.Int64T(), false)})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability([]string{"x"}, []dtype.Type{dtype.Int64T()}, []bool{false})

	direct, err := BindLogicalPlan(MakeScan(table, schema).Filter(MakeColumn("x").Gt(7)))
	if err != nil {
		t.Fatalf("bind direct literal: %v", err)
	}
	predicate := direct.expression(direct.root.predicate)
	literal := direct.expression(predicate.children[1])
	if literal.kind != exprLiteral || !literal.dType.Equal(dtype.Int64T()) || literal.literal.i64() != 7 {
		t.Fatalf("direct literal was not context-bound to INT64: %+v", literal)
	}
	if predicate.nullable {
		t.Fatal("non-nullable comparison became nullable")
	}

	coerced, err := BindLogicalPlan(MakeScan(table, schema).Filter(MakeColumn("x").Gt(MakeI32Scalar(7))))
	if err != nil {
		t.Fatalf("bind typed literal: %v", err)
	}
	predicate = coerced.expression(coerced.root.predicate)
	cast := coerced.expression(predicate.children[1])
	if cast.kind != exprCast || !cast.dType.Equal(dtype.Int64T()) {
		t.Fatalf("numeric coercion did not insert an INT64 cast: %+v", cast)
	}
	source := coerced.expression(cast.children[0])
	if source.kind != exprLiteral || !source.dType.Equal(dtype.Int32T()) {
		t.Fatal("inserted cast lost its INT32 source")
	}
	table.Release()
}

func TestBindUntypedOperandsJointly(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	batch := store.MakeBatch([]store.Vector{
		store.MakeVector(a, 1, dtype.Int32T(), false),
		store.MakeVector(a, 1, dtype.BoolT(), false),
	})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability(
		[]string{"value", "condition"}, []dtype.Type{dtype.Int32T(), dtype.BoolT()}, []bool{false, false},
	)
	logical := MakeScan(table, schema).Project(
		MakeColumn("value").Add(3_000_000_000).Alias("wide"),
		MakeLiteral(nil).Coalesce(1).Alias("coalesced"),
		MakeLiteral(nil).Coalesce(nil, 1).Alias("variadic"),
		MakeColumn("value").Coalesce(1, 2).Alias("contextual_variadic"),
		MakeCase(MakeColumn("condition"), nil, 1).Alias("chosen"),
		MakeColumn("value").Between(3_000_000_000, MakeF64Scalar(4)).Alias("between"),
		MakeLiteral(1).IsNull().Alias("literal_is_null"),
		MakeLiteral(nil).Coalesce(nil).Eq(MakeColumn("value")).Alias("contextual_nulls"),
	)
	bound, err := BindLogicalPlan(logical)
	if err != nil {
		t.Fatalf("bind jointly inferred operands: %v", err)
	}
	wantTypes := []dtype.Type{
		dtype.Int64T(), dtype.Int64T(), dtype.Int64T(), dtype.Int32T(), dtype.Int64T(), dtype.BoolT(), dtype.BoolT(), dtype.BoolT(),
	}
	wantNullable := []bool{false, false, false, false, true, false, false, true}
	for i := range wantTypes {
		field := bound.Schema().FieldAt(i)
		if !field.Type().Equal(wantTypes[i]) || field.Nullable() != wantNullable[i] {
			t.Fatalf("field %d: got %s nullable=%t", i, field.Type(), field.Nullable())
		}
	}
	wide := bound.expression(bound.root.projections[0])
	if bound.expression(wide.children[0]).kind != exprCast {
		t.Fatal("oversized literal did not widen the INT32 column")
	}
	table.Release()
}

func TestBindExplicitCastPreservesSourceType(t *testing.T) {
	table := store.MakeEmptyTable(nil)
	logical := MakeScan(table, MakeSchema(nil, nil)).Project(MakeLiteral(2_147_483_648).Cast(dtype.Int32T()))
	bound, err := BindLogicalPlan(logical)
	if err != nil {
		t.Fatalf("bind explicit narrowing cast: %v", err)
	}
	cast := bound.expression(bound.root.projections[0])
	if cast.kind != exprCast || cast.implicit || !cast.dType.Equal(dtype.Int32T()) {
		t.Fatalf("explicit cast was not preserved: %+v", cast)
	}
	source := bound.expression(cast.children[0])
	if source.kind != exprLiteral || !source.dType.Equal(dtype.Int64T()) || source.literal.i64() != 2_147_483_648 {
		t.Fatalf("explicit cast source was contextually narrowed: %+v", source)
	}

	invalid := MakeScan(table, MakeSchema(nil, nil)).Project(MakeLiteral(1).Cast(dtype.BoolT()))
	if plan, err := BindLogicalPlan(invalid); err == nil || plan != nil {
		t.Fatal("bound an undefined numeric-to-boolean cast")
	} else {
		var planErr *PlanError
		if !errors.As(err, &planErr) || planErr.Code() != ErrorImpossibleCoercion {
			t.Fatalf("unexpected cast error: %v", err)
		}
	}
	table.Release()
}

func TestBindExpressionTypesAndNullability(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	vectors := []store.Vector{
		store.MakeVector(a, 1, dtype.Int32T(), false),
		store.MakeVector(a, 1, dtype.Int64T(), true),
		store.MakeVector(a, 1, dtype.DateT(), false),
		store.MakeVector(a, 1, dtype.DateT(), false),
		store.MakeVector(a, 1, dtype.BoolT(), true),
	}
	batch := store.MakeBatch(vectors)
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability(
		[]string{"i32", "i64", "left_date", "right_date", "flag"},
		[]dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.DateT(), dtype.DateT(), dtype.BoolT()},
		[]bool{false, true, false, false, true},
	)
	logical := MakeScan(table, schema).Project(
		MakeColumn("i32").Add(MakeColumn("i64")).Alias("sum"),
		MakeColumn("i32").Div(2).Alias("ratio"),
		MakeColumn("left_date").Sub(MakeColumn("right_date")).Alias("days"),
		MakeColumn("flag").And(true).Alias("both"),
		MakeColumn("i64").IsNull().Alias("missing"),
	)
	bound, err := BindLogicalPlan(logical)
	if err != nil {
		t.Fatalf("bind expressions: %v", err)
	}
	wantTypes := []dtype.Type{dtype.Int64T(), dtype.Float32T(), dtype.Int32T(), dtype.BoolT(), dtype.BoolT()}
	wantNullable := []bool{true, false, false, true, false}
	for i := range wantTypes {
		field := bound.Schema().FieldAt(i)
		if !field.Type().Equal(wantTypes[i]) || field.Nullable() != wantNullable[i] {
			t.Fatalf("field %d: got %s nullable=%t", i, field.Type(), field.Nullable())
		}
	}
	add := bound.expression(bound.root.projections[0])
	if bound.expression(add.children[0]).kind != exprCast {
		t.Fatal("mixed-width addition did not cast INT32 to INT64")
	}
	table.Release()
}

func TestBindCaseCoalesceAndAliases(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	vectors := []store.Vector{
		store.MakeVector(a, 1, dtype.BoolT(), true),
		store.MakeVector(a, 1, dtype.Int32T(), true),
	}
	batch := store.MakeBatch(vectors)
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability(
		[]string{"condition", "value"}, []dtype.Type{dtype.BoolT(), dtype.Int32T()}, []bool{true, true},
	)
	logical := MakeScan(table, schema).Project(
		MakeCase(MakeColumn("condition"), MakeColumn("value"), 9).Alias("chosen"),
		MakeColumn("value").Coalesce(0).Alias("filled"),
	)
	bound, err := BindLogicalPlan(logical)
	if err != nil {
		t.Fatalf("bind conditional expressions: %v", err)
	}
	if bound.Schema().FieldAt(0).Name() != "chosen" || !bound.Schema().FieldAt(0).Nullable() {
		t.Fatal("unexpected CASE field")
	}
	if bound.Schema().FieldAt(1).Name() != "filled" || bound.Schema().FieldAt(1).Nullable() {
		t.Fatal("unexpected COALESCE field")
	}
	if bound.Schema().FieldAt(0).ID() == bound.Schema().FieldAt(1).ID() || bound.Schema().FieldAt(0).ID() == schema.FieldAt(1).ID() {
		t.Fatal("projected fields did not receive stable distinct IDs")
	}
	table.Release()
}

func TestBoundFieldIDsSurvivePlanEdges(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	batch := store.MakeBatch([]store.Vector{store.MakeVector(a, 1, dtype.Int32T(), false)})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability([]string{"x"}, []dtype.Type{dtype.Int32T()}, []bool{false})
	logical := MakeScan(table, schema).
		Project(MakeColumn("x").Alias("y")).
		Filter(MakeColumn("y").Eq(1))
	bound, err := BindLogicalPlan(logical)
	if err != nil {
		t.Fatalf("bind projected filter: %v", err)
	}
	project := bound.root.input
	projectedID := project.schema.FieldAt(0).ID()
	if projectedID == schema.FieldAt(0).ID() {
		t.Fatal("projection reused its input field identity")
	}
	predicate := bound.expression(bound.root.predicate)
	column := bound.expression(predicate.children[0])
	if column.field != projectedID || bound.Schema().FieldAt(0).ID() != projectedID {
		t.Fatal("filter or output schema lost the projected field identity")
	}
	projection := bound.expression(project.projections[0])
	if projection.field != schema.FieldAt(0).ID() {
		t.Fatal("projection expression lost its input field identity")
	}
	table.Release()
}

func TestScalarExceptionalCastReturnsNull(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(-1), math.Inf(1), math.MaxFloat64} {
		cast, ok := MakeF64Scalar(value).cast(dtype.Int64T())
		if !ok || !cast.IsNull() || !cast.Type().Equal(dtype.Int64T()) {
			t.Fatalf("exceptional cast %v: got %+v, %t", value, cast, ok)
		}
	}
	cast, ok := MakeF64Scalar(9.75).cast(dtype.Int32T())
	if !ok || cast.IsNull() || cast.i32() != 9 {
		t.Fatalf("ordinary lossy cast: got %+v, %t", cast, ok)
	}
}

func TestPhysicalRejectsBoundComputedProjection(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	batch := store.MakeBatch([]store.Vector{store.MakeVector(a, 1, dtype.Int32T(), false)})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	logical := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int32T()})).Project(MakeColumn("x").Add(1))
	physical, err := MakePhysicalPlan(logical)
	if err == nil || physical != nil {
		t.Fatal("lowered an unimplemented computed projection")
	}
	var planErr *PlanError
	if !errors.As(err, &planErr) || planErr.Code() != ErrorUnsupportedOperation {
		t.Fatalf("unexpected physical error: %v", err)
	}
	table.Release()
}

func TestBindRejectsUnderstatedSourceNullability(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	vector := store.MakeVector(a, 1, dtype.Int32T(), true)
	vector.Validity().Clear(0)
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchemaWithNullability([]string{"x"}, []dtype.Type{dtype.Int32T()}, []bool{false})
	bound, err := BindLogicalPlan(MakeScan(table, schema))
	if err == nil || bound != nil {
		t.Fatal("bound a non-nullable schema over nullable source data")
	}
	var planErr *PlanError
	if !errors.As(err, &planErr) || planErr.Code() != ErrorTypeMismatch {
		t.Fatalf("unexpected nullability error: %v", err)
	}
	table.Release()
}
