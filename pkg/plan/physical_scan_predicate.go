package plan

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
)

func makeParquetScanPredicates(program physicalExprProgram) ([]parquet.ScanPredicate, bool) {
	if len(program.roots) != 1 {
		return nil, false
	}
	var predicates []parquet.ScanPredicate
	var visit func(int) bool
	visit = func(at int) bool {
		root := program.nodes[at]
		if root.kind == exprBinary && root.operation == exprOpAnd {
			return visit(root.children[0]) && visit(root.children[1])
		}
		if root.kind == exprColumn && root.dType.ID() == dtype.BOOLT {
			predicates = append(predicates, parquet.ScanPredicate{Column: root.column, Op: parquet.PruneEqual, Integer: 1})
			return true
		}
		if root.kind == exprUnary && root.operation == exprOpNot {
			column := program.nodes[root.children[0]]
			if column.kind == exprColumn && column.dType.ID() == dtype.BOOLT {
				predicates = append(predicates, parquet.ScanPredicate{Column: column.column, Op: parquet.PruneEqual})
				return true
			}
			return false
		}
		if root.kind != exprBinary || root.childCount != 2 {
			return false
		}
		column, literal := program.nodes[root.children[0]], program.nodes[root.children[1]]
		flipped := column.kind == exprLiteral && literal.kind == exprColumn
		if flipped {
			column, literal = literal, column
		}
		if column.kind != exprColumn || literal.kind != exprLiteral || literal.literal.IsNull() || !column.dType.Equal(literal.dType) {
			return false
		}
		p := parquet.ScanPredicate{Column: column.column}
		switch literal.dType.ID() {
		case dtype.INT32T, dtype.DATET:
			p.Integer = int64(literal.literal.i32())
		case dtype.INT64T, dtype.TIMESTAMPTZT:
			p.Integer = literal.literal.i64()
		case dtype.BOOLT:
			if root.operation != exprOpEQ && root.operation != exprOpNE {
				return false
			}
			if literal.literal.boolean() {
				p.Integer = 1
			}
		case dtype.STRT:
			p.Text = literal.literal.stringValue()
		default:
			return false
		}
		switch root.operation {
		case exprOpEQ:
			p.Op = parquet.PruneEqual
		case exprOpNE:
			p.Op = parquet.PruneNotEqual
		case exprOpLT:
			if flipped {
				p.Op = parquet.PruneGreater
			} else {
				p.Op = parquet.PruneLess
			}
		case exprOpLE:
			if flipped {
				p.Op = parquet.PruneGreaterEqual
			} else {
				p.Op = parquet.PruneLessEqual
			}
		case exprOpGT:
			if flipped {
				p.Op = parquet.PruneLess
			} else {
				p.Op = parquet.PruneGreater
			}
		case exprOpGE:
			if flipped {
				p.Op = parquet.PruneLessEqual
			} else {
				p.Op = parquet.PruneGreaterEqual
			}
		default:
			return false
		}
		predicates = append(predicates, p)
		return true
	}
	if !visit(program.roots[0]) {
		return nil, false
	}
	return predicates, true
}
