package peg

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/plan"
)

type aggregateKind uint8

const (
	aggregateNone aggregateKind = iota
	aggregateCountStar
	aggregateCount
	aggregateSum
	aggregateAvg
	aggregateMin
	aggregateMax
)

// C returns an unresolved reference to a named column.
func C(name string) Expr {
	if name == "" {
		return Expr{}
	}
	return Expr{inner: plan.MakeColumn(name), source: name}
}

// Lit returns an unresolved literal expression.
func Lit(value any) Expr { return Expr{inner: plan.MakeLiteral(value)} }

// Null returns a typed NULL literal.
func Null(kind Kind) Expr {
	t := dtype.MakeType(kind)
	if !t.Valid() || t.ID() == dtype.NULLT {
		return Expr{}
	}
	return Lit(plan.MakeNullScalar(t))
}

// DateDays returns a DATE literal measured in days since the Unix epoch.
func DateDays(days int32) Expr { return Lit(plan.MakeDateScalar(days)) }

// TimestampMicros returns a TIMESTAMPTZ literal measured in UTC microseconds.
func TimestampMicros(micros int64) Expr { return Lit(plan.MakeTimestampTZScalar(micros)) }

// Case returns a conditional expression; a NULL condition takes the false branch.
func Case(condition Expr, whenTrue, whenFalse any) Expr {
	if condition.aggregate != aggregateNone || !condition.Valid() {
		return Expr{}
	}
	return Expr{inner: plan.MakeCase(condition.inner, operand(whenTrue), operand(whenFalse))}
}

// CountStar returns a COUNT(*) aggregate expression for Agg.
func CountStar() Expr { return Expr{aggregate: aggregateCountStar} }

// Expr is an immutable, lazy column or aggregate expression.
type Expr struct {
	inner     plan.Expr
	source    string
	alias     string
	aggregate aggregateKind
	distinct  bool
}

// Valid reports whether the expression can be used in its intended context.
func (e Expr) Valid() bool { return e.aggregate == aggregateCountStar || e.inner.Valid() }

// Alias names the output of a column or aggregate expression.
func (e Expr) Alias(name string) Expr {
	if !e.Valid() || name == "" {
		return Expr{}
	}
	e.alias = name
	if e.aggregate == aggregateNone {
		e.inner = e.inner.Alias(name)
	}
	return e
}

func (e Expr) name() string {
	if e.alias != "" {
		return e.alias
	}
	return e.source
}

func operand(value any) any {
	if expression, ok := value.(Expr); ok {
		if expression.aggregate != aggregateNone {
			return plan.Expr{}
		}
		return expression.inner
	}
	return value
}

func (e Expr) binary(value any, makeExpr func(plan.Expr, any) plan.Expr) Expr {
	if e.aggregate != aggregateNone || !e.Valid() {
		return Expr{}
	}
	return Expr{inner: makeExpr(e.inner, operand(value)), source: e.source}
}

func (e Expr) ternary(lower, upper any, makeExpr func(plan.Expr, any, any) plan.Expr) Expr {
	if e.aggregate != aggregateNone || !e.Valid() {
		return Expr{}
	}
	return Expr{inner: makeExpr(e.inner, operand(lower), operand(upper)), source: e.source}
}

// Cast explicitly converts a numeric expression to a numeric kind.
func (e Expr) Cast(kind Kind) Expr {
	if e.aggregate != aggregateNone || !e.Valid() {
		return Expr{}
	}
	return Expr{inner: e.inner.Cast(dtype.MakeType(kind)), source: e.source}
}

// Eq compares the expression with a scalar or another expression.
func (e Expr) Eq(value any) Expr { return e.binary(value, plan.Expr.Eq) }

// Ne compares for inequality.
func (e Expr) Ne(value any) Expr { return e.binary(value, plan.Expr.Ne) }

// Lt compares for less-than.
func (e Expr) Lt(value any) Expr { return e.binary(value, plan.Expr.Lt) }

// Le compares for less-than-or-equal.
func (e Expr) Le(value any) Expr { return e.binary(value, plan.Expr.Le) }

// Gt compares for greater-than.
func (e Expr) Gt(value any) Expr { return e.binary(value, plan.Expr.Gt) }

// Ge compares for greater-than-or-equal.
func (e Expr) Ge(value any) Expr { return e.binary(value, plan.Expr.Ge) }

// Add adds a scalar or expression.
func (e Expr) Add(value any) Expr { return e.binary(value, plan.Expr.Add) }

// Sub subtracts a scalar or expression.
func (e Expr) Sub(value any) Expr { return e.binary(value, plan.Expr.Sub) }

// Mul multiplies by a scalar or expression.
func (e Expr) Mul(value any) Expr { return e.binary(value, plan.Expr.Mul) }

// Sq squares the expression, equivalent to multiplying it by itself.
func (e Expr) Sq() Expr {
	if e.aggregate != aggregateNone || !e.Valid() {
		return Expr{}
	}
	return Expr{inner: e.inner.Sq(), source: e.source}
}

// Div divides by a scalar or expression.
func (e Expr) Div(value any) Expr { return e.binary(value, plan.Expr.Div) }

// And combines nullable boolean expressions.
func (e Expr) And(value any) Expr { return e.binary(value, plan.Expr.And) }

