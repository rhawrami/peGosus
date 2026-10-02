package plan

import (
	"encoding/binary"
	"math"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type distinctRow struct {
	values []Scalar
	text   []*mem.Segment
}

type distinctState struct {
	index   keyIndex
	keys    keyArena
	rows    packedRows
	scratch *mem.Segment
	charged int64
	scope   *mem.AllocationScope
}

func (s *distinctState) setScope(scope *mem.AllocationScope) {
	s.scope, s.index.scope, s.keys.scope, s.rows.scope = scope, scope, scope, scope
}

func (s *distinctState) release() {
	s.index.release()
	s.keys.release()
	s.rows.release()
	if s.scratch != nil {
		s.scratch.Dec()
		s.scratch = nil
	}
}

func (s *distinctState) add(a *mem.Allocator, batch *store.Batch, row int, budget int64) (int, bool) {
	length := encodedRowKeyLength(batch, row)
	if int64(length) > budget-s.charged {
		return 0, false
	}
	bytes := s.keyBuffer(a, length, budget)
	if bytes == nil {
		return 0, false
	}
	on := encodeRowKey(bytes, batch, row)
	if index, exists := s.index.lookup(bytes[:on], &s.keys); exists {
		return index, true
	}
	return s.appendRow(a, bytes[:on], batch, row, budget)
}

func (s *distinctState) addString(a *mem.Allocator, batch *store.Batch, row int, budget int64) (int, bool) {
	key := batch.VectorAt(0)
	isNull := key.Validity() != nil && !key.Validity().IsSet(row)
	var text string
	if !isNull {
		text = key.StringAt(row).View()
	}
	if index, exists := s.index.lookupString(text, isNull, &s.keys); exists {
		return index, true
	}
	length := 1
	if !isNull {
		if len(text) > int(^uint(0)>>1)-9 {
			return 0, false
		}
		length = 9 + len(text)
	}
	if int64(length) > budget-s.charged {
		return 0, false
	}
	encoded := s.keyBuffer(a, length, budget)
	if encoded == nil {
		return 0, false
	}
	encoded[0] = 0
	if !isNull {
		encoded[0] = 1
		binary.LittleEndian.PutUint64(encoded[1:], uint64(len(text)))
		copy(encoded[9:], text)
	}
	return s.appendRow(a, encoded, batch, row, budget)
}

func (s *distinctState) appendRow(a *mem.Allocator, encoded []byte, batch *store.Batch, row int, budget int64) (int, bool) {
	priorKeyBytes, priorIndexBytes := s.keys.bytes(), s.index.bytes()
	other := s.charged - priorKeyBytes - priorIndexBytes
	if !s.keys.append(a, encoded, budget-other-priorIndexBytes) || !s.index.put(a, encoded, s.rows.length, budget-other-s.keys.bytes()) || !s.rows.append(a, batch, row, budget-s.keys.bytes()-s.index.bytes()) {
		return 0, false
	}
	s.charged = saturatingAdd(saturatingAdd(s.keys.bytes(), s.index.bytes()), s.rows.bytes()+int64(s.scratch.Len()))
	return s.rows.length - 1, true
}

func (s *distinctState) addEncoded(a *mem.Allocator, encoded []byte, rows *packedRows, row int, values []Scalar, budget int64) (int, bool) {
	if len(values) != len(rows.types) || int64(len(encoded)) > budget-s.charged {
		return 0, false
	}
	if index, exists := s.index.lookup(encoded, &s.keys); exists {
		return index, true
	}
	priorKeyBytes, priorIndexBytes := s.keys.bytes(), s.index.bytes()
	other := s.charged - priorKeyBytes - priorIndexBytes
	if !s.keys.append(a, encoded, budget-other-priorIndexBytes) || !s.index.put(a, encoded, s.rows.length, budget-other-s.keys.bytes()) {
		return 0, false
	}
	for i := range values {
		values[i] = rows.at(row, i)
	}
	if !s.rows.appendScalars(a, values, budget-s.keys.bytes()-s.index.bytes()) {
		return 0, false
	}
	scratchBytes := int64(0)
	if s.scratch != nil {
		scratchBytes = int64(s.scratch.Len())
	}
	s.charged = saturatingAdd(saturatingAdd(s.keys.bytes(), s.index.bytes()), s.rows.bytes()+scratchBytes)
	return s.rows.length - 1, true
}

func (s *distinctState) lookup(a *mem.Allocator, batch *store.Batch, row int) (int, bool) {
	bytes := s.keyBuffer(a, encodedRowKeyLength(batch, row), int64(^uint64(0)>>1))
	if bytes == nil {
		return 0, false
	}
	on := encodeRowKey(bytes, batch, row)
	return s.index.lookup(bytes[:on], &s.keys)
}

func (s *distinctState) keyBuffer(a *mem.Allocator, length int, budget int64) []byte {
	if length <= 0 {
		return nil
	}
	if s.scratch != nil && s.scratch.Len() >= length {
		return s.scratch.AsBytes()[:length]
	}
	capacity := max(64, length)
	if s.scratch != nil && s.scratch.Len() <= int(^uint(0)>>1)/2 {
		capacity = max(capacity, s.scratch.Len()*2)
	}
	if int64(capacity) > budget-s.charged {
		return nil
	}
	var next *mem.Segment
	if s.scope != nil {
		next, _ = s.scope.AllocSegTemp(capacity)
	} else {
		next = a.AllocSegTemp(capacity)
	}
	if next == nil {
		return nil
	}
	if s.scratch != nil {
		s.charged -= int64(s.scratch.Len())
		s.scratch.Dec()
	}
	s.scratch = next
	s.charged = saturatingAdd(s.charged, int64(capacity))
	return next.AsBytes()[:length]
}

func encodeRowKey(bytes []byte, batch *store.Batch, row int) int {
	on := 0
	for column := range batch.NVectors() {
		value := scalarAt(batch.VectorAt(column), row)
		if value.IsNull() {
			bytes[on] = 0
			on++
			continue
		}
		bytes[on] = 1
		on++
		switch value.Type().ID() {
		case dtype.INT32T, dtype.DATET:
			binary.LittleEndian.PutUint32(bytes[on:], uint32(value.bits))
			on += 4
		case dtype.INT64T, dtype.TIMESTAMPTZT:
			binary.LittleEndian.PutUint64(bytes[on:], value.bits)
			on += 8
		case dtype.FLOAT32T:
			bits := uint32(value.bits)
			if math.IsNaN(float64(value.f32())) {
				bits = 0x7fc00000
			}
			if value.f32() == 0 {
				bits = 0
			}
			binary.LittleEndian.PutUint32(bytes[on:], bits)
			on += 4
		case dtype.FLOAT64T:
			bits := value.bits
			if math.IsNaN(value.f64()) {
				bits = 0x7ff8000000000000
			}
			if value.f64() == 0 {
				bits = 0
			}
			binary.LittleEndian.PutUint64(bytes[on:], bits)
			on += 8
		case dtype.BOOLT:
			bytes[on] = byte(value.bits)
			on++
		case dtype.STRT:
			binary.LittleEndian.PutUint64(bytes[on:], uint64(len(value.text)))
			on += 8
			on += copy(bytes[on:], value.text)
		}
	}
	return on
}

func encodedRowKeyLength(batch *store.Batch, row int) int {
	length := 0
	for column := range batch.NVectors() {
		length += 9
		v := batch.VectorAt(column)
		if v.TypeID() == dtype.STRT && (v.Validity() == nil || v.Validity().IsSet(row)) {
			length += len(v.StringAt(row).View())
		}
	}
	return length
}

func ownScalar(a *mem.Allocator, value Scalar) (Scalar, *mem.Segment, bool) {
	if value.Type().ID() != dtype.STRT || value.IsNull() {
		return value, nil, true
	}
	stringBytes := borrowedStringBytes(value.text)
	text := a.AllocSeg(len(stringBytes))
	if text == nil {
		return Scalar{}, nil, false
	}
	copy(text.AsBytes(), stringBytes)
	value.text = unsafe.String(unsafe.SliceData(text.AsBytes()), len(stringBytes))
	return value, text, true
}

func makeDistinctBatch(a *mem.Allocator, schema Schema, rows []distinctRow) *store.Batch {
	vectors := make([]store.Vector, schema.Len())
	for column := range vectors {
		field := schema.FieldAt(column)
		if field.Type().ID() == dtype.STRT {
			strings := make([][]byte, len(rows))
			var valid []bool
			if field.Nullable() {
				valid = make([]bool, len(rows))
			}
			for row, record := range rows {
				if record.values[column].IsNull() {
					continue
				}
				strings[row] = borrowedStringBytes(record.values[column].text)
				if valid != nil {
					valid[row] = true
				}
			}
			vectors[column] = store.MakeStringVector(a, strings, valid)
		} else {
			vectors[column] = store.MakeVector(a, len(rows), field.Type(), field.Nullable())
			if vectors[column].Kind() == store.VectorInvalid {
				break
			}
			for row, record := range rows {
				value := record.values[column]
				if value.IsNull() {
					vectors[column].Validity().Clear(row)
					continue
				}
				switch field.Type().ID() {
				case dtype.INT32T, dtype.DATET:
					vectors[column].I32s()[row] = value.i32()
				case dtype.INT64T, dtype.TIMESTAMPTZT:
					vectors[column].I64s()[row] = value.i64()
				case dtype.FLOAT32T:
					vectors[column].F32s()[row] = value.f32()
				case dtype.FLOAT64T:
					vectors[column].F64s()[row] = value.f64()
				case dtype.BOOLT:
					if value.boolean() {
						vectors[column].Bools()[row>>3] |= 1 << (row & 7)
					}
				}
			}
		}
	}
	batch := store.MakeBatch(vectors)
	if batch == nil {
		for i := range vectors {
			vectors[i].Release()
		}
	}
	return batch
}
