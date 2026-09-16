package plan

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/store"
)

type boundExprID uint32

type boundExpr struct {
	kind       exprKind
	operation  exprOp
	dType      dtype.Type
	nullable   bool
	children   [3]boundExprID
	childCount uint8
	field      FieldID
	literal    Scalar
	target     dtype.Type
	implicit   bool
}

type boundLogicalNode struct {
	operation   logicalOp
	input       *boundLogicalNode
	table       *store.Table
	schema      Schema
	predicate   boundExprID
	projections []boundExprID
}

// BoundPlan is an immutable logical plan with resolved fields, exact types,
// nullability, and explicit casts. It borrows scan sources until physical
// lowering retains them.
type BoundPlan struct {
	root        *boundLogicalNode
	expressions []boundExpr
}

// Valid returns whether the bound plan has a root.
func (p *BoundPlan) Valid() bool { return p != nil && p.root != nil }

// Schema returns the bound output schema.
func (p *BoundPlan) Schema() Schema {
	if p == nil || p.root == nil {
		return Schema{}
	}
	return p.root.schema
}

func (p *BoundPlan) expression(id boundExprID) *boundExpr {
	if p == nil || id == 0 || int(id) >= len(p.expressions) {
		return nil
	}
	return &p.expressions[id]
}
