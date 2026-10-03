package plan

import (
	"encoding/binary"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

// takeRecord transfers the current frame to an owned row, invalidating borrowed views.
func (s *spillStream) takeRecord(a *mem.Allocator, schema Schema) (*store.Batch, error) {
	if !s.recordReady || s.recordFrame == nil || !schema.Valid() || schema.Len() != s.recordColumns {
		return nil, errSpillFormat
	}
	frame, length := s.recordFrame, s.recordLength
	s.recordFrame = nil
	s.recordReady = false
	s.recordLength, s.recordColumns = 0, 0
	if s.recordOffsets != nil {
		s.recordOffsets.Dec()
		s.recordOffsets = nil
	}
	defer frame.Dec()
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if schema.Len() == 0 {
		return store.MakeBatchWithLength(nil, 1), nil
	}
	return decodeSpillFrame(a, schema, frame, length)
}

func decodeSpillFrame(a *mem.Allocator, schema Schema, frame *mem.Segment, length int) (*store.Batch, error) {
	vectors := make([]store.Vector, schema.Len())
	defer func() {
		for i := range vectors {
			vectors[i].Release()
		}
	}()
	data, at := frame.AsBytes()[:length], 0
	for column, field := range schema.fields {
		if at > len(data)-9 || data[at] > 1 {
			return nil, errSpillFormat
		}
		null := data[at] != 0
		bits := binary.LittleEndian.Uint64(data[at+1:])
		at += 9
		if null && !field.Nullable() {
			return nil, errSpillFormat
		}
		if field.Type().ID() == dtype.STRT {
			if bits > uint64(len(data)-at) {
				return nil, errSpillFormat
			}
			descriptors := a.AllocSeg(16)
			if descriptors == nil {
				return nil, errSpillBudget
			}
			clear(descriptors.AsBytes())
			var validity *store.BitMap
			if field.Nullable() {
				validity = store.MakeBitMap(a, 1)
				if validity == nil {
					descriptors.Dec()
					return nil, errSpillBudget
				}
				if !null {
					validity.SetAll()
				}
			}
			frame.Inc()
			vectors[column] = store.MakeVectorFromOwnedSegments(field.Type(), 1, descriptors, validity, frame)
			if !null {
				vectors[column].Strings()[0] = dtype.MakeString(data[at : at+int(bits)])
				at += int(bits)
			} else if bits != 0 {
				return nil, errSpillFormat
			}
		} else {
			vectors[column] = store.MakeVector(a, 1, field.Type(), field.Nullable())
			if vectors[column].Kind() == store.VectorInvalid {
				return nil, errSpillBudget
			}
			if null {
				vectors[column].Validity().Clear(0)
				continue
			}
			switch field.Type().ID() {
			case dtype.INT32T, dtype.DATET, dtype.FLOAT32T:
				binary.NativeEndian.PutUint32(vectors[column].Data().AsBytes(), uint32(bits))
			case dtype.INT64T, dtype.TIMESTAMPTZT, dtype.FLOAT64T:
				binary.NativeEndian.PutUint64(vectors[column].Data().AsBytes(), bits)
			case dtype.BOOLT:
				if bits > 1 {
					return nil, errSpillFormat
				}
				vectors[column].Bools()[0] = byte(bits)
			default:
				return nil, errSpillFormat
			}
		}
	}
	if at != len(data) {
		return nil, errSpillFormat
	}
	batch := store.MakeBatch(vectors)
	if batch == nil {
		return nil, errSpillFormat
	}
	vectors = nil
	return batch, nil
}
