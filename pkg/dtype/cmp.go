package dtype

// TypesEqType returns whether the types have identical canonical metadata.
func TypesEqType(x, y Type) bool {
	return x.Equal(y)
}

// TypesEqSize returns whether valid physical types have the same nonzero
// primary slot size.
func TypesEqSize(x, y Type) bool {
	return x.Valid() && y.Valid() && x.SlotBits() != 0 && x.SlotBits() == y.SlotBits()
}

// TypesCanConv returns whether an explicit conversion from `x` to `y` is
// supported by the type system.
func TypesCanConv(x, y Type) bool {
	if !x.Valid() || !y.Valid() {
		return false
	}
	if x.Equal(y) {
		return true
	}
	if x.id == NULLT {
		return y.id != NULLT
	}
	if y.id == NULLT {
		return false
	}
	if x.IsNumericType() && y.IsNumericType() {
		return true
	}
	if x.id == STRT || y.id == STRT {
		return true
	}
	if (x.IsNumericType() && y.id == BOOLT) || (x.id == BOOLT && y.IsNumericType()) {
		return true
	}
	return (x.id == DATET && y.id == TIMESTAMPTZT) || (x.id == TIMESTAMPTZT && y.id == DATET)
}

// CommonNumericType returns the type used to evaluate a binary numeric
// expression, or the invalid type and false for nonnumeric operands.
func CommonNumericType(x, y Type) (Type, bool) {
	if !x.IsNumericType() || !y.IsNumericType() {
		return Type{}, false
	}
	if x.Equal(y) {
		return x, true
	}
	if x.id == FLOAT64T || y.id == FLOAT64T {
		return Float64T(), true
	}
	if x.id == FLOAT32T || y.id == FLOAT32T {
		return Float64T(), true
	}
	if x.id == INT64T || y.id == INT64T {
		return Int64T(), true
	}
	return Int32T(), true
}
