package plan

import (
	"math"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPhysicalScanFilterProject(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	income := store.MakeVector(a, 4, dtype.Int64T(), true)
	copy(income.I64s(), []int64{50_000, 150_000, 200_000, 300_000})
	income.Validity().Clear(2)
	sex := store.MakeStringVector(a, [][]byte{[]byte("f"), []byte("male-long-value"), []byte("f"), []byte("m")}, nil)
	age := store.MakeVector(a, 4, dtype.Int32T(), false)
	copy(age.I32s(), []int32{20, 30, 40, 50})
	batch := store.MakeBatch([]store.Vector{income, sex, age})
	selected := store.MakeBitMap(a, 4)
	selected.Set(0)
	selected.Set(1)
	selected.Set(2)
	batch.SetSelection(store.MakeRowSelectionFromBitMap(selected))
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()

	schema := MakeSchema([]string{"income", "sex", "age"}, []dtype.Type{dtype.Int64T(), dtype.StringT(), dtype.Int32T()})
	logical := MakeScan(table, schema).
		Filter(MakeColumn("income").Gt(100_000)).
		Project(MakeColumn("sex"), MakeColumn("age"), MakeColumn("sex"))
	physical, err := MakePhysicalPlan(logical)
	if err != nil {
		t.Fatalf("failed to bind physical plan: %v", err)
	}
	table.Release()

	var output *store.Batch
	if completed := physical.Execute(a, func(batch *store.Batch) bool {
		output = batch.Retain()
		return true
	}); !completed {
		t.Fatal("execution stopped unexpectedly")
	}
	if output == nil || output.NVectors() != 3 || output.Len() != 4 || output.ActiveLen() != 1 {
		t.Fatal("unexpected output shape")
	}
	bitmap, ok := output.Selection().AsBitMap()
	if !ok || !bitmap.IsSet(1) || bitmap.ViN() != 1 {
		t.Fatal("unexpected output selection")
	}
	if got := output.VectorAt(0).Strings()[1].View(); got != "male-long-value" {
		t.Fatalf("projected string: got %q", got)
	}
	if got := output.VectorAt(1).I32s()[1]; got != 30 {
		t.Fatalf("projected age: got %d", got)
	}
	if !physical.Schema().FieldAt(0).Type().Equal(dtype.StringT()) || physical.Schema().FieldAt(1).Name() != "age" {
		t.Fatal("unexpected physical output schema")
	}
	output.Release()
	physical.Release()
}

func TestPhysicalComparisonDispatch(t *testing.T) {
	tests := []struct {
		name    string
		typeVal dtype.Type
		fill    func(*store.Vector)
		literal Scalar
	}{
		{"int32", dtype.Int32T(), func(v *store.Vector) { copy(v.I32s(), []int32{1, 2, 3}) }, MakeI32Scalar(2)},
		{"int64", dtype.Int64T(), func(v *store.Vector) { copy(v.I64s(), []int64{1, 2, 3}) }, MakeI64Scalar(2)},
		{"float32", dtype.Float32T(), func(v *store.Vector) { copy(v.F32s(), []float32{1, 2, 3}) }, MakeF32Scalar(2)},
		{"float64", dtype.Float64T(), func(v *store.Vector) { copy(v.F64s(), []float64{1, 2, 3}) }, MakeF64Scalar(2)},
		{"date", dtype.DateT(), func(v *store.Vector) { copy(v.I32s(), []int32{1, 2, 3}) }, MakeDateScalar(2)},
		{"timestamp", dtype.TimestampTZT(), func(v *store.Vector) { copy(v.I64s(), []int64{1, 2, 3}) }, MakeTimestampTZScalar(2)},
	}
	operations := []struct {
		name string
		make func(Expr, Scalar) Expr
		want []bool
	}{
		{"eq", func(e Expr, s Scalar) Expr { return e.Eq(s) }, []bool{false, true, false}},
		{"ne", func(e Expr, s Scalar) Expr { return e.Ne(s) }, []bool{true, false, true}},
		{"lt", func(e Expr, s Scalar) Expr { return e.Lt(s) }, []bool{true, false, false}},
		{"le", func(e Expr, s Scalar) Expr { return e.Le(s) }, []bool{true, true, false}},
		{"gt", func(e Expr, s Scalar) Expr { return e.Gt(s) }, []bool{false, false, true}},
		{"ge", func(e Expr, s Scalar) Expr { return e.Ge(s) }, []bool{false, true, true}},
	}

	for _, test := range tests {
		for _, operation := range operations {
			t.Run(test.name+"/"+operation.name, func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
				vector := store.MakeVector(a, 3, test.typeVal, false)
				test.fill(&vector)
				batch := store.MakeBatch([]store.Vector{vector})
				table := store.MakeTable([]*store.Batch{batch})
				batch.Release()
				logical := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{test.typeVal})).Filter(operation.make(MakeColumn("x"), test.literal))
				physical, err := MakePhysicalPlan(logical)
				table.Release()
				if err != nil {
					t.Fatalf("failed to bind comparison: %v", err)
				}
				physical.Execute(a, func(output *store.Batch) bool {
					bitmap, _ := output.Selection().AsBitMap()
					for i, want := range operation.want {
						if bitmap.IsSet(i) != want {
							t.Fatalf("row %d: got %t, expected %t", i, bitmap.IsSet(i), want)
						}
					}
					return true
				})
				physical.Release()
			})
		}
	}
}

