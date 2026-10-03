package plan

import (
	"encoding/binary"
	"io"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func spillRecordCapacity(prior, required, limit int) int {
	capacity := max(64, prior)
	for capacity < required {
		if capacity > limit/2 {
			return limit
		}
		capacity *= 2
	}
	return min(capacity, limit)
}

// nextRecord reads a borrowed record whose values expire on advance or release.
func (s *spillStream) nextRecord(a *mem.Allocator, schema Schema, budget int64) (bool, error) {
	s.recordReady = false
	s.recordLength, s.recordColumns = 0, 0
	if !schema.Valid() || schema.Len() > int(^uint(0)>>1)/8 {
		return false, errSpillFormat
	}
	if budget <= 0 || s.buffer == nil || int64(s.buffer.Len()) > budget {
		return false, errSpillBudget
	}
	var header [8]byte
	if err := s.read(header[:]); err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	length := binary.LittleEndian.Uint64(header[:])
	if length < uint64(schema.Len())*9 || length > uint64(^uint(0)>>1) {
		return false, errSpillFormat
	}
	limit := int(min(budget/4, int64(^uint(0)>>1)))
	if length > uint64(limit) {
		return false, errSpillBudget
	}
	frameCapacity, offsetCapacity := 0, 0
	if s.recordFrame != nil {
		frameCapacity = s.recordFrame.Len()
	}
	if s.recordOffsets != nil {
		offsetCapacity = s.recordOffsets.Len()
	}
	frameCapacity = spillRecordCapacity(frameCapacity, int(length), limit)
	if schema.Len() != 0 {
		offsetCapacity = spillRecordCapacity(offsetCapacity, schema.Len()*8, limit&^7)
	} else {
		offsetCapacity = 0
	}
	if int64(frameCapacity)+int64(offsetCapacity)+int64(s.buffer.Len()) > budget {
		return false, errSpillBudget
	}
	if s.recordFrame == nil || s.recordFrame.Len() != frameCapacity {
		if s.recordFrame != nil {
			s.recordFrame.Dec()
			s.recordFrame = nil
		}
		s.recordFrame = a.AllocSegTemp(frameCapacity)
		if s.recordFrame == nil {
			return false, errSpillBudget
		}
	}
	if s.recordOffsets == nil && offsetCapacity != 0 || s.recordOffsets != nil && s.recordOffsets.Len() != offsetCapacity {
		if s.recordOffsets != nil {
			s.recordOffsets.Dec()
			s.recordOffsets = nil
		}
		if offsetCapacity != 0 {
			s.recordOffsets = a.AllocSegTemp(offsetCapacity)
			if s.recordOffsets == nil {
				return false, errSpillBudget
			}
		}
	}
	data := s.recordFrame.AsBytes()[:int(length)]
	if err := s.read(data); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return false, err
	}
	var offsets []uint64
	if s.recordOffsets != nil {
		offsets = s.recordOffsets.AsU64T()
	}
	at := 0
	for column, field := range schema.fields {
		if at > len(data)-9 || data[at] > 1 {
			return false, errSpillFormat
		}
		offsets[column] = uint64(at)
		null := data[at] != 0
		bits := binary.LittleEndian.Uint64(data[at+1:])
		at += 9
		if null && !field.Nullable() {
			return false, errSpillFormat
		}
		switch field.Type().ID() {
		case dtype.STRT:
			if bits > uint64(len(data)-at) || null && bits != 0 {
				return false, errSpillFormat
			}
			at += int(bits)
		case dtype.BOOLT:
			if !null && bits > 1 {
				return false, errSpillFormat
			}
		case dtype.INT32T, dtype.DATET, dtype.FLOAT32T, dtype.INT64T, dtype.TIMESTAMPTZT, dtype.FLOAT64T:
		default:
			return false, errSpillFormat
		}
	}
	if at != len(data) {
		return false, errSpillFormat
	}
	s.recordReady = true
	s.recordLength, s.recordColumns = int(length), schema.Len()
	return true, nil
}

// recordValue returns a scalar whose string bytes borrow the current record.
func (s *spillStream) recordValue(schema Schema, column int) Scalar {
	if !s.recordReady || column < 0 || column >= s.recordColumns || schema.Len() != s.recordColumns {
		return Scalar{}
	}
	at := int(s.recordOffsets.AsU64T()[column])
	data := s.recordFrame.AsBytes()[:s.recordLength]
	field := schema.FieldAt(column)
	if data[at] != 0 {
		return MakeNullScalar(field.Type())
	}
	bits := binary.LittleEndian.Uint64(data[at+1:])
	if field.Type().ID() == dtype.STRT {
		payload := data[at+9 : at+9+int(bits)]
		return Scalar{dType: field.Type(), text: unsafe.String(unsafe.SliceData(payload), len(payload))}
	}
	if field.Type().ID() == dtype.INT32T || field.Type().ID() == dtype.FLOAT32T || field.Type().ID() == dtype.DATET {
		bits = uint64(uint32(bits))
	}
	return Scalar{dType: field.Type(), bits: bits}
}

// recordBytes returns the current borrowed wire body without its length header.
func (s *spillStream) recordBytes() []byte {
	if !s.recordReady {
		return nil
	}
	return s.recordFrame.AsBytes()[:s.recordLength]
}

// recordColumnOffset returns a field header offset, or the body end for the final boundary.
func (s *spillStream) recordColumnOffset(column int) int {
	if !s.recordReady || column < 0 || column > s.recordColumns {
		return -1
	}
	if column == s.recordColumns {
		return s.recordLength
	}
	return int(s.recordOffsets.AsU64T()[column])
}

func (s *spillStream) writeBytes(body []byte) error {
	var header [8]byte
	binary.LittleEndian.PutUint64(header[:], uint64(len(body)))
	if err := s.write(header[:]); err != nil {
		return err
	}
	return s.write(body)
}
