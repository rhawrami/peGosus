package plan

import (
	"github.com/rhawrami/peGosus/pkg/parse"
	"github.com/rhawrami/peGosus/pkg/store"
)

type logicalOp uint8

const (
	logicalInvalid logicalOp = iota
	logicalScan
	logicalFilter
	logicalProject
	logicalLimit
	logicalAggregate
	logicalDistinct
	logicalSort
	logicalAlias
	logicalJoin
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
	right       *logicalNode
	table       *store.Table
	csvPath     string
	csvOptions  parse.CSVOptions
	schema      Schema
	predicate   Expr
	projections []Expr
	limit       int64
	offset      int64
	aggregates  []Aggregate
	groupKeys   []Expr
	order       []OrderKey
	alias       string
	joinKind    JoinKind
	leftKeys    []Expr
	rightKeys   []Expr
	residual    Expr
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

// Limit returns a relation containing at most count active rows, after an
// optional offset. Negative values and multiple offsets make the plan invalid.
func (p LogicalPlan) Limit(count int64, offset ...int64) LogicalPlan {
	if !p.Valid() || count < 0 || len(offset) > 1 {
		return LogicalPlan{}
	}
	var skip int64
	if len(offset) == 1 {
		skip = offset[0]
	}
	if skip < 0 {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalLimit, input: p.root, limit: count, offset: skip}}
}

// Aggregate returns one global row of aggregate results.
func (p LogicalPlan) Aggregate(aggregates ...Aggregate) LogicalPlan {
	if !p.Valid() || len(aggregates) == 0 {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalAggregate, input: p.root, aggregates: append([]Aggregate(nil), aggregates...)}}
}

// GroupBy returns aggregate rows grouped by the supplied expressions.
func (p LogicalPlan) GroupBy(keys []Expr, aggregates ...Aggregate) LogicalPlan {
	if !p.Valid() || len(keys) == 0 || len(aggregates) == 0 {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalAggregate, input: p.root, groupKeys: append([]Expr(nil), keys...), aggregates: append([]Aggregate(nil), aggregates...)}}
}

// Distinct returns one row for each distinct tuple of active input values.
func (p LogicalPlan) Distinct() LogicalPlan {
	if !p.Valid() {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalDistinct, input: p.root}}
}

// OrderBy returns a relation ordered by the given keys.
func (p LogicalPlan) OrderBy(keys ...OrderKey) LogicalPlan {
	if !p.Valid() || len(keys) == 0 {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalSort, input: p.root, order: append([]OrderKey(nil), keys...)}}
}

// As assigns a source alias for qualified field references.
func (p LogicalPlan) As(alias string) LogicalPlan {
	if !p.Valid() || alias == "" {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalAlias, input: p.root, alias: alias}}
}
