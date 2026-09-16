package plan

import (
	"sync/atomic"

	"github.com/rhawrami/peGosus/pkg/dtype"
)

var nextFieldID atomic.Uint64

// FieldID identifies a field through logical rewrites.
type FieldID uint64

// MakeSchema returns a schema for matching names and types.
func MakeSchema(names []string, types []dtype.Type) Schema {
	nullable := make([]bool, len(types))
	for i := range nullable {
		nullable[i] = true
	}
	return MakeSchemaWithNullability(names, types, nullable)
}

// MakeSchemaWithNullability returns a schema for matching names, types, and
// nullability values.
func MakeSchemaWithNullability(names []string, types []dtype.Type, nullable []bool) Schema {
	if len(names) != len(types) || len(types) != len(nullable) {
		return Schema{}
	}
	fields := make([]Field, len(names))
	for i := range names {
		if !types[i].Valid() || types[i].ID() == dtype.NULLT {
			return Schema{}
		}
		fields[i] = Field{id: makeFieldID(), name: names[i], dType: types[i], nullable: nullable[i]}
	}
	return Schema{valid: true, fields: fields}
}

// Field describes one bound logical column.
type Field struct {
	id       FieldID
	name     string
	dType    dtype.Type
	nullable bool
}

// ID returns the stable field ID.
func (f Field) ID() FieldID { return f.id }

// Name returns the field name.
func (f Field) Name() string { return f.name }

// Type returns the field type.
func (f Field) Type() dtype.Type { return f.dType }

// Nullable returns whether the field may contain null values.
func (f Field) Nullable() bool { return f.nullable }

// Schema is an ordered set of logical fields.
type Schema struct {
	valid  bool
	fields []Field
}

// Valid returns whether the schema was constructed successfully.
func (s Schema) Valid() bool { return s.valid }

// Len returns the field count.
func (s Schema) Len() int { return len(s.fields) }

// FieldAt returns the field at `o`.
func (s Schema) FieldAt(o int) Field { return s.fields[o] }

func (s Schema) find(name string) (int, bool) {
	offset := -1
	for i, field := range s.fields {
		if field.name == name {
			if offset != -1 {
				return 0, false
			}
			offset = i
		}
	}
	return offset, offset != -1
}

func (s Schema) resolve(name string) (int, ErrorCode) {
	offset := -1
	for i, field := range s.fields {
		if field.name != name {
			continue
		}
		if offset != -1 {
			return 0, ErrorAmbiguousColumn
		}
		offset = i
	}
	if offset == -1 {
		return 0, ErrorMissingColumn
	}
	return offset, 0
}

func (s Schema) project(offsets []int) Schema {
	fields := make([]Field, len(offsets))
	for i, offset := range offsets {
		if offset < 0 || offset >= len(s.fields) {
			return Schema{}
		}
		fields[i] = s.fields[offset]
	}
	return Schema{valid: true, fields: fields}
}

func (s Schema) offsetOf(id FieldID) int {
	for i, field := range s.fields {
		if field.id == id {
			return i
		}
	}
	return -1
}

func makeFieldID() FieldID { return FieldID(nextFieldID.Add(1)) }
