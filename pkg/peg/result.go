package peg

import (
	"errors"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/plan"
	"github.com/rhawrami/peGosus/pkg/store"
)

// Kind identifies a column's logical value type.
type Kind = dtype.TID

const (
	Int32       Kind = dtype.INT32T
	Int64       Kind = dtype.INT64T
	Float32     Kind = dtype.FLOAT32T
	Float64     Kind = dtype.FLOAT64T
	Bool        Kind = dtype.BOOLT
	String      Kind = dtype.STRT
	Date        Kind = dtype.DATET
	TimestampTZ Kind = dtype.TIMESTAMPTZT
)

// Field describes one result column.
type Field struct {
	Name     string
	Kind     Kind
	Nullable bool
	Source   string
}

func makeFields(schema plan.Schema) []Field {
	fields := make([]Field, schema.Len())
	for i := range fields {
		field := schema.FieldAt(i)
		fields[i] = Field{Name: field.Name(), Kind: field.Type().ID(), Nullable: field.Nullable(), Source: field.Source()}
	}
	return fields
}

// Result owns materialized output batches. Call Release when finished. Row
// order is unspecified unless the query has an explicit ordering operation.
type Result struct {
	fields  []Field
	batches []*store.Batch
	rows    int
}

// Schema returns an independent copy of the result fields.
func (r *Result) Schema() []Field {
	if r == nil {
		return nil
	}
	return append([]Field(nil), r.fields...)
}

// NumRows returns the number of selected result rows.
func (r *Result) NumRows() int {
	if r == nil {
		return 0
	}
	return r.rows
}

// NumBatches returns the number of retained output batches.
func (r *Result) NumBatches() int {
	if r == nil {
		return 0
	}
	return len(r.batches)
}

// BatchAt returns a borrowed batch. Retain it if it must outlive the result.
func (r *Result) BatchAt(i int) (Batch, bool) {
	if r == nil || i < 0 || i >= len(r.batches) {
		return Batch{}, false
	}
	return Batch{inner: r.batches[i], fields: r.fields}, true
}

// ForEachRow visits selected rows across all batches without compaction.
// The callback's batch is borrowed and its row is a physical offset.
func (r *Result) ForEachRow(visit func(batch Batch, row int) bool) bool {
	if r == nil || visit == nil {
		return false
	}
	for _, owned := range r.batches {
		batch := Batch{inner: owned, fields: r.fields}
		if !batch.ForEachActive(func(row int) bool { return visit(batch, row) }) {
			return false
		}
	}
	return true
}

// Release frees the result's batch references. Separately retained batches
// remain valid and must be released by their owners.
func (r *Result) Release() {
	if r == nil {
		return
	}
	for _, batch := range r.batches {
		batch.Release()
	}
	r.fields, r.batches, r.rows = nil, nil, 0
}

// Batch is a physical row domain with optional validity and row selection.
// Borrowed column views must not be mutated or kept past its owner's Release.
type Batch struct {
	inner  *store.Batch
	fields []Field
	owned  bool
}

// Len returns the physical row-domain length.
func (b Batch) Len() int {
	if b.inner == nil {
		return 0
	}
	return b.inner.Len()
}

// ActiveLen returns the selected row count.
func (b Batch) ActiveLen() int {
	if b.inner == nil {
		return 0
	}
	return b.inner.ActiveLen()
}

// Retain returns independently owned references to this batch's storage.
func (b Batch) Retain() Batch {
	if b.inner == nil {
		return Batch{}
	}
	return Batch{inner: b.inner.Retain(), fields: b.fields, owned: true}
}

// Release releases an independently retained batch. Borrowed batches remain
// owned by their Result and are unaffected.
func (b *Batch) Release() {
	if b == nil {
		return
	}
	if b.owned && b.inner != nil {
		b.inner.Release()
	}
	*b = Batch{}
}

// Column returns a borrowed typed view for a named column.
func (b Batch) Column(name string) (Column, error) {
	if b.inner == nil {
		return Column{}, &Error{code: ErrorInvalidQuery, cause: errors.New("released batch")}
	}
	at := -1
	for i, field := range b.fields {
		if field.Name != name && (field.Source == "" || field.Source+"."+field.Name != name) {
			continue
		}
		if at != -1 {
			return Column{}, &Error{code: ErrorAmbiguousColumn, cause: errors.New("ambiguous result column: " + name)}
		}
		at = i
	}
	if at < 0 {
		return Column{}, &Error{code: ErrorMissingColumn, cause: errors.New("missing result column: " + name)}
	}
	return Column{vector: b.inner.VectorAt(at)}, nil
}

// ForEachActive visits only selected physical row offsets, without compacting.
// It returns false if visit stops early or is nil.
func (b Batch) ForEachActive(visit func(row int) bool) bool {
	if b.inner == nil || visit == nil {
		return false
	}
	selection := b.inner.Selection()
	if selection == nil {
		for row := range b.inner.Len() {
			if !visit(row) {
				return false
			}
		}
	} else if bitmap, ok := selection.AsBitMap(); ok {
		for row := range b.inner.Len() {
			if bitmap.IsSet(row) && !visit(row) {
				return false
			}
		}
	} else if offsets, ok := selection.AsSelVec(); ok {
		for _, row := range offsets.Offsets() {
			if !visit(int(row)) {
				return false
			}
		}
	}
	return true
}

// Column is a borrowed view; its typed slices share vector payload storage.
// Treat returned slices as immutable and keep the owning batch live.
type Column struct{ vector *store.Vector }

// Kind returns the logical column type.
func (c Column) Kind() Kind {
	if c.vector == nil {
		return dtype.INVALIDT
	}
	return c.vector.TypeID()
}

// Len returns the physical column length.
func (c Column) Len() int {
	if c.vector == nil {
		return 0
	}
	return c.vector.Len()
}

// IsValid reports whether the row contains a non-null value.
func (c Column) IsValid(row int) bool {
	if c.vector == nil || row < 0 || row >= c.vector.Len() {
		return false
	}
	return c.vector.Validity() == nil || c.vector.Validity().IsSet(row)
}

// Int32s returns borrowed INT32 or DATE values.
func (c Column) Int32s() []int32 {
	if c.vector == nil {
		return nil
	}
	return c.vector.I32s()
}

// Int64s returns borrowed INT64 or TIMESTAMPTZ values.
func (c Column) Int64s() []int64 {
	if c.vector == nil {
		return nil
	}
	return c.vector.I64s()
}

// Float32s returns borrowed FLOAT32 values.
func (c Column) Float32s() []float32 {
	if c.vector == nil {
		return nil
	}
	return c.vector.F32s()
}

// Float64s returns borrowed FLOAT64 values.
func (c Column) Float64s() []float64 {
	if c.vector == nil {
		return nil
	}
	return c.vector.F64s()
}

// BoolAt returns a bit-packed boolean value when the row is non-null.
func (c Column) BoolAt(row int) (bool, bool) {
	if c.Kind() != Bool || !c.IsValid(row) {
		return false, false
	}
	return c.vector.Bools()[row>>3]&(1<<(row&7)) != 0, true
}

// StringAt returns a borrowed string when the row is non-null.
func (c Column) StringAt(row int) (string, bool) {
	if c.Kind() != String || !c.IsValid(row) {
		return "", false
	}
	return c.vector.StringAt(row).View(), true
}
