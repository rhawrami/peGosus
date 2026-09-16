package plan

import (
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
)

type literalKind uint8

const (
	literalInvalid literalKind = iota
	literalUntypedInteger
	literalUntypedNull
	literalTyped
)

type literal struct {
	kind   literalKind
	scalar Scalar
}

func makeLiteral(value any) literal {
	switch value := value.(type) {
	case nil:
		return literal{kind: literalUntypedNull}
	case Scalar:
		if value.valid() {
			return literal{kind: literalTyped, scalar: value}
		}
	case int:
		return literal{kind: literalUntypedInteger, scalar: MakeI64Scalar(int64(value))}
	case int32:
		return literal{kind: literalTyped, scalar: MakeI32Scalar(value)}
	case int64:
		return literal{kind: literalTyped, scalar: MakeI64Scalar(value)}
	case float32:
		return literal{kind: literalTyped, scalar: MakeF32Scalar(value)}
	case float64:
		return literal{kind: literalTyped, scalar: MakeF64Scalar(value)}
	case bool:
		return literal{kind: literalTyped, scalar: MakeBoolScalar(value)}
	case string:
		return literal{kind: literalTyped, scalar: MakeStringScalar(value)}
	}
	return literal{}
}

func (l literal) bind(expected dtype.Type) (Scalar, *PlanError) {
	switch l.kind {
	case literalTyped:
		return l.scalar, nil
	case literalUntypedNull:
		if !expected.Valid() || expected.ID() == dtype.NULLT {
			return Scalar{}, makePlanError(ErrorImpossibleCoercion, "cannot infer the type of NULL")
		}
		return MakeNullScalar(expected), nil
	case literalUntypedInteger:
		if !expected.Valid() {
			return l.scalar, nil
		}
		value := l.scalar.i64()
		switch expected.ID() {
		case dtype.INT32T:
			if value < math.MinInt32 || value > math.MaxInt32 {
				return Scalar{}, makePlanError(ErrorImpossibleCoercion, "integer literal does not fit INT32")
			}
			return MakeI32Scalar(int32(value)), nil
		case dtype.INT64T:
			return l.scalar, nil
		case dtype.FLOAT32T:
			return MakeF32Scalar(float32(value)), nil
		case dtype.FLOAT64T:
			return MakeF64Scalar(float64(value)), nil
		default:
			return Scalar{}, makePlanError(ErrorImpossibleCoercion, "integer literal cannot be coerced to "+expected.String())
		}
	default:
		return Scalar{}, makePlanError(ErrorInvalidExpression, "unsupported literal value")
	}
}
