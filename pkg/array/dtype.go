package array

// LogicalType represents a logical type ID for an array.
// Idea was pulled from Apache Arrow source code.
type LogicalType int

const (
	// null type
	NULL LogicalType = iota
	// 32-bit signed integer
	I32
	// 64-bit signed integer
	I64
	// 32-bit floating-point
	F32
	// 64-bit floating-point
	F64
	// date (32-bit signed integer, representing days from Unix epoch)
	DATE
	// boolean
	BOOL
	// string
	STR
)

// ArrayType represents any given type that an array can possess.
type ArrayType interface {
	// Logical returns the logical type ID of a given type.
	Logical() LogicalType
	// IsNumeric returns true if a given type is numeric; note that
	// date types, while stored as numeric, will return false
	IsNumeric() bool
	// IsFixedSize returns true if a given type has a fixed element size;
	// For example, int64 returns true, and string returns false.
	IsFixedSize() bool
	// ByteCount returns the bytes required to store a single element of the
	// given type; if data is variable size or bitpacked, returns -1.
	ByteCount() bool
	// BitCount returns the bits required to store a single element of the
	// given type; if data is variable size, returns -1.
	BitCount() bool
}
