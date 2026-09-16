package plan

import "github.com/rhawrami/peGosus/pkg/dtype"

type exprKind uint8

const (
	exprInvalid exprKind = iota
	exprColumn
	exprLiteral
	exprUnary
	exprBinary
	exprTernary
	exprCast
)

type exprOp uint8

const (
	exprOpInvalid exprOp = iota
	exprOpEQ
	exprOpNE
	exprOpLT
	exprOpLE
	exprOpGT
	exprOpGE
	exprOpAdd
	exprOpSub
	exprOpMul
	exprOpDiv
	exprOpAnd
	exprOpOr
	exprOpNot
	exprOpAbs
	exprOpNeg
	exprOpSqrt
	exprOpIsNull
	exprOpIsNotNull
	exprOpBetween
	exprOpNotBetween
	exprOpClip
	exprOpCoalesce
	exprOpCase
)

// MakeColumn returns an unresolved column expression.
func MakeColumn(name string) Expr {
	return Expr{node: &exprNode{kind: exprColumn, column: name}}
}

// MakeLiteral returns an unresolved literal expression from a supported Go
// scalar value.
func MakeLiteral(value any) Expr {
	return Expr{node: &exprNode{kind: exprLiteral, literal: makeLiteral(value)}}
}

// MakeCase returns a conditional expression.
func MakeCase(condition Expr, whenTrue, whenFalse any) Expr {
	return condition.ternary(exprOpCase, whenTrue, whenFalse)
}

// Expr is an immutable unresolved logical expression.
type Expr struct {
	node  *exprNode
	alias string
}

type exprNode struct {
	kind      exprKind
	operation exprOp
	children  [3]*exprNode
	column    string
	literal   literal
	cast      dtype.Type
}

// Valid returns whether the expression has a valid unresolved root.
func (e Expr) Valid() bool { return e.node != nil && e.node.kind != exprInvalid }

// Alias returns the expression with an output name.
func (e Expr) Alias(name string) Expr {
	if !e.Valid() || name == "" {
		return Expr{}
	}
	return Expr{node: e.node, alias: name}
}

// Cast returns an explicit cast expression.
func (e Expr) Cast(t dtype.Type) Expr {
	if !e.Valid() || !t.Valid() || t.ID() == dtype.NULLT {
		return Expr{}
	}
	return Expr{node: &exprNode{kind: exprCast, children: [3]*exprNode{e.node}, cast: t}}
}

// Eq returns an equality comparison.
func (e Expr) Eq(value any) Expr { return e.binary(exprOpEQ, value) }

// Ne returns an inequality comparison.
func (e Expr) Ne(value any) Expr { return e.binary(exprOpNE, value) }

// Lt returns a less-than comparison.
func (e Expr) Lt(value any) Expr { return e.binary(exprOpLT, value) }

// Le returns a less-than-or-equal comparison.
func (e Expr) Le(value any) Expr { return e.binary(exprOpLE, value) }

// Gt returns a greater-than comparison.
func (e Expr) Gt(value any) Expr { return e.binary(exprOpGT, value) }

// Ge returns a greater-than-or-equal comparison.
func (e Expr) Ge(value any) Expr { return e.binary(exprOpGE, value) }

// Add returns an addition expression.
func (e Expr) Add(value any) Expr { return e.binary(exprOpAdd, value) }

// Sub returns a subtraction expression.
func (e Expr) Sub(value any) Expr { return e.binary(exprOpSub, value) }

// Mul returns a multiplication expression.
func (e Expr) Mul(value any) Expr { return e.binary(exprOpMul, value) }

// Div returns a division expression.
func (e Expr) Div(value any) Expr { return e.binary(exprOpDiv, value) }

// And returns a three-valued boolean conjunction.
func (e Expr) And(value any) Expr { return e.binary(exprOpAnd, value) }

// Or returns a three-valued boolean disjunction.
func (e Expr) Or(value any) Expr { return e.binary(exprOpOr, value) }

// Coalesce returns the first non-null value from the receiver and `values`.
func (e Expr) Coalesce(values ...any) Expr {
	if len(values) == 0 {
		return e
	}
	right := makeOperand(values[len(values)-1])
	for i := len(values) - 2; i >= 0; i-- {
		left := makeOperand(values[i])
		if left.node == nil || right.node == nil {
			return Expr{}
		}
		right = Expr{node: &exprNode{
			kind: exprBinary, operation: exprOpCoalesce, children: [3]*exprNode{left.node, right.node},
		}}
	}
	if e.node == nil || right.node == nil {
		return Expr{}
	}
	return Expr{node: &exprNode{
		kind: exprBinary, operation: exprOpCoalesce, children: [3]*exprNode{e.node, right.node},
	}}
}

// Not returns a three-valued boolean negation.
func (e Expr) Not() Expr { return e.unary(exprOpNot) }

// Abs returns an absolute-value expression.
func (e Expr) Abs() Expr { return e.unary(exprOpAbs) }

// Neg returns an arithmetic-negation expression.
func (e Expr) Neg() Expr { return e.unary(exprOpNeg) }

// Sqrt returns a square-root expression.
func (e Expr) Sqrt() Expr { return e.unary(exprOpSqrt) }

// IsNull returns a null-test expression.
func (e Expr) IsNull() Expr { return e.unary(exprOpIsNull) }

// IsNotNull returns a non-null-test expression.
func (e Expr) IsNotNull() Expr { return e.unary(exprOpIsNotNull) }

// Between returns an inclusive range comparison.
func (e Expr) Between(lower, upper any) Expr { return e.ternary(exprOpBetween, lower, upper) }

// NotBetween returns an exclusive outside-range comparison.
func (e Expr) NotBetween(lower, upper any) Expr { return e.ternary(exprOpNotBetween, lower, upper) }

// Clip returns a numeric clipping expression.
func (e Expr) Clip(lower, upper any) Expr { return e.ternary(exprOpClip, lower, upper) }

func (e Expr) unary(operation exprOp) Expr {
	if !e.Valid() {
		return Expr{}
	}
	return Expr{node: &exprNode{kind: exprUnary, operation: operation, children: [3]*exprNode{e.node}}}
}

func (e Expr) binary(operation exprOp, value any) Expr {
	right := makeOperand(value)
	if !e.Valid() || !right.Valid() {
		return Expr{}
	}
	return Expr{node: &exprNode{kind: exprBinary, operation: operation, children: [3]*exprNode{e.node, right.node}}}
}

func (e Expr) ternary(operation exprOp, lower, upper any) Expr {
	left := makeOperand(lower)
	right := makeOperand(upper)
	if !e.Valid() || !left.Valid() || !right.Valid() {
		return Expr{}
	}
	return Expr{node: &exprNode{kind: exprTernary, operation: operation, children: [3]*exprNode{e.node, left.node, right.node}}}
}

func makeOperand(value any) Expr {
	if expression, ok := value.(Expr); ok {
		return expression
	}
	return MakeLiteral(value)
}