// Or combines nullable boolean expressions.
func (e Expr) Or(value any) Expr { return e.binary(value, plan.Expr.Or) }

// Concat concatenates strings.
func (e Expr) Concat(value any) Expr { return e.binary(value, plan.Expr.Concat) }

// Like matches a SQL LIKE pattern.
func (e Expr) Like(value any) Expr { return e.binary(value, plan.Expr.Like) }

// Contains matches a byte sequence.
func (e Expr) Contains(value any) Expr { return e.binary(value, plan.Expr.Contains) }

// Between tests an inclusive range.
func (e Expr) Between(lower, upper any) Expr { return e.ternary(lower, upper, plan.Expr.Between) }

// NotBetween tests values outside an inclusive range.
func (e Expr) NotBetween(lower, upper any) Expr {
	return e.ternary(lower, upper, plan.Expr.NotBetween)
}

// Clip clamps a numeric expression to the given bounds.
func (e Expr) Clip(lower, upper any) Expr { return e.ternary(lower, upper, plan.Expr.Clip) }

// Coalesce returns the first non-null value.
func (e Expr) Coalesce(values ...any) Expr {
	if e.aggregate != aggregateNone || !e.Valid() {
		return Expr{}
	}
	converted := make([]any, len(values))
	for i, value := range values {
		converted[i] = operand(value)
	}
	return Expr{inner: e.inner.Coalesce(converted...), source: e.source}
}

// Slice extracts a zero-based byte range with an exclusive stop.
func (e Expr) Slice(start, stop any) Expr { return e.ternary(start, stop, plan.Expr.Slice) }

// Not negates a nullable boolean expression.
func (e Expr) Not() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.Not(), source: e.source}
}

// IsNull tests whether an expression is null.
func (e Expr) IsNull() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.IsNull(), source: e.source}
}

// IsNotNull tests whether an expression is not null.
func (e Expr) IsNotNull() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.IsNotNull(), source: e.source}
}

// Abs returns the absolute value.
func (e Expr) Abs() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.Abs(), source: e.source}
}

// Neg returns the arithmetic negation.
func (e Expr) Neg() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.Neg(), source: e.source}
}

// Sqrt returns the square root with a floating-point result.
func (e Expr) Sqrt() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.Sqrt(), source: e.source}
}

// ExtractYear returns the DATE's calendar year.
func (e Expr) ExtractYear() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.ExtractYear(), source: e.source}
}

// ExtractMonth returns the DATE's calendar month.
func (e Expr) ExtractMonth() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.ExtractMonth(), source: e.source}
}

// ExtractDay returns the DATE's calendar day.
func (e Expr) ExtractDay() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.ExtractDay(), source: e.source}
}

// TruncateYear returns the first day of the DATE's year.
func (e Expr) TruncateYear() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.TruncateYear(), source: e.source}
}

// TruncateMonth returns the first day of the DATE's month.
func (e Expr) TruncateMonth() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.TruncateMonth(), source: e.source}
}

// Upper converts ASCII letters to uppercase.
func (e Expr) Upper() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.Upper(), source: e.source}
}

// Lower converts ASCII letters to lowercase.
func (e Expr) Lower() Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.Lower(), source: e.source}
}

// Replace substitutes non-overlapping byte matches.
func (e Expr) Replace(old, replacement any) Expr {
	if e.aggregate != aggregateNone {
		return Expr{}
	}
	return Expr{inner: e.inner.Replace(operand(old), operand(replacement)), source: e.source}
}

// Count counts non-null values of this expression.
func (e Expr) Count() Expr { return e.asAggregate(aggregateCount) }

// Sum sums numeric values of this expression.
func (e Expr) Sum() Expr { return e.asAggregate(aggregateSum) }

// Avg averages numeric values of this expression.
func (e Expr) Avg() Expr { return e.asAggregate(aggregateAvg) }

// Min finds the minimum comparable value.
func (e Expr) Min() Expr { return e.asAggregate(aggregateMin) }

// Max finds the maximum comparable value.
func (e Expr) Max() Expr { return e.asAggregate(aggregateMax) }

// Distinct restricts an aggregate to distinct non-null input values.
func (e Expr) Distinct() Expr {
	if e.aggregate == aggregateNone || e.aggregate == aggregateCountStar {
		return Expr{}
	}
	e.distinct = true
	return e
}

func (e Expr) asAggregate(kind aggregateKind) Expr {
	if e.aggregate != aggregateNone || !e.inner.Valid() {
		return Expr{}
	}
	return Expr{inner: e.inner, source: e.source, aggregate: kind}
}

func (e Expr) asPlanAggregate() plan.Aggregate {
	var aggregate plan.Aggregate
	switch e.aggregate {
	case aggregateCountStar:
		aggregate = plan.MakeCountStar()
	case aggregateCount:
		aggregate = plan.MakeCount(e.inner)
	case aggregateSum:
		aggregate = plan.MakeSum(e.inner)
	case aggregateAvg:
		aggregate = plan.MakeAvg(e.inner)
	case aggregateMin:
		aggregate = plan.MakeMin(e.inner)
	case aggregateMax:
		aggregate = plan.MakeMax(e.inner)
	}
	if e.alias != "" {
		aggregate = aggregate.Alias(e.alias)
	}
	if e.distinct {
		aggregate = aggregate.Distinct()
	}
	return aggregate
}