func TestPhysicalComparisonKernelBoundaries(t *testing.T) {
	lengths := []int{1, 7, 8, 9, 15, 16, 17}
	types := []struct {
		typeVal dtype.Type
		fill    func(*store.Vector)
		literal Scalar
	}{
		{dtype.Int32T(), func(v *store.Vector) {
			for i := range v.I32s() {
				v.I32s()[i] = int32(i % 5)
			}
		}, MakeI32Scalar(2)},
		{dtype.Int64T(), func(v *store.Vector) {
			for i := range v.I64s() {
				v.I64s()[i] = int64(i % 5)
			}
		}, MakeI64Scalar(2)},
		{dtype.Float32T(), func(v *store.Vector) {
			for i := range v.F32s() {
				v.F32s()[i] = float32(i % 5)
			}
		}, MakeF32Scalar(2)},
		{dtype.Float64T(), func(v *store.Vector) {
			for i := range v.F64s() {
				v.F64s()[i] = float64(i % 5)
			}
		}, MakeF64Scalar(2)},
	}
	for _, typeTest := range types {
		for _, length := range lengths {
			t.Run(typeTest.typeVal.String(), func(t *testing.T) {
				a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
				vector := store.MakeVector(a, length, typeTest.typeVal, false)
				typeTest.fill(&vector)
				mask := store.MakeBitMapTemp(a, length)
				evaluateComparison(&vector, mask.Bytes(), physicalFilterData{
					dType: typeTest.typeVal, operation: exprOpGT, literal: typeTest.literal,
				})
				mask.RecalcNiN()
				for i := range length {
					if got, want := mask.IsSet(i), i%5 > 2; got != want {
						t.Fatalf("length %d, row %d: got %t, expected %t", length, i, got, want)
					}
				}
				mask.Release()
				vector.Release()
			})
		}
	}
}

func TestPhysicalFloatingPointSemantics(t *testing.T) {
	tests := []struct {
		operation exprOp
		want      []bool
	}{
		{exprOpEQ, []bool{false, true, true, false, false}},
		{exprOpNE, []bool{true, false, false, true, true}},
		{exprOpLT, []bool{false, false, false, true, false}},
		{exprOpLE, []bool{false, true, true, true, false}},
		{exprOpGT, []bool{false, false, false, false, true}},
		{exprOpGE, []bool{false, true, true, false, true}},
	}
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	for _, test := range tests {
		vector := store.MakeVector(a, 5, dtype.Float64T(), false)
		copy(vector.F64s(), []float64{math.NaN(), math.Copysign(0, -1), 0, math.Inf(-1), math.Inf(1)})
		mask := store.MakeBitMapTemp(a, 5)
		evaluateComparison(&vector, mask.Bytes(), physicalFilterData{
			dType: dtype.Float64T(), operation: test.operation, literal: MakeF64Scalar(0),
		})
		mask.RecalcNiN()
		for i, want := range test.want {
			if got := mask.IsSet(i); got != want {
				t.Fatalf("operation %d, row %d: got %t, expected %t", test.operation, i, got, want)
			}
		}
		mask.Release()
		vector.Release()
	}
}

