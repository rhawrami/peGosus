package plan

import (
	"math"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makeCompactSortState(step physicalStep) *compactSortState {
	if len(step.order) != 1 || step.hasTopN || len(step.order[0].program.roots) != 1 {
		return nil
	}
	program := step.order[0].program
	keyType := program.nodes[program.roots[0]].dType
	switch keyType.ID() {
	case dtype.INT32T, dtype.INT64T, dtype.FLOAT32T, dtype.FLOAT64T, dtype.DATET, dtype.TIMESTAMPTZT:
	default:
		return nil
	}
	for _, field := range step.schema.fields {
		if !field.Type().IsFixedSize() {
			return nil
		}
	}
	return &compactSortState{keyType: keyType}
}

type compactSortItem struct {
	key  uint64
	row  uint64
	null uint64
}

type compactSortState struct {
	items    *mem.Segment
	scope    *mem.AllocationScope
	batches  []*store.Batch
	keyType  dtype.Type
	length   int
	capacity int
	chargedB int64
}

func (s *compactSortState) release() {
	if s.items != nil {
		s.items.Dec()
	}
	for _, batch := range s.batches {
		batch.Release()
	}
	*s = compactSortState{}
}

func (s *compactSortState) charged() int64 {
	return saturatingAdd(int64(s.capacity)*24, s.chargedB)
}

func (s *compactSortState) slice() []compactSortItem {
	if s.items == nil {
		return nil
	}
	return unsafe.Slice((*compactSortItem)(unsafe.Pointer(unsafe.SliceData(s.items.AsBytes()))), s.capacity)
}

func (s *compactSortState) grow(a *mem.Allocator, required int, budget int64) bool {
	capacity := s.capacity
	if capacity == 0 {
		capacity = 16
	}
	for capacity < required {
		if capacity > int(^uint(0)>>1)/2 {
			return false
		}
		capacity *= 2
	}
	if capacity > int(^uint(0)>>1)/24 || int64(capacity)*24 > budget-s.charged() {
		return false
	}
	items, ok := allocOperatorSegment(a, s.scope, capacity*24)
	if !ok {
		return false
	}
	if s.items != nil {
		copy(items.AsBytes(), s.items.AsBytes()[:s.length*24])
		s.items.Dec()
	}
	s.items, s.capacity = items, capacity
	return true
}

func (s *compactSortState) add(a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64) bool {
	if uint64(len(s.batches)) >= math.MaxUint32 || uint64(batch.Len()) > math.MaxUint32 {
		return false
	}
	values, ok := executePhysicalExprProgram(a, batch, step.order[0].program)
	if !ok {
		return false
	}
	defer values.release()
	key := &values.vectors[step.order[0].program.roots[0]]
	selection := batch.Selection().RetainBitMap(a)
	if batch.Selection() != nil && selection == nil {
		return false
	}
	if selection != nil {
		defer selection.Release()
	}
	active := batch.ActiveLen()
	if active != 0 && s.length+active > s.capacity && !s.grow(a, s.length+active, budget) {
		return false
	}
	var sourceBytes int64
	for column := range batch.NVectors() {
		vector := batch.VectorAt(column)
		sourceBytes = saturatingAdd(sourceBytes, int64(vector.Data().Len()))
		if vector.Validity() != nil {
			sourceBytes = saturatingAdd(sourceBytes, int64(len(vector.Validity().Bytes())))
		}
	}
	if sourceBytes > budget-s.charged() {
		return false
	}
	batchID := uint64(len(s.batches)) << 32
	s.batches = append(s.batches, batch.Retain())
	s.chargedB = saturatingAdd(s.chargedB, sourceBytes)
	items := s.slice()
	for row := range batch.Len() {
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		item := &items[s.length]
		item.row = batchID | uint64(row)
		item.key, item.null = encodeNumericOrderKey(key, row, step.order[0].descending)
		s.length++
	}
	return true
}

func encodeNumericOrderKey(key *store.Vector, row int, descending bool) (uint64, uint64) {
	if key.Validity() != nil && !key.Validity().IsSet(row) {
		return 0, 1
	}
	value := scalarAt(key, row)
	var encoded uint64
	switch key.TypeID() {
	case dtype.INT32T, dtype.DATET:
		encoded = uint64(uint32(value.bits) ^ 0x80000000)
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		encoded = value.bits ^ 0x8000000000000000
	case dtype.FLOAT32T:
		bits := uint32(value.bits)
		if math.IsNaN(float64(value.f32())) {
			encoded = math.MaxUint32
		} else {
			if value.f32() == 0 {
				bits = 0
			}
			if bits&0x80000000 != 0 {
				encoded = uint64(^bits)
			} else {
				encoded = uint64(bits ^ 0x80000000)
			}
		}
	case dtype.FLOAT64T:
		bits := value.bits
		if math.IsNaN(value.f64()) {
			encoded = math.MaxUint64
		} else {
			if value.f64() == 0 {
				bits = 0
			}
			if bits&0x8000000000000000 != 0 {
				encoded = ^bits
			} else {
				encoded = bits ^ 0x8000000000000000
			}
		}
	}
	if descending {
		encoded = ^encoded
	}
	return encoded, 0
}

func (s *compactSortState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
	if s.length == 0 {
		return nil
	}
	temp := a.AllocSegTemp(s.length * 24)
	if temp == nil {
		return nil
	}
	defer temp.Dec()
	source := s.slice()[:s.length]
	destination := unsafe.Slice((*compactSortItem)(unsafe.Pointer(unsafe.SliceData(temp.AsBytes()))), s.length)
	for shift := uint(0); shift < 64; shift += 8 {
		var counts [256]int
		for _, item := range source {
			counts[byte(item.key>>shift)]++
		}
		position := 0
		for i := range counts {
			n := counts[i]
			counts[i] = position
			position += n
		}
		for _, item := range source {
			bucket := byte(item.key >> shift)
			destination[counts[bucket]] = item
			counts[bucket]++
		}
		source, destination = destination, source
	}
	if step.order[0].nullsFirst {
		on := 0
		for _, item := range source {
			if item.null != 0 {
				destination[on] = item
				on++
			}
		}
		for _, item := range source {
			if item.null == 0 {
				destination[on] = item
				on++
			}
		}
	} else {
		on := 0
		for _, item := range source {
			if item.null == 0 {
				destination[on] = item
				on++
			}
		}
		for _, item := range source {
			if item.null != 0 {
				destination[on] = item
				on++
			}
		}
	}
	vectors := make([]store.Vector, step.schema.Len())
	for column := range vectors {
		field := step.schema.FieldAt(column)
		vectors[column] = store.MakeVector(a, s.length, field.Type(), field.Nullable())
		if vectors[column].Kind() == store.VectorInvalid {
			break
		}
		for i, item := range destination {
			vector := s.batches[item.row>>32].VectorAt(column)
			row := int(uint32(item.row))
			if vector.Validity() != nil && !vector.Validity().IsSet(row) {
				vectors[column].Validity().Clear(i)
				continue
			}
			switch field.Type().ID() {
			case dtype.INT32T, dtype.DATET:
				vectors[column].I32s()[i] = vector.I32s()[row]
			case dtype.INT64T, dtype.TIMESTAMPTZT:
				vectors[column].I64s()[i] = vector.I64s()[row]
			case dtype.FLOAT32T:
				vectors[column].F32s()[i] = vector.F32s()[row]
			case dtype.FLOAT64T:
				vectors[column].F64s()[i] = vector.F64s()[row]
			case dtype.BOOLT:
				if vector.Bools()[row>>3]&(1<<(row&7)) != 0 {
					vectors[column].Bools()[i>>3] |= 1 << (i & 7)
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
