package dtype

// NullT represents a null type.
func NullT() Type {
	return Type{
		id:    NULLT,
		size:  -1,
		flags: 0,
	}
}

// Int32T is a 32-bit signed integer type.
func Int32T() Type {
	return Type{
		id:    INT32T,
		size:  32,
		flags: 7,
	}
}

// Int64T is a 64-bit signed integer type.
func Int64T() Type {
	return Type{
		id:    INT64T,
		size:  64,
		flags: 7,
	}
}

// Float32T is a 32-bit floating-point type.
func Float32T() Type {
	return Type{
		id:    FLOAT32T,
		size:  32,
		flags: 7,
	}
}

// Float64T is a 64-bit floating-point type.
func Float64T() Type {
	return Type{
		id:    FLOAT64T,
		size:  64,
		flags: 7,
	}
}

// DateT represents a date (e.g., "YYYY-MM-DD"), stored as a 32-bit signed integer type.
func DateT() Type {
	return Type{
		id:    DATET,
		size:  32,
		flags: 3,
	}
}

// TimestampTZT represents a timezone, stored as a 64-bit signed integer type.
func TimestampTZT() Type {
	return Type{
		id:    TIMESTAMPTZT,
		size:  64,
		flags: 3,
	}
}

// StringT represents a variable length string.
func StringT() Type {
	return Type{
		id:    STRT,
		size:  -1,
		flags: 0,
	}
}

// BoolT is a boolean type, stored bitpacked, where 1 is true.
func BoolT() Type {
	return Type{
		id:    STRT,
		size:  1,
		flags: 1,
	}
}
