package plan

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParquetScanCompiledPredicates(t *testing.T) {
	encoded, err := os.ReadFile("../io/parquet/testdata/duckdb-predicates.b64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "predicates.parquet")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	schema := MakeSchemaWithNullability([]string{"seq", "id", "active", "txt"}, []dtype.Type{dtype.Int32T(), dtype.Int32T(), dtype.BoolT(), dtype.StringT()}, []bool{true, true, true, true})
	cases := []struct {
		expr       Expr
		match      func(int) bool
		projection []int
		compiled   bool
	}{
		{MakeColumn("seq").Ge(0).And(MakeColumn("active")), func(i int) bool { return i%19 != 0 && i%3 == 0 }, []int{}, true},
		{MakeColumn("active").Not(), func(i int) bool { return i%19 != 0 && i%3 != 0 }, []int{}, true},
		{MakeColumn("txt").Eq("north"), func(i int) bool { return i%23 != 0 && i%7 == 1 }, []int{}, true},
		{MakeColumn("txt").Eq("a\x00b").And(MakeColumn("id").Ge(-7)), func(i int) bool { return i%23 != 0 && i%7 == 2 && i%17 != 0 && i*48271%101-50 >= -7 }, []int{}, true},
		{MakeColumn("id").Add(1).Ge(0), func(i int) bool { return i%17 != 0 && i*48271%101-50+1 >= 0 }, []int{1}, false},
		{MakeColumn("id").Ge(100), func(int) bool { return false }, []int{}, true},
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	for _, tc := range cases {
		query := MakeParquetScan(path, schema, parquet.ParquetOptions{BatchSize: 257}).Filter(tc.expr).Project(MakeColumn("txt"), MakeColumn("id"), MakeColumn("seq"), MakeColumn("active")).Aggregate(MakeCountStar())
		p, err := MakePhysicalPlan(query)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.source.parquetFilterHandled) != 1 || p.source.parquetFilterHandled[0] != tc.compiled {
			t.Fatalf("wrong compiled predicates: %v", p.source.parquetFilterHandled)
		}
		if len(p.source.projection) != len(tc.projection) {
			t.Fatalf("predicate dependencies retained: %v want %v", p.source.projection, tc.projection)
		}
		for i, col := range tc.projection {
			if p.source.projection[i] != col {
				t.Fatal("wrong fallback projection")
			}
		}
		expected := int64(0)
		for i := range 4097 {
			if tc.match(i) {
				expected++
			}
		}
		for _, workers := range []int{1, 4} {
			for iteration := range 2 {
				calls := 0
				result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: 8 << 20, Workers: workers}, func(b *store.Batch) bool {
					calls++
					if b.Len() != 1 || b.VectorAt(0).I64s()[0] != expected {
						t.Fatalf("workers=%d iteration=%d got %v want %d", workers, iteration, b.VectorAt(0).I64s(), expected)
					}
					return true
				})
				if result.Code() != ExecutionCompleted || calls != 1 {
					t.Fatalf("query %v/%v calls=%d", result.Code(), result.Err(), calls)
				}
			}
		}
		p.Release()
		if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
			t.Fatalf("query storage leaked %+v", usage)
		}
	}
}

func TestParquetScanPredicateCompilerRejectsPartialConjunction(t *testing.T) {
	column := physicalExprNode{kind: exprColumn, column: 0, dType: dtype.Int32T()}
	literal := physicalExprNode{kind: exprLiteral, dType: dtype.Int32T(), literal: MakeI32Scalar(1)}
	program := physicalExprProgram{roots: []int{4}, nodes: []physicalExprNode{column, literal, {kind: exprBinary, operation: exprOpGE, childCount: 2, children: [3]int{0, 1}}, {kind: exprBinary, operation: exprOpAdd, childCount: 2, children: [3]int{0, 1}}, {kind: exprBinary, operation: exprOpAnd, childCount: 2, children: [3]int{2, 3}}}}
	if predicates, ok := makeParquetScanPredicates(program); ok || predicates != nil {
		t.Fatal("partly handled conjunction must retain executor evaluation")
	}
	program.roots = []int{2}
	program.nodes[1].literal = MakeNullScalar(dtype.Int32T())
	if _, ok := makeParquetScanPredicates(program); ok {
		t.Fatal("compiled null comparison")
	}
}

func TestAggregateProjectionFoldingPreservesIDsAndMappings(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	key := store.MakeVector(a, 5, dtype.Int32T(), true)
	copy(key.I32s(), []int32{2, 1, 2, 1, 9})
	key.Validity().Clear(4)
	value := store.MakeVector(a, 5, dtype.Int32T(), true)
	copy(value.I32s(), []int32{7, -3, 11, 17, 23})
	value.Validity().Clear(1)
	unused := store.MakeVector(a, 5, dtype.Int64T(), false)
	b := store.MakeBatch([]store.Vector{key, value, unused})
	selection := store.MakeBitMap(a, 5)
	selection.SetAll()
	selection.Clear(3)
	b.SetSelection(store.MakeRowSelectionFromBitMap(selection))
	table := store.MakeTable([]*store.Batch{b})
	b.Release()
	schema := MakeSchemaWithNullability([]string{"key", "value", "unused"}, []dtype.Type{dtype.Int32T(), dtype.Int32T(), dtype.Int64T()}, []bool{true, true, false})
	query := MakeScan(table, schema).Project(MakeColumn("unused"), MakeColumn("value").Alias("v"), MakeColumn("key").Alias("k"), MakeColumn("value").Alias("copy")).Project(MakeColumn("k"), MakeColumn("copy").Alias("amount"), MakeColumn("v")).GroupBy([]Expr{MakeColumn("k")}, MakeSum(MakeColumn("amount")).Alias("total"), MakeCount(MakeColumn("v")).Alias("count"), MakeCountStar().Alias("rows"))
	bound, err := BindLogicalPlan(query)
	if err != nil {
		t.Fatal(err)
	}
	p, err := MakePhysicalPlanFromBound(bound)
	if err != nil {
		t.Fatal(err)
	}
	table.Release()
	if len(p.steps) != 1 || p.steps[0].operation != physicalAggregate {
		t.Fatal("plain aggregate input projections were not folded")
	}
	for i := range bound.Schema().Len() {
		if p.Schema().FieldAt(i).ID() != bound.Schema().FieldAt(i).ID() || p.Schema().FieldAt(i).Name() != bound.Schema().FieldAt(i).Name() {
			t.Fatal("changed bound output identity")
		}
	}
	rows := 0
	if !p.Execute(a, func(output *store.Batch) bool {
		for row := range output.Len() {
			rows++
			null := !output.VectorAt(0).Validity().IsSet(row)
			k := output.VectorAt(0).I32s()[row]
			wantSum, wantCount, wantRows := int64(18), int64(2), int64(2)
			if null {
				wantSum, wantCount, wantRows = 23, 1, 1
			} else if k == 1 {
				wantSum, wantCount, wantRows = 0, 0, 1
			} else if k != 2 {
				t.Fatal("wrong grouped key")
			}
			if output.VectorAt(1).Validity().IsSet(row) != (wantCount != 0) || wantCount != 0 && output.VectorAt(1).I64s()[row] != wantSum || output.VectorAt(2).I64s()[row] != wantCount || output.VectorAt(3).I64s()[row] != wantRows {
				t.Fatal("projection folding changed nullable grouped aggregates")
			}
		}
		return true
	}) || rows != 3 {
		t.Fatal("wrong aggregate output")
	}
	p.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leak %+v", usage)
	}
}
