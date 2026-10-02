package plan

import (
	"math"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

const compactDenseLimit = 65536

func (g *compactGroupState) auxiliaryBytes() int64 {
	bytes := g.workBytes
	if g.dense != nil {
		bytes += int64(g.dense.Len())
	}
	return bytes
}

func (g *compactGroupState) groupIndex(a *mem.Allocator, lookup, original uint64, isNull bool, budget int64) int {
	index := g.nullIndex
	var dense []uint32
	var offset uint64
	if !isNull {
		if g.dense != nil {
			value := int64(original)
			if g.keyType.ID() == dtype.INT32T || g.keyType.ID() == dtype.DATET {
				value = int64(int32(original))
			}
			dense = g.dense.AsU32T()
			offset = uint64(value) - uint64(g.denseBase)
			if offset >= uint64(len(dense)) {
				if !g.disableDense(a, budget) {
					return -1
				}
				dense = nil
			}
		}
		if dense != nil {
			index = int(dense[offset]) - 1
		} else {
			var found bool
			index, found = g.index.get(lookup)
			if !found {
				index = -1
			}
		}
	}
	if index >= 0 {
		return index
	}
	if g.length == g.capacity && !g.grow(a, budget) {
		return -1
	}
	index = g.length
	if !isNull {
		if dense != nil {
			dense[offset] = uint32(index) + 1
		} else if !g.index.put(a, lookup, index, budget-g.charged()+g.index.bytes()) {
			return -1
		}
	} else {
		g.nullIndex = index
	}
	g.length++
	g.keys.AsU64T()[index] = original
	clear(g.counts.AsI64T()[index*g.aggregateCount : (index+1)*g.aggregateCount])
	if g.sums != nil {
		clear(g.sums.AsU64T()[index*g.aggregateCount : (index+1)*g.aggregateCount])
	}
	return index
}

func (g *compactGroupState) disableDense(a *mem.Allocator, budget int64) bool {
	g.dense.Dec()
	g.dense = nil
	for row, key := range g.keys.AsU64T()[:g.length] {
		if row != g.nullIndex && !g.index.put(a, key, row, budget-g.charged()+g.index.bytes()) {
			return false
		}
	}
	return true
}

func prepareCompactDense[T ~int32 | ~int64](g *compactGroupState, a *mem.Allocator, keys []T, validity, selection *store.BitMap, budget int64) {
	if g.length != 0 || g.dense != nil || g.index.slots != nil {
		return
	}
	low, high, count := int64(math.MaxInt64), int64(math.MinInt64), 0
	for row, key := range keys {
		if selection != nil && !selection.IsSet(row) || validity != nil && !validity.IsSet(row) {
			continue
		}
		value := int64(key)
		low, high = min(low, value), max(high, value)
		count++
	}
	span := uint64(high) - uint64(low)
	if count < 64 || span >= compactDenseLimit || span > uint64(count)*8 {
		return
	}
	capacity := 16
	for uint64(capacity) <= span {
		capacity *= 2
	}
	bytes := int64(capacity) * 4
	// Leave enough room for the first group buffers; declining is only a hint.
	reserve := bytes + int64(16*(g.aggregateCount*2+1))*8
	if reserve > budget-g.charged() || g.scope != nil && reserve > g.scope.Limit()-g.scope.Live() {
		return
	}
	var segment *mem.Segment
	if g.scope != nil {
		var ok bool
		segment, ok = g.scope.TryAllocSeg(int(bytes))
		if !ok {
			return
		}
	} else {
		segment = a.AllocSeg(int(bytes))
	}
	clear(segment.AsU32T())
	padding := int64((uint64(capacity) - span - 1) / 2)
	g.denseBase = math.MinInt64
	if low >= math.MinInt64+padding {
		g.denseBase = low - padding
	}
	g.dense = segment
}

func compactIntegerGroupIDs[T ~int32 | ~int64](g *compactGroupState, a *mem.Allocator, keys []T, validity, selection *store.BitMap, ids []int32, budget int64) bool {
	prepareCompactDense(g, a, keys, validity, selection, budget)
	var dense []uint32
	if g.dense != nil {
		dense = g.dense.AsU32T()
	}
	for row, key := range keys {
		ids[row] = -1
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		isNull := validity != nil && !validity.IsSet(row)
		bits := uint64(key)
		if unsafe.Sizeof(key) == 4 {
			bits = uint64(uint32(key))
		}
		if !isNull && dense != nil {
			offset := uint64(int64(key)) - uint64(g.denseBase)
			if offset < uint64(len(dense)) && dense[offset] != 0 {
				ids[row] = int32(dense[offset] - 1)
				continue
			}
		}
		index := g.groupIndex(a, bits, bits, isNull, budget)
		if index < 0 {
			return false
		}
		ids[row] = int32(index)
		if g.dense == nil {
			dense = nil
		}
	}
	return true
}

func (g *compactGroupState) addTyped(a *mem.Allocator, batch *store.Batch, step physicalStep, key *store.Vector, values []physicalExprValues, selection *store.BitMap, budget int64) bool {
	bytes := int64(batch.Len()) * 4
	if dictionary := key.Dictionary(); dictionary != nil && dictionary.Len() <= compactDenseLimit && int64(dictionary.Len()) <= max(64, int64(batch.Len())*4) {
		bytes += int64(dictionary.Len()) * 4
	}
	if bytes > budget-g.charged() {
		return false
	}
	segment, ok := allocOperatorSegment(a, g.scope, int(bytes))
	if !ok {
		return false
	}
	g.workBytes = bytes
	defer func() { segment.Dec(); g.workBytes = 0 }()
	ids := segment.AsI32T()[:batch.Len()]
	switch g.keyType.ID() {
	case dtype.STRT:
		ok = g.stringGroupIDs(a, key, selection, ids, segment.AsI32T()[batch.Len():], budget)
	case dtype.INT32T, dtype.DATET:
		ok = compactIntegerGroupIDs(g, a, key.I32s(), key.Validity(), selection, ids, budget)
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		ok = compactIntegerGroupIDs(g, a, key.I64s(), key.Validity(), selection, ids, budget)
	default:
		for row := range ids {
			ids[row] = -1
			if selection != nil && !selection.IsSet(row) {
				continue
			}
			isNull := key.Validity() != nil && !key.Validity().IsSet(row)
			var lookup, original uint64
			if !isNull {
				original = scalarAt(key, row).bits
				lookup = original
				if g.keyType.ID() == dtype.FLOAT32T {
					value := math.Float32frombits(uint32(original))
					if math.IsNaN(float64(value)) {
						lookup = 0x7fc00000
					} else if value == 0 {
						lookup = 0
					}
				} else if g.keyType.ID() == dtype.FLOAT64T {
					value := math.Float64frombits(original)
					if math.IsNaN(value) {
						lookup = 0x7ff8000000000000
					} else if value == 0 {
						lookup = 0
					}
				}
			}
			index := g.groupIndex(a, lookup, original, isNull, budget)
			if index < 0 {
				return false
			}
			ids[row] = int32(index)
		}
		ok = true
	}
	if !ok {
		return false
	}
	if g.length == 0 {
		return true
	}
	counts := g.counts.AsI64T()
	var sums []uint64
	if g.sums != nil {
		sums = g.sums.AsU64T()
	}
	for i, aggregate := range step.aggregates {
		if aggregate.kind == AggregateCountStar {
			for _, id := range ids {
				if id >= 0 {
					counts[int(id)*g.aggregateCount+i]++
				}
			}
			continue
		}
		value := &values[i].vectors[aggregate.program.roots[0]]
		if aggregate.kind == AggregateCount {
			validity := value.Validity()
			for row, id := range ids {
				if id >= 0 && (validity == nil || validity.IsSet(row)) {
					counts[int(id)*g.aggregateCount+i]++
				}
			}
			continue
		}
		switch g.sumTypes[i] {
		case dtype.INT32T:
			compactIntegerSums(value.I32s(), value.Validity(), ids, counts, sums, g.aggregateCount, i)
		case dtype.INT64T:
			compactIntegerSums(value.I64s(), value.Validity(), ids, counts, sums, g.aggregateCount, i)
		case dtype.FLOAT32T:
			compactFloatSums(value.F32s(), value.Validity(), ids, counts, sums, g.aggregateCount, i)
		case dtype.FLOAT64T:
			compactFloatSums(value.F64s(), value.Validity(), ids, counts, sums, g.aggregateCount, i)
		}
	}
	return true
}

func compactIntegerSums[T ~int32 | ~int64](values []T, validity *store.BitMap, ids []int32, counts []int64, sums []uint64, stride, column int) {
	for row, id := range ids {
		if id < 0 || validity != nil && !validity.IsSet(row) {
			continue
		}
		at := int(id)*stride + column
		sums[at] += uint64(int64(values[row]))
		counts[at]++
	}
}

func compactFloatSums[T ~float32 | ~float64](values []T, validity *store.BitMap, ids []int32, counts []int64, sums []uint64, stride, column int) {
	for row, id := range ids {
		if id < 0 || validity != nil && !validity.IsSet(row) {
			continue
		}
		value := float64(values[row])
		if math.IsNaN(value) {
			continue
		}
		at := int(id)*stride + column
		sums[at] = math.Float64bits(math.Float64frombits(sums[at]) + value)
		counts[at]++
	}
}
