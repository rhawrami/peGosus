package array

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
	STRING
)

// ArrayType represents a vector type
type ArrayType struct {
	lt            LogicalType
	bytesRequired int
}

// Logical returns the type's logical type
func (t ArrayType) Logical() LogicalType { return t.lt }

// String returns the string representation of the type
func (t ArrayType) String() string {
	switch t.lt {
	case NULL:
		return "null"
	case BOOL:
		return "bool"
	case I32:
		return "int32"
	case I64:
		return "int64"
	case F32:
		return "float32"
	case F64:
		return "float64"
	case STRING:
		return "string"
	case DATE:
		return "date"
	}
	return ""
}

// IsNumeric returns true if the type is numeric
func (t ArrayType) IsNumeric() bool {
	switch t.lt {
	case I32, I64, F32, F64:
		return true
	default:
		return false
	}
}

// BytesReq returns the bytes required to store a single element of ArrayType
//
// If type is variable length (e.g., string) or bitpacked (e.g., bool), BytesReq
// will return -1
func (t ArrayType) BytesReq() int { return t.bytesRequired }

// NewNullT returns a null type identifier
func NewNullT() ArrayType { return ArrayType{lt: NULL, bytesRequired: 0} }

// NewInt32T returns a 32-bit signed integer type identifier
func NewInt32T() ArrayType { return ArrayType{lt: I32, bytesRequired: 4} }

// NewInt64T returns a 64-bit signed integer type identifier
func NewInt64T() ArrayType { return ArrayType{lt: I64, bytesRequired: 8} }

// NewFloat32T returns a 32-bit floating-point type identifier
func NewFloat32T() ArrayType { return ArrayType{lt: F32, bytesRequired: 4} }

// NewFloat64T returns a 64-bit floating-point type identifier
func NewFloat64T() ArrayType { return ArrayType{lt: F64, bytesRequired: 8} }

// NewDateT returns a date type identifier
func NewDateT() ArrayType { return ArrayType{lt: DATE, bytesRequired: 4} }

// NewBoolT returns a bool type identifier
func NewBoolT() ArrayType { return ArrayType{lt: BOOL, bytesRequired: -1} }

// NewStringT returns a string type identifier
func NewStringT() ArrayType { return ArrayType{lt: STRING, bytesRequired: -1} }