func TestPhysicalPlanBindingFailures(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	vector := store.MakeStringVector(a, [][]byte{[]byte("x")}, nil)
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	schema := MakeSchema([]string{"value"}, []dtype.Type{dtype.StringT()})

	tests := []LogicalPlan{
		MakeScan(table, schema).Filter(MakeColumn("missing").Eq(MakeI32Scalar(1))),
		MakeScan(table, schema).Filter(MakeColumn("value").Eq(MakeI32Scalar(1))),
		MakeScan(table, schema).Project(MakeColumn("missing")),
	}
	for i, logical := range tests {
		if physical, err := MakePhysicalPlan(logical); err == nil || physical != nil {
			t.Fatalf("invalid plan %d was bound", i)
		}
	}
	table.Release()
}

func TestPhysicalPlanRejectsSemanticNarrowing(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	vector := store.MakeVector(a, 1, dtype.Int32T(), false)
	vector.I32s()[0] = 1
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	scan := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int32T()}))

	for _, literal := range []Scalar{MakeI64Scalar(1), MakeF64Scalar(1.5)} {
		if physical, err := MakePhysicalPlan(scan.Filter(MakeColumn("x").Eq(literal))); err == nil || physical != nil {
			t.Fatalf("bound narrowing comparison from %s", literal.Type())
		}
	}
	table.Release()
}

func TestPhysicalMultipleFiltersWithSelectionVector(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	vector := store.MakeVector(a, 10, dtype.Int64T(), false)
	for i := range vector.I64s() {
		vector.I64s()[i] = int64(i)
	}
	batch := store.MakeBatch([]store.Vector{vector})
	batch.SetSelection(store.MakeRowSelectionFromSelVec(store.MakeSelVecFromOffsets(a, 10, []uint32{9, 7, 5, 3, 1})))
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	logical := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()})).
		Filter(MakeColumn("x").Gt(MakeI64Scalar(2))).
		Filter(MakeColumn("x").Lt(MakeI64Scalar(8)))
	physical, err := MakePhysicalPlan(logical)
	table.Release()
	if err != nil {
		t.Fatalf("failed to bind multiple filters: %v", err)
	}
	physical.Execute(a, func(output *store.Batch) bool {
		bitmap, _ := output.Selection().AsBitMap()
		if output.ActiveLen() != 3 || !bitmap.IsSet(3) || !bitmap.IsSet(5) || !bitmap.IsSet(7) {
			t.Fatal("filters did not intersect the selection vector")
		}
		return true
	})
	physical.Release()
}

func TestPhysicalProjectThenFilterUsesProjectedSlots(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	x := store.MakeVector(a, 4, dtype.Int32T(), false)
	y := store.MakeVector(a, 4, dtype.Int64T(), false)
	copy(x.I32s(), []int32{10, 20, 30, 40})
	copy(y.I64s(), []int64{4, 3, 2, 1})
	batch := store.MakeBatch([]store.Vector{x, y})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	logical := MakeScan(table, MakeSchema([]string{"x", "y"}, []dtype.Type{dtype.Int32T(), dtype.Int64T()})).
		Project(MakeColumn("y"), MakeColumn("x")).
		Filter(MakeColumn("x").Ge(MakeI32Scalar(30))).
		Project(MakeColumn("x"))
	physical, err := MakePhysicalPlan(logical)
	table.Release()
	if err != nil {
		t.Fatalf("failed to bind projected filter: %v", err)
	}
	physical.Execute(a, func(output *store.Batch) bool {
		if output.NVectors() != 1 || output.ActiveLen() != 2 || output.VectorAt(0).I32s()[2] != 30 {
			t.Fatal("projected filter used the wrong physical slot")
		}
		return true
	})
	physical.Release()
}

func TestPhysicalPushesEmptyActiveBatch(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	vector := store.MakeVector(a, 3, dtype.Int64T(), true)
	vector.Validity().ClearAll()
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	logical := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()})).Filter(MakeColumn("x").Gt(MakeI64Scalar(0)))
	physical, err := MakePhysicalPlan(logical)
	table.Release()
	if err != nil {
		t.Fatalf("failed to bind all-null filter: %v", err)
	}
	var calls int
	physical.Execute(a, func(output *store.Batch) bool {
		calls++
		if output.Len() != 3 || output.ActiveLen() != 0 {
			t.Fatal("unexpected empty-active batch")
		}
		return true
	})
	if calls != 1 {
		t.Fatalf("empty-active callbacks: got %d, expected 1", calls)
	}
	physical.Release()
}

