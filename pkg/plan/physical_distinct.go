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
}

func (s *distinctState) add(a *mem.Allocator, batch *store.Batch, row int, budget int64) (int, bool) {
	length := encodedRowKeyLength(batch, row)
	if int64(length) > budget-s.charged {
		return 0, false
	}
	buffer, on := encodeRowKey(a, batch, row, length)
	if buffer == nil {
		return 0, false
	}
	defer buffer.Dec()
	bytes := buffer.AsBytes()
	if index, exists := s.index.lookup(bytes[:on], &s.keys); exists {
		return index, true
	}
	priorKeyBytes, priorIndexBytes := s.keys.bytes(), s.index.bytes()
	other := s.charged - priorKeyBytes - priorIndexBytes
	if !s.keys.append(a, bytes[:on], budget-other-priorIndexBytes) || !s.index.put(a, bytes[:on], s.rows.length, budget-other-s.keys.bytes()) || !s.rows.append(a, batch, row, budget-s.keys.bytes()-s.index.bytes()) {
		return 0, false
	}
	s.charged = saturatingAdd(saturatingAdd(s.keys.bytes(), s.index.bytes()), s.rows.bytes())
	return s.rows.length - 1, true
}

func (s *distinctState) lookup(a *mem.Allocator, batch *store.Batch, row int) (int, bool) {
	buffer, on := encodeRowKey(a, batch, row, encodedRowKeyLength(batch, row))
	if buffer == nil {
		return 0, false
	}
	index, ok := s.index.lookup(buffer.AsBytes()[:on], &s.keys)
	buffer.Dec()
	return index, ok
}

func encodeRowKey(a *mem.Allocator, batch *store.Batch, row, length int) (*mem.Segment, int) {
	buffer := a.AllocSegTemp(length)
	bytes := buffer.AsBytes()[:length]
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
	return buffer, on
}

func encodedRowKeyLength(batch *store.Batch, row int) int {
	length := 0
	for column := range batch.NVectors() {
		length += 9
		v := batch.VectorAt(column)
		if v.TypeID() == dtype.STRT && (v.Validity() == nil || v.Validity().IsSet(row)) {
			length += len(v.Strings()[row].View())
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
