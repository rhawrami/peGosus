package dtype

const (
	flagIsFixedSize   uint16 = 0
	flagIsNumericPrim uint16 = 1
	flagisNumericType uint16 = 2
)

// variable-length element offsets are unsigned 32 bit integers.
const VariableOffsetByteSize = 4

// TID signifies a data type ID
type TID uint8

const (
	// null
	NULLT TID = iota
	// 32-bit signed integer
	INT32T
	// 64-bit signed integer
	INT64T
	// 32-bit floating-point
	FLOAT32T
	// 64-bit floating-point
	FLOAT64T
	// 32-bit signed integer
	DATET
	// 64-bit signed integer
	TIMESTAMPTZT
	// string ("German string" style)
	STRT
	// bitpacked
	BOOLT
)

// Type represents a supported data type.
type Type struct {
	id    TID    // type ID
	size  int8   // size in bits
	flags uint16 // flags
}

func (t Type) String() string {
	switch t.id {
	case NULLT:
		return "na_t"
	case INT32T:
		return "int32_t"
	case INT64T:
		return "int64_t"
	case FLOAT32T:
		return "float32_t"
	case FLOAT64T:
		return "float64_t"
	case DATET:
		return "date_t"
	case TIMESTAMPTZT:
		return "timestamptz_t"
	case BOOLT:
		return "bool_t"
	default:
		return "unknown"
	}
}

// ID returns a type's type ID.
func (t Type) ID() TID { return t.id }

// Size1 returns a type's size in bits (not valid for non-fixed-size types).
func (t Type) Size1() int { return int(t.size) }

// IsFixedSize returns true if a type's storage is fixed size.
func (t Type) IsFixedSize() bool {
	return (t.flags>>flagIsFixedSize)&1 == 1
}

// IsNumericPrim returns true if a type's primitive type is numeric.
func (t Type) IsNumericPrim() bool {
	return (t.flags>>flagIsNumericPrim)&1 == 1
}

// IsNumericType returns true if a type's type is numeric.
func (t Type) IsNumericType() bool {
	return (t.flags>>flagisNumericType)&1 == 1
}

// TypesEq returns true if the types are equal to each other.
func TypesEq(x, y Type) bool {
	return x.ID() == y.ID()
}
