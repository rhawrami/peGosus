package dtype

const (
	flagIsFixedSize uint16 = iota
	flagIsNumericPrim
	flagIsNumericType
	flagIsTimeType
	flagIsIntegral
	flagIsFloating
	flagHasVarlen
	flagIsBitPacked
)

// TID identifies a logical data type.
type TID uint8

const (
	INVALIDT TID = iota
	NULLT
	INT32T
	INT64T
	FLOAT32T
	FLOAT64T
	DATET
	TIMESTAMPTZT
	STRT
	BOOLT
)

// Type represents a supported logical data type and its physical slot traits.
type Type struct {
	id    TID    // type ID
	size  uint16 // primary storage size in bits per value
	flags uint16 // type traits
}

func (t Type) String() string {
	switch t.id {
	case INVALIDT:
		return "invalid_t"
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
	case STRT:
		return "string_t"
	case BOOLT:
		return "bool_t"
	default:
		return "unknown"
	}
}

// ID returns the type ID.
func (t Type) ID() TID { return t.id }

// Valid returns whether the type has a recognized logical type ID.
func (t Type) Valid() bool { return t.id > INVALIDT && t.id <= BOOLT }

// Equal returns whether the types have identical canonical metadata.
func (t Type) Equal(x Type) bool { return t == x }

// Size1 returns the primary storage size in bits per value.
func (t Type) Size1() int { return int(t.size) }

// SlotBits returns the primary storage size in bits per value. Strings use a
// fixed descriptor slot and separately retain any non-inline payload.
func (t Type) SlotBits() int { return int(t.size) }

// IsFixedSize returns whether values require no auxiliary variable storage.
func (t Type) IsFixedSize() bool { return t.hasFlag(flagIsFixedSize) }

// IsNumericPrim returns whether the physical primitive is numeric.
func (t Type) IsNumericPrim() bool { return t.hasFlag(flagIsNumericPrim) }

// IsNumericType returns whether the logical type is numeric.
func (t Type) IsNumericType() bool { return t.hasFlag(flagIsNumericType) }

// IsTimeType returns whether the logical type is temporal.
func (t Type) IsTimeType() bool { return t.hasFlag(flagIsTimeType) }

// IsIntegral returns whether the logical type is an integer.
func (t Type) IsIntegral() bool { return t.hasFlag(flagIsIntegral) }

// IsFloating returns whether the logical type is floating point.
func (t Type) IsFloating() bool { return t.hasFlag(flagIsFloating) }

// HasVarlen returns whether values may reference auxiliary variable storage.
func (t Type) HasVarlen() bool { return t.hasFlag(flagHasVarlen) }

// IsBitPacked returns whether adjacent values are stored one bit apart.
func (t Type) IsBitPacked() bool { return t.hasFlag(flagIsBitPacked) }

// CanArithmetic returns whether arithmetic operators accept the type.
func (t Type) CanArithmetic() bool { return t.IsNumericType() }

// CanCompareEquality returns whether equality operators accept the type.
func (t Type) CanCompareEquality() bool {
	return t.Valid() && t.id != NULLT
}

// CanOrder returns whether ordering operators accept the type.
func (t Type) CanOrder() bool {
	return t.IsNumericType() || t.IsTimeType() || t.id == STRT
}

// CanHash returns whether hash operators accept the type.
func (t Type) CanHash() bool {
	return t.Valid() && t.id != NULLT
}

func (t Type) hasFlag(flag uint16) bool {
	return (t.flags>>flag)&1 == 1
}
