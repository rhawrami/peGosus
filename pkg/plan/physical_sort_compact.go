package plan

import (
	"context"
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makeCompactSortState(step physicalStep) *compactSortState {
	if len(step.order) == 0 || step.hasTopN {
		return nil
	}
	keys := make([]compactSortKey, len(step.order))
	for i, order := range step.order {
		if len(order.program.roots) != 1 {
			return nil
		}
		keyType := order.program.nodes[order.program.roots[0]].dType
		if !keyType.CanOrder() {
			return nil
		}
		keys[i].stringKey = keyType.ID() == dtype.STRT
	}
	return &compactSortState{keys: keys}
}

type compactSortKey struct {
	values    *mem.Segment
	vectors   []store.Vector
	stringKey bool
}

type compactSortState struct {
	items    *mem.Segment
	rows     *mem.Segment
	keys     []compactSortKey
	scope    *mem.AllocationScope
	ctx      context.Context
	batches  []*store.Batch
	length   int
	capacity int
	chargedB int64
}

func (s *compactSortState) release() {
	if s.items != nil {
		s.items.Dec()
	}
	if s.rows != nil {
		s.rows.Dec()
	}
	for i := range s.keys {
		if s.keys[i].values != nil {
			s.keys[i].values.Dec()
		}
		for j := range s.keys[i].vectors {
			s.keys[i].vectors[j].Release()
		}
	}
	for _, batch := range s.batches {
		batch.Release()
	}
	*s = compactSortState{}
}

func (s *compactSortState) charged() int64 {
	perRow := int64(16)
	for _, key := range s.keys {
		if !key.stringKey {
			perRow += 16
		}
	}
	return saturatingAdd(saturatingMultiply(int64(s.capacity), perRow), s.chargedB)
}

func (s *compactSortState) grow(a *mem.Allocator, required int, budget int64) bool {
	capacity := max(16, s.capacity)
	for capacity < required {
		if capacity > int(^uint(0)>>1)/2 {
			return false
		}
		capacity *= 2
	}
	perRow := 16
	for _, key := range s.keys {
		if !key.stringKey {
			perRow += 16
		}
	}
	if capacity > int(^uint(0)>>1)/perRow || int64(capacity)*int64(perRow) > budget-s.charged() {
		return false
	}
	items, ok := allocOperatorSegment(a, s.scope, capacity*8)
	if !ok || items == nil {
		return false
	}
	rows, ok := allocOperatorSegment(a, s.scope, capacity*8)
	if !ok || rows == nil {
		items.Dec()
		return false
	}
	values := make([]*mem.Segment, len(s.keys))
	for i := range values {
		if s.keys[i].stringKey {
			continue
		}
		values[i], ok = allocOperatorSegment(a, s.scope, capacity*16)
		if !ok || values[i] == nil {
			items.Dec()
			rows.Dec()
			for _, value := range values {
				if value != nil {
					value.Dec()
				}
			}
			return false
		}
		if s.keys[i].values != nil {
			copy(values[i].AsU64T(), s.keys[i].values.AsU64T()[:s.length*2])
		}
	}
	if s.items != nil {
		copy(items.AsU64T(), s.items.AsU64T()[:s.length])
		copy(rows.AsU64T(), s.rows.AsU64T()[:s.length])
		s.items.Dec()
		s.rows.Dec()
	}
	for i := range values {
		if s.keys[i].values != nil {
			s.keys[i].values.Dec()
		}
		s.keys[i].values = values[i]
	}
	s.items, s.rows, s.capacity = items, rows, capacity
	return true
}

func sortVectorBytes(vector *store.Vector) int64 {
	n := int64(vector.Data().Len())
	if vector.Backing() != nil {
		n = saturatingAdd(n, int64(vector.Backing().Len()))
	}
	if vector.Validity() != nil {
		n = saturatingAdd(n, int64(len(vector.Validity().Bytes())))
	}
	if vector.Dictionary() != nil {
		n = saturatingAdd(n, sortVectorBytes(vector.Dictionary()))
	}
	return n
}

func (s *compactSortState) add(a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64) bool {
	active := batch.ActiveLen()
	if active == 0 {
		return true
	}
	if active > int(^uint(0)>>1)-s.length || uint64(len(s.batches)) >= math.MaxUint32 || uint64(batch.Len()) > math.MaxUint32 {
		return false
	}
	if s.length+active > s.capacity && !s.grow(a, s.length+active, budget) {
		return false
	}
	selection := batch.Selection().RetainBitMap(a)
	if batch.Selection() != nil && selection == nil {
		return false
	}
	if selection != nil {
		defer selection.Release()
	}
	var sourceBytes int64
	for column := range batch.NVectors() {
		sourceBytes = saturatingAdd(sourceBytes, sortVectorBytes(batch.VectorAt(column)))
	}
	if batch.Selection() != nil {
		sourceBytes = saturatingAdd(sourceBytes, int64(batch.Len())*4)
	}
	if sourceBytes > budget-s.charged() {
		return false
	}
	computed := make([]physicalExprValues, len(step.order))
	defer func() {
		for i := range computed {
			computed[i].release()
		}
	}()
	for i, order := range step.order {
		var ok bool
		computed[i], ok = executePhysicalExprProgram(a, batch, order.program)
		if !ok {
			return false
		}
		key := &computed[i].vectors[order.program.roots[0]]
		if s.keys[i].stringKey {
			if order.program.nodes[order.program.roots[0]].kind != exprColumn {
				sourceBytes = saturatingAdd(sourceBytes, sortVectorBytes(key))
			}
			if sourceBytes > budget-s.charged() {
				return false
			}
			continue
		}
		values := s.keys[i].values.AsU64T()
		on := s.length
		for row := range batch.Len() {
			if selection != nil && !selection.IsSet(row) {
				continue
			}
			values[on*2], values[on*2+1] = encodeNumericOrderKey(key, row, order.descending)
			on++
		}
	}
	batchID := uint64(len(s.batches)) << 32
	for i, order := range step.order {
		if s.keys[i].stringKey {
			s.keys[i].vectors = append(s.keys[i].vectors, computed[i].vectors[order.program.roots[0]].Retain())
		}
	}
	s.batches = append(s.batches, batch.Retain())
	s.chargedB = saturatingAdd(s.chargedB, sourceBytes)
	items, rows := s.items.AsU64T(), s.rows.AsU64T()
	for row := range batch.Len() {
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		items[s.length], rows[s.length] = uint64(s.length), batchID|uint64(row)
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

func (s *compactSortState) cancelled() bool { return s.ctx != nil && s.ctx.Err() != nil }

func (s *compactSortState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
	if s.length == 0 || s.cancelled() {
		return nil
	}
	temp := a.AllocSegTemp(s.length * 8)
	if temp == nil {
		return nil
	}
	defer temp.Dec()
	source, destination := s.items.AsU64T()[:s.length], temp.AsU64T()
	rows := s.rows.AsU64T()
	for k := len(s.keys) - 1; k >= 0; k-- {
		if s.cancelled() {
			return nil
		}
		key, order := &s.keys[k], step.order[k]
		if key.stringKey {
			less := func(left, right uint64) bool {
				l, r := rows[left], rows[right]
				lv, rv := &key.vectors[l>>32], &key.vectors[r>>32]
				li, ri := int(uint32(l)), int(uint32(r))
				ln, rn := lv.Validity() != nil && !lv.Validity().IsSet(li), rv.Validity() != nil && !rv.Validity().IsSet(ri)
				if ln != rn {
					return ln == order.nullsFirst
				}
				if ln {
					return false
				}
				ltext, rtext := lv.StringAt(li).View(), rv.StringAt(ri).View()
				if order.descending {
					return ltext > rtext
				}
				return ltext < rtext
			}
			for width := 1; width < s.length; width *= 2 {
				for start := 0; start < s.length; start += 2 * width {
					if s.cancelled() {
						return nil
					}
					middle, end := min(start+width, s.length), min(start+2*width, s.length)
					l, r := start, middle
					for on := start; on < end; on++ {
						if l < middle && (r == end || !less(source[r], source[l])) {
							destination[on] = source[l]
							l++
						} else {
							destination[on] = source[r]
							r++
						}
					}
				}
				source, destination = destination, source
			}
			continue
		}
		values := key.values.AsU64T()
		var vary uint64
		first := values[source[0]*2]
		for _, id := range source {
			vary |= values[id*2] ^ first
		}
		for shift := uint(0); shift < 64; shift += 8 {
			if byte(vary>>shift) == 0 {
				continue
			}
			var counts [256]int
			for i, id := range source {
				if i&65535 == 0 && s.cancelled() {
					return nil
				}
				counts[byte(values[id*2]>>shift)]++
			}
			position := 0
			for i := range counts {
				n := counts[i]
				counts[i] = position
				position += n
			}
			for _, id := range source {
				bucket := byte(values[id*2] >> shift)
				destination[counts[bucket]] = id
				counts[bucket]++
			}
			source, destination = destination, source
		}
		on := 0
		for _, null := range []bool{order.nullsFirst, !order.nullsFirst} {
			for _, id := range source {
				if (values[id*2+1] != 0) == null {
					destination[on] = id
					on++
				}
			}
		}
		source, destination = destination, source
	}
	for i, id := range source {
		source[i] = rows[id]
	}
	return s.gather(a, step.schema, source)
}

func (s *compactSortState) gather(a *mem.Allocator, schema Schema, ids []uint64) *store.Batch {
	vectors := make([]store.Vector, schema.Len())
	defer func() {
		for i := range vectors {
			vectors[i].Release()
		}
	}()
	for column := range vectors {
		if s.cancelled() {
			return nil
		}
		field := schema.FieldAt(column)
		if field.Type().ID() == dtype.STRT {
			data := a.AllocSeg(len(ids) * 16)
			if data == nil {
				return nil
			}
			clear(data.AsBytes())
			var validity *store.BitMap
			if field.Nullable() {
				validity = store.MakeBitMap(a, len(ids))
				if validity == nil {
					data.Dec()
					return nil
				}
				validity.SetAll()
			}
			var size int
			for i, id := range ids {
				src, row := s.batches[id>>32].VectorAt(column), int(uint32(id))
				if src.Validity() != nil && !src.Validity().IsSet(row) {
					validity.Clear(i)
					continue
				}
				n := src.StringAt(row).Len()
				if n > dtype.StringInlineSize {
					if n > int(^uint(0)>>1)-size {
						data.Dec()
						validity.Release()
						return nil
					}
					size += n
				}
			}
			var backing *mem.Segment
			if size != 0 {
				backing = a.AllocSeg(size)
				if backing == nil {
					data.Dec()
					validity.Release()
					return nil
				}
			}
			vectors[column] = store.MakeVectorFromOwnedSegments(field.Type(), len(ids), data, validity, backing)
			if vectors[column].Kind() == store.VectorInvalid {
				return nil
			}
			descriptors, on := vectors[column].Strings(), 0
			for i, id := range ids {
				if validity != nil && !validity.IsSet(i) {
					continue
				}
				src := s.batches[id>>32].VectorAt(column).StringAt(int(uint32(id))).View()
				if len(src) <= dtype.StringInlineSize {
					descriptors[i] = dtype.MakeString(borrowedStringBytes(src))
				} else {
					payload := backing.AsBytes()[on : on+len(src)]
					copy(payload, src)
					descriptors[i] = dtype.MakeString(payload)
					on += len(src)
				}
			}
			continue
		}
		vectors[column] = store.MakeVector(a, len(ids), field.Type(), field.Nullable())
		if vectors[column].Kind() == store.VectorInvalid {
			return nil
		}
		for i, id := range ids {
			src, row := s.batches[id>>32].VectorAt(column), int(uint32(id))
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
			}
		}
	}
	result := store.MakeBatch(vectors)
	if result != nil {
		vectors = nil
	}
	return result
}