func TestPhysicalPushControlAndEmptySource(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16_384}, []int{16_384})
	x := store.MakeBatch([]store.Vector{store.MakeVector(a, 1, dtype.Int64T(), false)})
	y := store.MakeBatch([]store.Vector{store.MakeVector(a, 1, dtype.Int64T(), false)})
	table := store.MakeTable([]*store.Batch{x, y})
	x.Release()
	y.Release()
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()})))
	table.Release()
	if err != nil {
		t.Fatalf("failed to bind scan: %v", err)
	}
	var calls int
	if completed := physical.Execute(a, func(*store.Batch) bool {
		calls++
		return true
	}); !completed || calls != 2 {
		t.Fatalf("complete traversal: completed=%t, calls=%d", completed, calls)
	}
	calls = 0
	if completed := physical.Execute(a, func(*store.Batch) bool {
		calls++
		return false
	}); completed || calls != 1 {
		t.Fatalf("early stop: completed=%t, calls=%d", completed, calls)
	}
	physical.Release()

	empty := store.MakeEmptyTable([]dtype.Type{dtype.Int64T()})
	physical, err = MakePhysicalPlan(MakeScan(empty, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()})))
	empty.Release()
	if err != nil {
		t.Fatalf("failed to bind typed empty table: %v", err)
	}
	calls = 0
	if !physical.Execute(a, func(*store.Batch) bool { calls++; return true }) || calls != 0 {
		t.Fatalf("empty execution pushed %d batches", calls)
	}
	physical.Release()
}

func TestPhysicalBindingRejectsReleasedBorrow(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	batch := store.MakeBatch([]store.Vector{store.MakeVector(a, 1, dtype.Int64T(), false)})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	logical := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()}))
	table.Release()
	if physical, err := MakePhysicalPlan(logical); err == nil || physical != nil {
		t.Fatal("bound a scan after its borrowed table was released")
	}

	batch = store.MakeBatch([]store.Vector{store.MakeVector(a, 1, dtype.Int64T(), false)})
	table = store.MakeTable([]*store.Batch{batch})
	batch.Release()
	bound, err := BindLogicalPlan(MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()})))
	if err != nil {
		t.Fatalf("bind live source: %v", err)
	}
	table.Release()
	if physical, err := MakePhysicalPlanFromBound(bound); err == nil || physical != nil {
		t.Fatal("lowered a bound plan after its borrowed table was released")
	}
}

func TestPhysicalSinkPanicReleasesBatch(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	batch := store.MakeBatch([]store.Vector{store.MakeVector(a, 1, dtype.Int64T(), false)})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	physical, err := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()})))
	table.Release()
	if err != nil {
		t.Fatalf("failed to bind scan: %v", err)
	}
	data := physical.source.BatchAt(0).VectorAt(0).Data()
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		physical.Execute(a, func(*store.Batch) bool { panic("sink") })
	}()
	if recovered == nil {
		t.Fatal("sink panic did not propagate")
	}
	if got := data.RefCount(); got != 1 {
		t.Fatalf("source references after sink panic: got %d, expected 1", got)
	}
	physical.Release()
}

func TestPhysicalPlanConcurrentExecution(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	vector := store.MakeVector(a, 64, dtype.Int64T(), false)
	for i := range vector.I64s() {
		vector.I64s()[i] = int64(i)
	}
	batch := store.MakeBatch([]store.Vector{vector})
	table := store.MakeTable([]*store.Batch{batch})
	batch.Release()
	logical := MakeScan(table, MakeSchema([]string{"x"}, []dtype.Type{dtype.Int64T()})).Filter(MakeColumn("x").Ge(MakeI64Scalar(32)))
	physical, err := MakePhysicalPlan(logical)
	table.Release()
	if err != nil {
		t.Fatalf("failed to bind physical plan: %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var calls int
			completed := physical.Execute(a, func(output *store.Batch) bool {
				calls++
				if output.ActiveLen() != 32 {
					t.Errorf("active rows: got %d, expected 32", output.ActiveLen())
				}
				bitmap, _ := output.Selection().AsBitMap()
				if bitmap.IsSet(31) || !bitmap.IsSet(32) || !bitmap.IsSet(63) {
					t.Error("unexpected concurrent selection bits")
				}
				return true
			})
			if !completed || calls != 1 {
				t.Errorf("execution result: completed=%t, calls=%d", completed, calls)
			}
		}()
	}
	wg.Wait()
	physical.Release()
}

func TestNilPhysicalPlanIsClean(t *testing.T) {
	var physical *PhysicalPlan
	if physical.Schema().Valid() || physical.Execute(nil, nil) {
		t.Fatal("nil physical plan was valid or executable")
	}
	physical.Release()
}
