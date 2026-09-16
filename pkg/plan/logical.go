package plan

import "github.com/rhawrami/peGosus/pkg/store"

type logicalOp uint8

const (
	logicalInvalid logicalOp = iota
	logicalScan
	logicalFilter
	logicalProject
)

// MakeScan returns a logical in-memory table scan. The plan borrows `table`.
func MakeScan(table *store.Table, schema Schema) LogicalPlan {
	if !table.Valid() || !schema.Valid() || table.NColumns() != schema.Len() {
		return LogicalPlan{}
	}
	for i := range schema.Len() {
		if !table.TypeAt(i).Equal(schema.FieldAt(i).Type()) {
			return LogicalPlan{}
		}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalScan, table: table, schema: schema}}
}

// LogicalPlan is an immutable logical relation rooted at one node.
type LogicalPlan struct {
	root *logicalNode
}

type logicalNode struct {
	operation   logicalOp
	input       *logicalNode
	table       *store.Table
	schema      Schema
	predicate   Expr
	projections []Expr
}

// Valid returns whether the logical plan has a root.
func (p LogicalPlan) Valid() bool { return p.root != nil }

// Filter returns a plan with `predicate` above the current root.
func (p LogicalPlan) Filter(predicate Expr) LogicalPlan {
	if p.root == nil || !predicate.Valid() {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalFilter, input: p.root, predicate: predicate}}
}

// Project returns a plan with column projections above the current root.
func (p LogicalPlan) Project(expressions ...Expr) LogicalPlan {
	if p.root == nil || len(expressions) == 0 {
		return LogicalPlan{}
	}
	projected := append([]Expr(nil), expressions...)
	for _, expression := range projected {
		if !expression.Valid() {
			return LogicalPlan{}
		}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalProject, input: p.root, projections: projected}}
}
