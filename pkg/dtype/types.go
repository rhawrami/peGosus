package dtype

// MakeType returns the canonical Type for `id`, or the invalid type if `id`
// is unknown.
func MakeType(id TID) Type {
	switch id {
	case NULLT:
		return NullT()
	case INT32T:
		return Int32T()
	case INT64T:
		return Int64T()
	case FLOAT32T:
		return Float32T()
	case FLOAT64T:
		return Float64T()
	case DATET:
		return DateT()
	case TIMESTAMPTZT:
		return TimestampTZT()
	case STRT:
		return StringT()
	case BOOLT:
		return BoolT()
	default:
		return Type{}
	}
}

// NullT represents an untyped null used during expression binding.
func NullT() Type {
	return Type{id: NULLT}
}

// Int32T is a 32-bit signed integer type.
func Int32T() Type {
	return Type{
		id:    INT32T,
		size:  32,
		flags: (1 << flagIsFixedSize) | (1 << flagIsNumericPrim) | (1 << flagIsNumericType) | (1 << flagIsIntegral),
	}
}

// Int64T is a 64-bit signed integer type.
func Int64T() Type {
	return Type{
		id:    INT64T,
		size:  64,
		flags: (1 << flagIsFixedSize) | (1 << flagIsNumericPrim) | (1 << flagIsNumericType) | (1 << flagIsIntegral),
	}
}

// Float32T is a 32-bit floating-point type.
func Float32T() Type {
	return Type{
		id:    FLOAT32T,
		size:  32,
		flags: (1 << flagIsFixedSize) | (1 << flagIsNumericPrim) | (1 << flagIsNumericType) | (1 << flagIsFloating),
	}
}

// Float64T is a 64-bit floating-point type.
func Float64T() Type {
	return Type{
		id:    FLOAT64T,
		size:  64,
		flags: (1 << flagIsFixedSize) | (1 << flagIsNumericPrim) | (1 << flagIsNumericType) | (1 << flagIsFloating),
	}
}

// DateT represents a signed count of days since the Unix epoch.
func DateT() Type {
	return Type{
		id:    DATET,
		size:  32,
		flags: (1 << flagIsFixedSize) | (1 << flagIsNumericPrim) | (1 << flagIsTimeType),
	}
}

// TimestampTZT represents a UTC instant as signed microseconds since the Unix
// epoch. Display and parsing may apply a separate timezone.
func TimestampTZT() Type {
	return Type{
		id:    TIMESTAMPTZT,
		size:  64,
		flags: (1 << flagIsFixedSize) | (1 << flagIsNumericPrim) | (1 << flagIsTimeType),
	}
}

// StringT represents a 16-byte German-string descriptor whose long values
// reference separately owned payload storage.
func StringT() Type {
	return Type{
		id:    STRT,
		size:  StringSize * 8,
		flags: (1 << flagHasVarlen),
	}
}

// BoolT is a boolean type stored one bit per value.
func BoolT() Type {
	return Type{
		id:    BOOLT,
		size:  1,
		flags: (1 << flagIsFixedSize) | (1 << flagIsBitPacked),
	}
}
