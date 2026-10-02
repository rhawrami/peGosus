package plan

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
)

func splitParquetScanPredicates(program physicalExprProgram) ([]parquet.ScanPredicate, physicalExprProgram, bool) {
	if len(program.roots) != 1 {
		return nil, program, false
	}
	var predicates []parquet.ScanPredicate
	var residual []int
	var visit func(int)
	visit = func(at int) {
		node := program.nodes[at]
		if node.kind == exprBinary && node.operation == exprOpAnd {
			visit(node.children[0])
			visit(node.children[1])
			return
		}
		part := program
		part.roots = []int{at}
		if exact, ok := makeParquetScanPredicates(part); ok {
			predicates = append(predicates, exact...)
		} else {
			residual = append(residual, at)
		}
	}
	visit(program.roots[0])
	if len(predicates) == 0 || len(residual) == 0 {
		return predicates, program, len(residual) == 0
	}
	remaining := physicalExprProgram{outputRoots: program.outputRoots}
	mapping := make(map[int]int)
	var copyNode func(int) int
	copyNode = func(at int) int {
		if index, exists := mapping[at]; exists {
			return index
		}
		node := program.nodes[at]
		for i := range int(node.childCount) {
			node.children[i] = copyNode(node.children[i])
		}
		index := len(remaining.nodes)
		mapping[at] = index
		remaining.nodes = append(remaining.nodes, node)
		remaining.materialize = append(remaining.materialize, program.materialize[at])
		return index
	}
	root := copyNode(residual[0])
	for _, at := range residual[1:] {
		right := copyNode(at)
		remaining.nodes = append(remaining.nodes, physicalExprNode{kind: exprBinary, operation: exprOpAnd, dType: dtype.BoolT(), nullable: remaining.nodes[root].nullable || remaining.nodes[right].nullable, children: [3]int{root, right}, childCount: 2})
		remaining.materialize = append(remaining.materialize, true)
		root = len(remaining.nodes) - 1
	}
	remaining.roots = []int{root}
	remaining.materialize[root] = true
	return predicates, remaining, false
}

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
