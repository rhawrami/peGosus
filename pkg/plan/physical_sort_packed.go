package plan

import (
	"context"
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makePackedSortState(ctx context.Context, a *mem.Allocator, source *store.Table, step physicalStep, scope *mem.AllocationScope) (*packedSortState, bool) {
	if step.operation != physicalSort || step.hasTopN || source.NBatches() == 0 || len(step.order) == 0 {
		return nil, false
	}
	types := make([]dtype.Type, len(step.order))
	offsets := make([]int, len(step.order))
	descending, nullsFirst := make([]bool, len(step.order)), make([]bool, len(step.order))
	for i, order := range step.order {
		if len(order.program.roots) != 1 {
			return nil, false
		}
		root := order.program.nodes[order.program.roots[0]]
		if root.kind != exprColumn || root.column < 0 || root.column >= source.NColumns() {
			return nil, false
		}
		types[i], offsets[i] = root.dType, root.column
		descending[i], nullsFirst[i] = order.descending, order.nullsFirst
	}
	stats := makePackedKeyStats(types)
	for i := range source.NBatches() {
		if ctx.Err() != nil {
			return nil, false
		}
		batch := source.BatchAt(i)
		if uint64(batch.Len()) > math.MaxUint32 {
			return nil, false
		}
		keys := make([]store.Vector, len(offsets))
		for j, offset := range offsets {
			keys[j] = *batch.VectorAt(offset)
		}
		if !stats.observe(a, keys, batch.Selection()) {
			return nil, false
		}
	}
	layout, ok := stats.layout(keyOrder, descending, nullsFirst)
	if !ok {
		return nil, false
	}
	return &packedSortState{layout: layout, offsets: offsets, source: source, scope: scope}, true
}

type packedSortState struct {
	layout   packedKeyLayout
	offsets  []int
	source   *store.Table
	scope    *mem.AllocationScope
	items    *mem.Segment
	length   int
	capacity int
	batches  int
}

func (s *packedSortState) release() {
	if s.items != nil {
		s.items.Dec()
	}
	*s = packedSortState{}
}

func (s *packedSortState) charged() int64 {
	return int64(s.capacity) * 16
}

func (s *packedSortState) grow(a *mem.Allocator, required int, budget int64) bool {
	capacity := max(16, s.capacity)
	for capacity < required {
		if capacity > int(^uint(0)>>1)/2 {
			return false
		}
		capacity *= 2
	}
	if capacity > int(^uint(0)>>1)/16 || int64(capacity)*16 > budget-s.charged() {
		return false
	}
	items, ok := allocOperatorSegment(a, s.scope, capacity*16)
	if !ok {
		return false
	}
	if s.items != nil {
		old, newData := s.items.AsU64T(), items.AsU64T()
		copy(newData[:s.length], old[:s.length])
		copy(newData[capacity:capacity+s.length], old[s.capacity:s.capacity+s.length])
		s.items.Dec()
	}
	s.items, s.capacity = items, capacity
	return true
}

func (s *packedSortState) add(a *mem.Allocator, batch *store.Batch, budget int64) bool {
	if uint64(s.batches) >= math.MaxUint32 || uint64(batch.Len()) > math.MaxUint32 {
		return false
	}
	selection := batch.Selection().RetainBitMap(a)
	if batch.Selection() != nil && selection == nil {
		return false
	}
	if selection != nil {
		defer selection.Release()
	}
	active := batch.ActiveLen()
	if active > int(^uint(0)>>1)-s.length {
		return false
	}
	if active > 0 && s.length+active > s.capacity && !s.grow(a, s.length+active, budget) {
		return false
	}
	var keys, ids []uint64
	if s.items != nil {
		items := s.items.AsU64T()
		keys, ids = items[:s.capacity], items[s.capacity:]
	}
	columns := make([]store.Vector, len(s.offsets))
	for i, offset := range s.offsets {
		columns[i] = *batch.VectorAt(offset)
	}
	for row := range batch.Len() {
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		key, ok := s.layout.encode(columns, row)
		if !ok {
			return false
		}
		keys[s.length], ids[s.length] = key, uint64(s.batches)<<32|uint64(row)
		s.length++
	}
	s.batches++
	return true
}

func (s *packedSortState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
	if s.length == 0 {
		return nil
	}
	data := s.items.AsU64T()
	keys, ids := data[:s.length], data[s.capacity:s.capacity+s.length]
	if !sortPackedKeyIDs(a, keys, ids, s.layout.width) {
		return nil
	}
	vectors := make([]store.Vector, step.schema.Len())
	for column := range vectors {
		field := step.schema.FieldAt(column)
		if field.Type().ID() == dtype.STRT {
			strings := make([][]byte, s.length)
			var valid []bool
			if field.Nullable() {
				valid = make([]bool, s.length)
			}
			for i, id := range ids {
				src := s.source.BatchAt(int(id >> 32)).VectorAt(column)
				row := int(uint32(id))
				if src.Validity() != nil && !src.Validity().IsSet(row) {
					continue
				}
				strings[i] = borrowedStringBytes(src.StringAt(row).View())
				if valid != nil {
					valid[i] = true
				}
			}
			vectors[column] = store.MakeStringVector(a, strings, valid)
		} else {
			vectors[column] = store.MakeVector(a, s.length, field.Type(), field.Nullable())
			if vectors[column].Kind() == store.VectorInvalid {
				break
			}
			for i, id := range ids {
				src := s.source.BatchAt(int(id >> 32)).VectorAt(column)
				row := int(uint32(id))
				if src.Validity() != nil && !src.Validity().IsSet(row) {
					vectors[column].Validity().Clear(i)
					continue
				}
				switch field.Type().ID() {
				case dtype.INT32T, dtype.DATET:
					vectors[column].I32s()[i] = src.I32s()[row]
				case dtype.INT64T, dtype.TIMESTAMPTZT:
					vectors[column].I64s()[i] = src.I64s()[row]
				case dtype.FLOAT32T:
					vectors[column].F32s()[i] = src.F32s()[row]
				case dtype.FLOAT64T:
					vectors[column].F64s()[i] = src.F64s()[row]
				case dtype.BOOLT:
					if src.Bools()[row>>3]&(1<<(row&7)) != 0 {
						vectors[column].Bools()[i>>3] |= 1 << (i & 7)
					}
				default:
					for j := range vectors {
						vectors[j].Release()
					}
					return nil
				}
			}
		}
		if vectors[column].Kind() == store.VectorInvalid {
			break
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
