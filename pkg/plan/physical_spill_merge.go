package plan

import (
	"encoding/binary"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (s *spillStream) writeJoinedRecords(left, right *spillStream, keys int) error {
	if !left.recordReady || !right.recordReady || keys > left.recordColumns || keys < 0 {
		return errSpillFormat
	}
	offset := right.recordColumnOffset(keys)
	if offset < 0 {
		return errSpillFormat
	}
	l, r := left.recordBytes(), right.recordBytes()[offset:]
	var header [8]byte
	binary.LittleEndian.PutUint64(header[:], uint64(len(l))+uint64(len(r)))
	if err := s.write(header[:]); err != nil {
		return err
	}
	if err := s.write(l); err != nil {
		return err
	}
	return s.write(r)
}

func (s *spillStream) writeAggregateRow(keys *packedRows, values *store.Batch) error {
	columns := len(keys.types) + values.NVectors()
	length := uint64(columns) * 9
	for col := range columns {
		var value Scalar
		if col < len(keys.types) {
			value = keys.at(0, col)
		} else {
			value = scalarAt(values.VectorAt(col-len(keys.types)), 0)
		}
		length += uint64(len(value.text))
	}
	var header [9]byte
	binary.LittleEndian.PutUint64(header[:8], length)
	if err := s.write(header[:8]); err != nil {
		return err
	}
	for col := range columns {
		var value Scalar
		if col < len(keys.types) {
			value = keys.at(0, col)
		} else {
			value = scalarAt(values.VectorAt(col-len(keys.types)), 0)
		}
		clear(header[:])
		if value.IsNull() {
			header[0] = 1
		}
		bits := value.bits
		if value.Type().ID() == dtype.STRT {
			bits = uint64(len(value.text))
		}
		binary.LittleEndian.PutUint64(header[1:], bits)
		if err := s.write(header[:]); err != nil {
			return err
		}
		if len(value.text) != 0 {
			if err := s.write(borrowedStringBytes(value.text)); err != nil {
				return err
			}
		}
	}
	return nil
}
