package plan

import (
	"context"
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

const (
	joinOutputBatchRows = 1024
	joinNullRow         = math.MaxUint64
)

func (s *joinState) sourceVector(left *store.Batch, column, leftColumns int, leftRow, rightRow uint64) (*store.Vector, int) {
	if column < leftColumns {
		if leftRow == joinNullRow {
			return nil, 0
		}
		return left.VectorAt(column), int(leftRow)
	}
	if rightRow == joinNullRow {
		return nil, 0
	}
	id := s.rows.AsU64T()[rightRow]
	return s.batches[id>>32].VectorAt(column - leftColumns), int(uint32(id))
}

func (s *joinState) gather(ctx context.Context, a *mem.Allocator, left *store.Batch, schema Schema, leftColumns int, pairs []uint64, budget int64) *store.Batch {
	length := len(pairs) / 2
	var required int64
	for column, field := range schema.fields {
		required = saturatingAdd(required, (int64(length)*int64(field.dType.SlotBits())+7)/8)
		if field.nullable {
			required = saturatingAdd(required, int64((length+7)/8))
		}
		if field.dType.ID() != dtype.STRT {
			continue
		}
		for i := range length {
			if i&1023 == 0 && ctx.Err() != nil {
				return nil
			}
			v, row := s.sourceVector(left, column, leftColumns, pairs[2*i], pairs[2*i+1])
			if v != nil && (v.Validity() == nil || v.Validity().IsSet(row)) {
				n := v.StringAt(row).Len()
				if n > dtype.StringInlineSize {
					required = saturatingAdd(required, int64(n))
				}
			}
		}
	}
	if required > budget {
		return nil
	}
	vectors := make([]store.Vector, schema.Len())
	defer func() {
		for i := range vectors {
			vectors[i].Release()
		}
	}()
	for column, field := range schema.fields {
		if ctx.Err() != nil {
			return nil
		}
		if field.dType.ID() == dtype.STRT {
			var size int
			for i := range length {
				v, row := s.sourceVector(left, column, leftColumns, pairs[2*i], pairs[2*i+1])
				if v != nil && (v.Validity() == nil || v.Validity().IsSet(row)) {
					n := v.StringAt(row).Len()
					if n > dtype.StringInlineSize {
						if n > int(^uint(0)>>1)-size {
							return nil
						}
						size += n
					}
				}
			}
			data := a.AllocSeg(length * 16)
			if data == nil {
				return nil
			}
			clear(data.AsBytes())
			var validity *store.BitMap
			if field.nullable {
				validity = store.MakeBitMap(a, length)
				if validity == nil {
					data.Dec()
					return nil
				}
				validity.SetAll()
			}
			var backing *mem.Segment
			if size > 0 {
				backing = a.AllocSeg(size)
				if backing == nil {
					data.Dec()
					validity.Release()
					return nil
				}
			}
			vectors[column] = store.MakeVectorFromOwnedSegments(field.dType, length, data, validity, backing)
			if vectors[column].Kind() == store.VectorInvalid {
				return nil
			}
			descriptors, on := vectors[column].Strings(), 0
			for i := range length {
				v, row := s.sourceVector(left, column, leftColumns, pairs[2*i], pairs[2*i+1])
				if v == nil || v.Validity() != nil && !v.Validity().IsSet(row) {
					if validity == nil {
						return nil
					}
					validity.Clear(i)
					continue
				}
				text := v.StringAt(row).View()
				if len(text) <= dtype.StringInlineSize {
					descriptors[i] = dtype.MakeString(borrowedStringBytes(text))
				} else {
					payload := backing.AsBytes()[on : on+len(text)]
					copy(payload, text)
					descriptors[i] = dtype.MakeString(payload)
					on += len(text)
				}
			}
			continue
		}
		vectors[column] = store.MakeVector(a, length, field.dType, field.nullable)
		v := &vectors[column]
		if v.Kind() == store.VectorInvalid {
			return nil
		}
		for i := range length {
			if i&1023 == 0 && ctx.Err() != nil {
				return nil
			}
			src, row := s.sourceVector(left, column, leftColumns, pairs[2*i], pairs[2*i+1])
			if src == nil || src.Validity() != nil && !src.Validity().IsSet(row) {
				if v.Validity() == nil {
					return nil
				}
				v.Validity().Clear(i)
				continue
			}
			switch field.dType.ID() {
			case dtype.INT32T, dtype.DATET, dtype.FLOAT32T:
				v.Data().AsU32T()[i] = src.Data().AsU32T()[row]
			case dtype.INT64T, dtype.TIMESTAMPTZT, dtype.FLOAT64T:
				v.Data().AsU64T()[i] = src.Data().AsU64T()[row]
			case dtype.BOOLT:
				if src.Bools()[row>>3]&(1<<(row&7)) != 0 {
					v.Bools()[i>>3] |= 1 << (i & 7)
				}
			}
		}
	}
	result := store.MakeBatch(vectors)
	if result != nil {
		for i := range vectors {
			vectors[i] = store.Vector{}
		}
	}
	return result
}
