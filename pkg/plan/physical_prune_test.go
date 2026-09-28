package plan

import (
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
)

func TestParquetPruningPredicateCompilation(t *testing.T) {
	schema := MakeSchema([]string{"unused", "id"}, []dtype.Type{dtype.StringT(), dtype.Int32T()})
	program := physicalExprProgram{
		roots: []int{2},
		nodes: []physicalExprNode{
			{kind: exprColumn, column: 0, dType: dtype.Int32T()},
			{kind: exprLiteral, dType: dtype.Int32T(), literal: MakeI32Scalar(-7)},
			{kind: exprBinary, operation: exprOpGE, children: [3]int{0, 1}, childCount: 2},
		},
	}
	source := &scanSource{schema: schema, projection: []int{1}, filters: []physicalExprProgram{program}}
	predicates := makeParquetPruningPredicates(source)
	if len(predicates) != 1 || predicates[0] != (parquet.PruningPredicate{Column: 1, Op: parquet.PruneGreaterEqual, Literal: -7}) {
		t.Fatalf("wrong bound predicate: %v", predicates)
	}
	source.filters[0].nodes[2].children = [3]int{1, 0}
	source.filters[0].nodes[2].operation = exprOpLT
	predicates = makeParquetPruningPredicates(source)
	if len(predicates) != 1 || predicates[0].Op != parquet.PruneGreater {
		t.Fatalf("scalar-left predicate reversed incorrectly: %v", predicates)
	}
	source.filters[0].nodes[2].operation = exprOpAnd
	if len(makeParquetPruningPredicates(source)) != 0 {
		t.Fatal("treated composite expression as a simple comparison")
	}
	source.filters[0].nodes[2].operation = exprOpLT
	source.filters[0].nodes[0].kind = exprCast
	if len(makeParquetPruningPredicates(source)) != 0 {
		t.Fatal("pruned a cast column using incompatible source statistics")
	}
}
