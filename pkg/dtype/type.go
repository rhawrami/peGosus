package dtype

// TID signifies a data type ID
type TID int

// variable-length element offsets are unsigned 32 bit integers.
const VariableOffsetByteSize = 4

const (
	// 32-bit signed integer
	INT32T TID = iota
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
	// string (contig buff + offset buff)
	STRT
	// bitpacked
	BOOLT
)

// Type represents a supported data type.
type Type interface {
	// implement Stringer.
	String() string
	// ID returns a type's TID.
	ID() TID
	// Size1 returns the bit count for storing one element;
	// returns -1 if variable length.
	Size1() int
	// Size8 returns the byte count for storing one element;
	// returns -1 if variable length or bitpacked.
	Size8() int
	// IsNumeric returns true if the underlying storage is numeric.
	IsNumeric() bool
	// IsNumericT returns true if the type is numeric (e.g., DateT returns false).
	IsNumericT() bool
}

// TypesEq returns true if the types are equal to each other.
func TypesEq(x, y Type) bool {
	return x.ID() == y.ID()
}
