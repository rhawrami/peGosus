package plan

import "github.com/rhawrami/peGosus/pkg/dtype"

// AggregateKind identifies a supported reduction.
type AggregateKind uint8

const (
	AggregateCountStar AggregateKind = iota + 1
	AggregateCount
	AggregateSum
	AggregateAvg
	AggregateMin
	AggregateMax
)

// Aggregate describes a reduction over an expression.
type Aggregate struct {
	kind     AggregateKind
	input    Expr
	alias    string
	distinct bool
}

// MakeCountStar counts active rows.
func MakeCountStar() Aggregate { return Aggregate{kind: AggregateCountStar} }

// MakeCount counts non-null expression values.
func MakeCount(input Expr) Aggregate { return Aggregate{kind: AggregateCount, input: input} }

// MakeCountDistinct counts distinct non-null expression values.
func MakeCountDistinct(input Expr) Aggregate {
	return Aggregate{kind: AggregateCount, input: input, distinct: true}
}

// MakeSum sums non-null numeric values.
func MakeSum(input Expr) Aggregate { return Aggregate{kind: AggregateSum, input: input} }

// MakeAvg averages non-null numeric values.
func MakeAvg(input Expr) Aggregate { return Aggregate{kind: AggregateAvg, input: input} }

// MakeMin finds the least comparable non-null value.
func MakeMin(input Expr) Aggregate { return Aggregate{kind: AggregateMin, input: input} }

// MakeMax finds the greatest comparable non-null value.
func MakeMax(input Expr) Aggregate { return Aggregate{kind: AggregateMax, input: input} }

// Alias returns a named aggregate result.
func (a Aggregate) Alias(name string) Aggregate { a.alias = name; return a }

// Distinct restricts a reduction to distinct non-null input values.
func (a Aggregate) Distinct() Aggregate { a.distinct = true; return a }

func aggregateType(kind AggregateKind, input dtype.Type) (dtype.Type, bool, bool) {
	switch kind {
	case AggregateCountStar, AggregateCount:
		return dtype.Int64T(), false, true
	case AggregateSum:
		switch input.ID() {
		case dtype.INT32T, dtype.INT64T:
			return dtype.Int64T(), true, true
		case dtype.FLOAT32T, dtype.FLOAT64T:
			return dtype.Float64T(), true, true
		}
	case AggregateAvg:
		if input.IsNumericType() {
			return dtype.Float64T(), true, true
		}
	case AggregateMin, AggregateMax:
		if input.Valid() && input.ID() != dtype.NULLT && input.ID() != dtype.BOOLT {
			return input, true, true
		}
	}
	return dtype.Type{}, false, false
}
