package plan

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
)

func makeParquetPruningPredicates(source *scanSource) []parquet.PruningPredicate {
	var predicates []parquet.PruningPredicate
	for _, p := range source.parquetPredicates {
		switch source.schema.FieldAt(p.Column).Type().ID() {
		case dtype.INT32T, dtype.INT64T, dtype.DATET, dtype.TIMESTAMPTZT:
			predicates = append(predicates, parquet.PruningPredicate{Column: p.Column, Op: p.Op, Literal: p.Integer})
		}
	}
	for i, program := range source.filters {
		if i < len(source.parquetFilterHandled) && source.parquetFilterHandled[i] {
			continue
		}
		if len(program.roots) != 1 {
			continue
		}
		root := program.nodes[program.roots[0]]
		if root.kind != exprBinary || root.childCount != 2 {
			continue
		}
		column := program.nodes[root.children[0]]
		literal := program.nodes[root.children[1]]
		flipped := false
		if column.kind == exprLiteral && literal.kind == exprColumn {
			column, literal = literal, column
			flipped = true
		}
		if column.kind != exprColumn || literal.kind != exprLiteral || literal.literal.IsNull() {
			continue
		}
		original := column.column
		if len(source.projection) != 0 {
			if original < 0 || original >= len(source.projection) {
				continue
			}
			original = source.projection[original]
		}
		if original < 0 || original >= source.schema.Len() || !source.schema.FieldAt(original).Type().Equal(literal.dType) {
			continue
		}
		predicate := parquet.PruningPredicate{Column: original}
		switch literal.dType.ID() {
		case dtype.INT32T, dtype.DATET:
			predicate.Literal = int64(literal.literal.i32())
		case dtype.INT64T, dtype.TIMESTAMPTZT:
			predicate.Literal = literal.literal.i64()
		default:
			continue
		}
		switch root.operation {
		case exprOpEQ:
			predicate.Op = parquet.PruneEqual
		case exprOpNE:
			predicate.Op = parquet.PruneNotEqual
		case exprOpLT:
			if flipped {
				predicate.Op = parquet.PruneGreater
			} else {
				predicate.Op = parquet.PruneLess
			}
		case exprOpLE:
			if flipped {
				predicate.Op = parquet.PruneGreaterEqual
			} else {
				predicate.Op = parquet.PruneLessEqual
			}
		case exprOpGT:
			if flipped {
				predicate.Op = parquet.PruneLess
			} else {
				predicate.Op = parquet.PruneGreater
			}
		case exprOpGE:
			if flipped {
				predicate.Op = parquet.PruneLessEqual
			} else {
				predicate.Op = parquet.PruneGreaterEqual
			}
		default:
			continue
		}
		predicates = append(predicates, predicate)
	}
	return predicates
}
