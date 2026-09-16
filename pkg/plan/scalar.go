package plan

import (
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
)

// MakeI32Scalar returns an INT32 scalar.
func MakeI32Scalar(value int32) Scalar {
	return Scalar{dType: dtype.Int32T(), bits: uint64(uint32(value))}
}

// MakeI64Scalar returns an INT64 scalar.
func MakeI64Scalar(value int64) Scalar {
	return Scalar{dType: dtype.Int64T(), bits: uint64(value)}
}

// MakeF32Scalar returns a FLOAT32 scalar.
func MakeF32Scalar(value float32) Scalar {
	return Scalar{dType: dtype.Float32T(), bits: uint64(math.Float32bits(value))}
}

// MakeF64Scalar returns a FLOAT64 scalar.
func MakeF64Scalar(value float64) Scalar {
	return Scalar{dType: dtype.Float64T(), bits: math.Float64bits(value)}
}

// MakeBoolScalar returns a BOOL scalar.
func MakeBoolScalar(value bool) Scalar {
	if value {
		return Scalar{dType: dtype.BoolT(), bits: 1}
	}
	return Scalar{dType: dtype.BoolT()}
}

// MakeStringScalar returns an owned STRING scalar.
func MakeStringScalar(value string) Scalar {
	return Scalar{dType: dtype.StringT(), text: value}
}

// MakeDateScalar returns a DATE scalar represented as days since Unix epoch.
func MakeDateScalar(value int32) Scalar {
	return Scalar{dType: dtype.DateT(), bits: uint64(uint32(value))}
}

// MakeTimestampTZScalar returns a TIMESTAMPTZ scalar represented as UTC
// microseconds since Unix epoch.
func MakeTimestampTZScalar(value int64) Scalar {
	return Scalar{dType: dtype.TimestampTZT(), bits: uint64(value)}
}

// MakeNullScalar returns a typed null scalar.
func MakeNullScalar(t dtype.Type) Scalar {
	if !t.Valid() || t.ID() == dtype.NULLT {
		return Scalar{}
	}
	return Scalar{dType: t, null: true}
}

// Scalar is an exactly typed bound literal value.
type Scalar struct {
	dType dtype.Type
	bits  uint64
	text  string
	null  bool
}

// Type returns the scalar type.
func (s Scalar) Type() dtype.Type { return s.dType }

// IsNull returns whether the scalar is null.
func (s Scalar) IsNull() bool { return s.null }

func (s Scalar) i32() int32          { return int32(uint32(s.bits)) }
func (s Scalar) i64() int64          { return int64(s.bits) }
func (s Scalar) f32() float32        { return math.Float32frombits(uint32(s.bits)) }
func (s Scalar) f64() float64        { return math.Float64frombits(s.bits) }
func (s Scalar) boolean() bool       { return s.bits != 0 }
func (s Scalar) stringValue() string { return s.text }
func (s Scalar) valid() bool {
	return s.dType.Valid() && s.dType.ID() != dtype.NULLT
}

func (s Scalar) cast(t dtype.Type) (Scalar, bool) {
	if !s.valid() || !t.Valid() || t.ID() == dtype.NULLT {
		return Scalar{}, false
	}
	if s.null {
		return MakeNullScalar(t), true
	}
	if s.dType.Equal(t) {
		return s, true
	}
	if !s.dType.IsNumericType() || !t.IsNumericType() {
		return Scalar{}, false
	}

	switch t.ID() {
	case dtype.INT32T:
		switch s.dType.ID() {
		case dtype.INT64T:
			return MakeI32Scalar(int32(s.i64())), true
		case dtype.FLOAT32T:
			value := s.f32()
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || value < -2147483648 || value >= 2147483648 {
				return MakeNullScalar(t), true
			}
			return MakeI32Scalar(int32(value)), true
		case dtype.FLOAT64T:
			value := s.f64()
			if math.IsNaN(value) || math.IsInf(value, 0) || value < -2147483648 || value >= 2147483648 {
				return MakeNullScalar(t), true
			}
			return MakeI32Scalar(int32(value)), true
		}
	case dtype.INT64T:
		switch s.dType.ID() {
		case dtype.INT32T:
			return MakeI64Scalar(int64(s.i32())), true
		case dtype.FLOAT32T:
			value := float64(s.f32())
			if math.IsNaN(value) || math.IsInf(value, 0) || value < -9223372036854775808 || value >= 9223372036854775808 {
				return MakeNullScalar(t), true
			}
			return MakeI64Scalar(int64(value)), true
		case dtype.FLOAT64T:
			value := s.f64()
			if math.IsNaN(value) || math.IsInf(value, 0) || value < -9223372036854775808 || value >= 9223372036854775808 {
				return MakeNullScalar(t), true
			}
			return MakeI64Scalar(int64(value)), true
		}
	case dtype.FLOAT32T:
		switch s.dType.ID() {
		case dtype.INT32T:
			return MakeF32Scalar(float32(s.i32())), true
		case dtype.INT64T:
			return MakeF32Scalar(float32(s.i64())), true
		case dtype.FLOAT64T:
			return MakeF32Scalar(float32(s.f64())), true
		}
	case dtype.FLOAT64T:
		switch s.dType.ID() {
		case dtype.INT32T:
			return MakeF64Scalar(float64(s.i32())), true
		case dtype.INT64T:
			return MakeF64Scalar(float64(s.i64())), true
		case dtype.FLOAT32T:
			return MakeF64Scalar(float64(s.f32())), true
		}
	}
	return Scalar{}, false
}

func (s Scalar) convert(t dtype.Type) (Scalar, bool) {
	if s.dType.Equal(t) {
		return s, true
	}
	common, ok := dtype.CommonNumericType(s.dType, t)
	if !ok || !common.Equal(t) {
		return Scalar{}, false
	}
	return s.cast(t)
}
