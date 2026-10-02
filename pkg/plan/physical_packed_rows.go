package plan

import (
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type packedValue struct {
	bits   uint64
	offset uint64
	length uint32
	null   uint8
	_      [3]byte
}

type packedRows struct {
	values      *mem.Segment
	strings     *mem.Segment
	scope       *mem.AllocationScope
	types       []dtype.Type
	length      int
	capacity    int
	stringBytes int
}

func (r *packedRows) release() {
	if r.values != nil {
		r.values.Dec()
	}
	if r.strings != nil {
		r.strings.Dec()
	}
	*r = packedRows{}
}

func (r *packedRows) bytes() int64 {
	var n int64
	if r.values != nil {
		n += int64(r.values.Len())
	}
	if r.strings != nil {
		n += int64(r.strings.Len())
	}
	return n
}

func (r *packedRows) slots() []packedValue {
	if r.values == nil {
		return nil
	}
	return unsafe.Slice((*packedValue)(unsafe.Pointer(unsafe.SliceData(r.values.AsBytes()))), r.capacity*len(r.types))
}

func (r *packedRows) at(row, column int) Scalar {
	value := r.slots()[row*len(r.types)+column]
	if value.null != 0 {
		return MakeNullScalar(r.types[column])
	}
	s := Scalar{dType: r.types[column], bits: value.bits}
	if r.types[column].ID() == dtype.STRT && value.length != 0 {
		payload := r.strings.AsBytes()[value.offset : value.offset+uint64(value.length)]
		s.text = unsafe.String(unsafe.SliceData(payload), len(payload))
	}
	return s
}

func (r *packedRows) append(a *mem.Allocator, batch *store.Batch, row int, budget int64) bool {
	if r.types == nil {
		r.types = make([]dtype.Type, batch.NVectors())
		for i := range r.types {
			r.types[i] = batch.VectorAt(i).Type()
		}
	}
	if len(r.types) != batch.NVectors() {
		return false
	}
	maxInt := int(^uint(0) >> 1)
	additional := 0
	for column := range r.types {
		v := batch.VectorAt(column)
		if !v.Type().Equal(r.types[column]) {
			return false
		}
		if v.TypeID() == dtype.STRT && (v.Validity() == nil || v.Validity().IsSet(row)) {
			value := v.StringAt(row).View()
			if len(value) > maxInt-additional {
				return false
			}
			additional += len(value)
		}
	}
	if !r.reserve(a, additional, budget) {
		return false
	}
	slots := r.slots()[r.length*len(r.types) : (r.length+1)*len(r.types)]
	for column := range slots {
		value := scalarAt(batch.VectorAt(column), row)
		slots[column] = packedValue{bits: value.bits}
		if value.IsNull() {
			slots[column].null = 1
		} else if r.types[column].ID() == dtype.STRT {
			slots[column].offset = uint64(r.stringBytes)
			slots[column].length = uint32(len(value.text))
			if len(value.text) != 0 {
				copy(r.strings.AsBytes()[r.stringBytes:], value.text)
			}
			r.stringBytes += len(value.text)
		}
	}
	r.length++
	return true
}

func (r *packedRows) appendScalars(a *mem.Allocator, values []Scalar, budget int64) bool {
	if r.types == nil {
		r.types = make([]dtype.Type, len(values))
		for i, value := range values {
			r.types[i] = value.Type()
		}
	}
	if len(r.types) != len(values) {
		return false
	}
	additional := 0
	maxInt := int(^uint(0) >> 1)
	for i, value := range values {
		if !value.Type().Equal(r.types[i]) {
			return false
		}
		if !value.IsNull() && value.Type().ID() == dtype.STRT {
			if len(value.text) > maxInt-additional {
				return false
			}
			additional += len(value.text)
		}
	}
	if !r.reserve(a, additional, budget) {
		return false
	}
	slots := r.slots()[r.length*len(r.types) : (r.length+1)*len(r.types)]
	for i, value := range values {
		slots[i] = packedValue{bits: value.bits}
		if value.IsNull() {
			slots[i].null = 1
		} else if r.types[i].ID() == dtype.STRT {
			slots[i].offset = uint64(r.stringBytes)
			slots[i].length = uint32(len(value.text))
			if len(value.text) != 0 {
				copy(r.strings.AsBytes()[r.stringBytes:], value.text)
			}
			r.stringBytes += len(value.text)
		}
	}
	r.length++
	return true
}

func (r *packedRows) setFixed(batch *store.Batch, row, index int) bool {
	if index < 0 || index >= r.length || batch.NVectors() != len(r.types) {
		return false
	}
	slots := r.slots()[index*len(r.types) : (index+1)*len(r.types)]
	for column := range slots {
		vector := batch.VectorAt(column)
		if !vector.Type().Equal(r.types[column]) || !vector.Type().IsFixedSize() {
			return false
		}
		value := scalarAt(vector, row)
		slots[column] = packedValue{bits: value.bits}
		if value.IsNull() {
			slots[column].null = 1
		}
	}
	return true
}

func (r *packedRows) reserve(a *mem.Allocator, additional int, budget int64) bool {
	maxInt := int(^uint(0) >> 1)
	if additional > maxInt-r.stringBytes {
		return false
	}
	valueCapacity, stringCapacity := r.capacity, 0
	if r.strings != nil {
		stringCapacity = r.strings.Len()
	}
	if valueCapacity == 0 {
		valueCapacity = 4
	}
	for valueCapacity <= r.length {
		if valueCapacity > maxInt/2 {
			return false
		}
		valueCapacity *= 2
	}
	valueSize := int64(len(r.types)) * 24
	if valueSize == 0 || int64(valueCapacity) > int64(maxInt)/valueSize {
		return false
	}
	newStringCapacity := stringCapacity
	if newStringCapacity == 0 && additional > 0 {
		newStringCapacity = 64
	}
	for newStringCapacity < r.stringBytes+additional {
		if newStringCapacity > maxInt/2 {
			return false
		}
		newStringCapacity *= 2
	}
	newValues := int64(valueCapacity) * valueSize
	oldValues := int64(r.capacity) * valueSize
	if newValues+int64(stringCapacity) > budget && valueCapacity == r.capacity || newValues+oldValues+int64(stringCapacity) > budget && valueCapacity != r.capacity || newValues+int64(stringCapacity)+int64(newStringCapacity) > budget && newStringCapacity != stringCapacity || newValues+int64(newStringCapacity) > budget {
		return false
	}
	if valueCapacity != r.capacity {
		values, ok := allocOperatorSegment(a, r.scope, int(newValues))
		if !ok {
			return false
		}
		if r.values != nil {
			copy(values.AsBytes(), r.values.AsBytes()[:r.length*len(r.types)*24])
			r.values.Dec()
		}
		r.values, r.capacity = values, valueCapacity
	}
	if newStringCapacity != stringCapacity {
		strings, ok := allocOperatorSegment(a, r.scope, newStringCapacity)
		if !ok {
			return false
		}
		if r.strings != nil {
			copy(strings.AsBytes(), r.strings.AsBytes()[:r.stringBytes])
			r.strings.Dec()
		}
		r.strings = strings
	}
	return true
}

func (r *packedRows) makeBatch(a *mem.Allocator, schema Schema) *store.Batch {
	vectors := make([]store.Vector, len(r.types))
	for column := range vectors {
		field := schema.FieldAt(column)
		if field.Type().ID() == dtype.STRT {
			strings := make([][]byte, r.length)
			var valid []bool
			if field.Nullable() {
				valid = make([]bool, r.length)
			}
			for i := range r.length {
				value := r.at(i, column)
				if value.IsNull() {
					continue
				}
				strings[i] = borrowedStringBytes(value.text)
				if valid != nil {
					valid[i] = true
				}
			}
			vectors[column] = store.MakeStringVector(a, strings, valid)
		} else {
			vectors[column] = store.MakeVector(a, r.length, field.Type(), field.Nullable())
			if vectors[column].Kind() == store.VectorInvalid {
				break
			}
			for i := range r.length {
				value := r.at(i, column)
				if value.IsNull() {
					vectors[column].Validity().Clear(i)
					continue
				}
				switch field.Type().ID() {
				case dtype.INT32T, dtype.DATET:
					vectors[column].I32s()[i] = value.i32()
				case dtype.INT64T, dtype.TIMESTAMPTZT:
					vectors[column].I64s()[i] = value.i64()
				case dtype.FLOAT32T:
					vectors[column].F32s()[i] = value.f32()
				case dtype.FLOAT64T:
					vectors[column].F64s()[i] = value.f64()
				case dtype.BOOLT:
					if value.boolean() {
						vectors[column].Bools()[i>>3] |= 1 << (i & 7)
					}
				}
			}
		}
	}
	result := store.MakeBatch(vectors)
	if result == nil {
		for i := range vectors {
			vectors[i].Release()
		}
	}
	return result
}
