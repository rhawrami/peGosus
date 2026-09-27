package store

import "github.com/rhawrami/peGosus/pkg/dtype"

// MakeEmptyTable returns an immutable typed table with no batches.
func MakeEmptyTable(types []dtype.Type) *Table {
	owned := append([]dtype.Type(nil), types...)
	for _, t := range owned {
		if !t.Valid() || t.ID() == dtype.NULLT {
			return nil
		}
	}
	return &Table{valid: true, types: owned, nullable: make([]bool, len(owned))}
}

// MakeTable returns an immutable table that retains `batches`. Every batch
// must have the same vector types.
func MakeTable(batches []*Batch) *Table {
	var types []dtype.Type
	if len(batches) != 0 {
		if batches[0] == nil {
			return nil
		}
		types = make([]dtype.Type, batches[0].NVectors())
		for i := range types {
			types[i] = batches[0].VectorAt(i).Type()
		}
	}
	for _, batch := range batches {
		if batch == nil || batch.NVectors() != len(types) {
			return nil
		}
		for i, t := range types {
			if !batch.VectorAt(i).Type().Equal(t) {
				return nil
			}
		}
	}
	retained := make([]*Batch, len(batches))
	nullable := make([]bool, len(types))
	for i, batch := range batches {
		retained[i] = batch.Retain()
		for j := range types {
			nullable[j] = nullable[j] || batch.VectorAt(j).NiN() != 0
		}
	}
	return &Table{valid: true, types: types, nullable: nullable, batches: retained}
}

// Table is an immutable set of type-compatible batches. Source batch payloads
// must remain unchanged for the table's lifetime.
type Table struct {
	valid    bool
	types    []dtype.Type
	nullable []bool
	batches  []*Batch
}

// Valid returns whether the table has not been released.
func (t *Table) Valid() bool { return t != nil && t.valid }

// NColumns returns the table column count.
func (t *Table) NColumns() int { return len(t.types) }

// TypeAt returns the column type at `o`.
func (t *Table) TypeAt(o int) dtype.Type { return t.types[o] }

// NullableAt returns whether the column at `o` contains any null values.
func (t *Table) NullableAt(o int) bool { return t.nullable[o] }

// NBatches returns the batch count.
func (t *Table) NBatches() int { return len(t.batches) }

// BatchAt returns a borrowed immutable source batch at `o`.
func (t *Table) BatchAt(o int) *Batch { return t.batches[o] }

// Retain returns an independently releasable table over the same batches.
func (t *Table) Retain() *Table {
	if !t.Valid() {
		return nil
	}
	retained := &Table{
		valid: true, types: append([]dtype.Type(nil), t.types...), nullable: append([]bool(nil), t.nullable...),
		batches: make([]*Batch, len(t.batches)),
	}
	for i, batch := range t.batches {
		retained.batches[i] = batch.Retain()
	}
	return retained
}

// Release releases every retained source batch and clears the table.
func (t *Table) Release() {
	if t == nil {
		return
	}
	for _, batch := range t.batches {
		batch.Release()
	}
	t.types = nil
	t.nullable = nil
	t.batches = nil
	t.valid = false
}
