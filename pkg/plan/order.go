package plan

// OrderKey specifies one ordering expression and its null placement.
type OrderKey struct {
	input      Expr
	descending bool
	nullsFirst bool
}

// MakeOrderKey returns an ascending, nulls-last ordering key.
func MakeOrderKey(input Expr) OrderKey { return OrderKey{input: input} }

// Desc returns a descending ordering key.
func (k OrderKey) Desc() OrderKey { k.descending = true; return k }

// NullsFirst returns a key that sorts nulls before non-null values.
func (k OrderKey) NullsFirst() OrderKey { k.nullsFirst = true; return k }

// NullsLast returns a key that sorts nulls after non-null values.
func (k OrderKey) NullsLast() OrderKey { k.nullsFirst = false; return k }

type boundOrderKey struct {
	input      boundExprID
	descending bool
	nullsFirst bool
}

type physicalOrderKey struct {
	program    physicalExprProgram
	descending bool
	nullsFirst bool
}
