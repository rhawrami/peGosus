package plan

import (
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makeCompactGroupState(step physicalStep) *compactGroupState {
	if len(step.groupKeys) != 1 || len(step.aggregates) == 0 {
		return nil
	}
	switch step.schema.FieldAt(0).Type().ID() {
	case dtype.INT32T, dtype.INT64T, dtype.FLOAT32T, dtype.FLOAT64T, dtype.DATET, dtype.TIMESTAMPTZT, dtype.BOOLT:
	default:
		return nil
	}
	for _, aggregate := range step.aggregates {
		if aggregate.distinct {
			return nil
		}
		switch aggregate.kind {
		case AggregateCount, AggregateCountStar:
		case AggregateSum:
			if len(aggregate.program.roots) != 1 || len(aggregate.program.nodes) == 0 {
				return nil
			}
			switch aggregate.program.nodes[aggregate.program.roots[0]].dType.ID() {
			case dtype.INT32T, dtype.INT64T, dtype.FLOAT32T, dtype.FLOAT64T:
			default:
				return nil
			}
		default:
			return nil
		}
	}
	g := &compactGroupState{keyType: step.schema.FieldAt(0).Type(), aggregateCount: len(step.aggregates), nullIndex: -1}
	for _, aggregate := range step.aggregates {
		if aggregate.kind == AggregateSum {
			g.sumTypes = make([]dtype.TID, len(step.aggregates))
			for i, aggregate := range step.aggregates {
				if aggregate.kind == AggregateSum {
					g.sumTypes[i] = aggregate.program.nodes[aggregate.program.roots[0]].dType.ID()
				}
			}
			break
		}
	}
	return g
}

// compactGroupState stores fixed-width group keys and COUNT/SUM accumulators
// in allocator-backed columns, indexed by a single group-ID map.
type compactGroupState struct {
	index          u64Index
	keys           *mem.Segment
	counts         *mem.Segment
	sums           *mem.Segment
	sumTypes       []dtype.TID
	keyType        dtype.Type
	aggregateCount int
	nullIndex      int
	length         int
	capacity       int
	scope          *mem.AllocationScope
}

func (g *compactGroupState) release() {
	g.index.release()
	if g.keys != nil {
		g.keys.Dec()
	}
	if g.counts != nil {
		g.counts.Dec()
	}
	if g.sums != nil {
		g.sums.Dec()
	}
	*g = compactGroupState{}
}

func (g *compactGroupState) charged() int64 {
	width := g.aggregateCount + 1
	if g.sumTypes != nil {
		width += g.aggregateCount
	}
	return int64(g.capacity)*int64(width)*8 + g.index.bytes()
}

func (g *compactGroupState) grow(a *mem.Allocator, budget int64) bool {
	if g.capacity > int(^uint(0)>>1)/2 {
		return false
	}
	capacity := g.capacity * 2
	if capacity == 0 {
		capacity = 16
	}
	width := int64(g.aggregateCount+1) * 8
	if g.sumTypes != nil {
		width += int64(g.aggregateCount) * 8
	}
	if width <= 0 || int64(capacity) > int64(int(^uint(0)>>1))/width || int64(capacity) > (budget-g.index.bytes())/width-int64(g.capacity) {
		return false
	}
	newKeys, ok := allocOperatorSegment(a, g.scope, capacity*8)
	if !ok {
		return false
	}
	newCounts, ok := allocOperatorSegment(a, g.scope, capacity*g.aggregateCount*8)
	if !ok {
		newKeys.Dec()
		return false
	}
	var newSums *mem.Segment
	if g.sumTypes != nil {
		newSums, ok = allocOperatorSegment(a, g.scope, capacity*g.aggregateCount*8)
		if !ok {
			newKeys.Dec()
			newCounts.Dec()
			return false
		}
	}
	if g.keys != nil {
		copy(newKeys.AsBytes(), g.keys.AsBytes())
		g.keys.Dec()
		copy(newCounts.AsBytes(), g.counts.AsBytes())
		g.counts.Dec()
		if g.sums != nil {
			copy(newSums.AsBytes(), g.sums.AsBytes())
			g.sums.Dec()
		}
	}
	g.keys, g.counts, g.sums, g.capacity = newKeys, newCounts, newSums, capacity
	return true
}

func (g *compactGroupState) add(a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64) bool {
	keyValues, ok := executePhysicalExprProgram(a, batch, step.groupKeys[0])
	if !ok {
		return false
	}
	defer keyValues.release()
	key := &keyValues.vectors[step.groupKeys[0].roots[0]]
	if !key.Type().Equal(g.keyType) {
		return false
	}
	values := make([]physicalExprValues, len(step.aggregates))
	defer func() {
		for i := range values {
			values[i].release()
		}
	}()
	for i, aggregate := range step.aggregates {
		if aggregate.kind == AggregateCountStar {
			continue
		}
		values[i], ok = executePhysicalExprProgram(a, batch, aggregate.program)
		if !ok {
			return false
		}
	}
	selection := batch.Selection().MakeBitMapTemp(a)
	if batch.Selection() != nil && selection == nil {
		return false
	}
	if selection != nil {
		defer selection.Release()
	}
	for row := range batch.Len() {
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		isNull := key.Validity() != nil && !key.Validity().IsSet(row)
		index := g.nullIndex
		var lookup, original uint64
		if !isNull {
			value := scalarAt(key, row)
			lookup, original = value.bits, value.bits
			switch g.keyType.ID() {
			case dtype.FLOAT32T:
				if math.IsNaN(float64(value.f32())) {
					lookup = 0x7fc00000
				} else if value.f32() == 0 {
					lookup = 0
				}
			case dtype.FLOAT64T:
				if math.IsNaN(value.f64()) {
					lookup = 0x7ff8000000000000
				} else if value.f64() == 0 {
					lookup = 0
				}
			}
			var found bool
			index, found = g.index.get(lookup)
			if !found {
				index = -1
			}
		}
		if index < 0 {
			if g.length == g.capacity && !g.grow(a, budget) {
				return false
			}
			index = g.length
			if !isNull && !g.index.put(a, lookup, index, budget-g.charged()+g.index.bytes()) {
				return false
			}
			g.length++
			g.keys.AsU64T()[index] = original
			counts := g.counts.AsI64T()[index*g.aggregateCount : (index+1)*g.aggregateCount]
			clear(counts)
			if g.sums != nil {
				clear(g.sums.AsU64T()[index*g.aggregateCount : (index+1)*g.aggregateCount])
			}
			if isNull {
				g.nullIndex = index
			}
		}
		counts := g.counts.AsI64T()[index*g.aggregateCount : (index+1)*g.aggregateCount]
		var sums []uint64
		if g.sums != nil {
			sums = g.sums.AsU64T()[index*g.aggregateCount : (index+1)*g.aggregateCount]
		}
		for i, aggregate := range step.aggregates {
			if aggregate.kind == AggregateCountStar {
				counts[i]++
			} else {
				v := &values[i].vectors[aggregate.program.roots[0]]
				if v.Validity() != nil && !v.Validity().IsSet(row) {
					continue
				}
				if aggregate.kind == AggregateSum {
					switch g.sumTypes[i] {
					case dtype.INT32T:
						sums[i] += uint64(int64(v.I32s()[row]))
					case dtype.INT64T:
						sums[i] += uint64(v.I64s()[row])
					case dtype.FLOAT32T, dtype.FLOAT64T:
						var value float64
						if g.sumTypes[i] == dtype.FLOAT32T {
							value = float64(v.F32s()[row])
						} else {
							value = v.F64s()[row]
						}
						if math.IsNaN(value) {
							continue
						}
						sums[i] = math.Float64bits(math.Float64frombits(sums[i]) + value)
					}
				}
				counts[i]++
			}
		}
	}
	return true
}

func (g *compactGroupState) merge(a *mem.Allocator, src *compactGroupState, budget int64) bool {
	if !g.keyType.Equal(src.keyType) || g.aggregateCount != src.aggregateCount || len(g.sumTypes) != len(src.sumTypes) {
		return false
	}
	for i, typ := range g.sumTypes {
		if typ != src.sumTypes[i] {
			return false
		}
	}
	for row := range src.length {
		key := src.keys.AsU64T()[row]
		index := g.nullIndex
		if row != src.nullIndex {
			lookup := key
			switch g.keyType.ID() {
			case dtype.FLOAT32T:
				value := math.Float32frombits(uint32(key))
				if math.IsNaN(float64(value)) {
					lookup = 0x7fc00000
				} else if value == 0 {
					lookup = 0
				}
			case dtype.FLOAT64T:
				value := math.Float64frombits(key)
				if math.IsNaN(value) {
					lookup = 0x7ff8000000000000
				} else if value == 0 {
					lookup = 0
				}
			}
			var found bool
			index, found = g.index.get(lookup)
			if !found {
				index = -1
			}
			if index < 0 {
				if g.length == g.capacity && !g.grow(a, budget) {
					return false
				}
				index = g.length
				if !g.index.put(a, lookup, index, budget-g.charged()+g.index.bytes()) {
					return false
				}
			}
		}
		if index < 0 {
			if g.length == g.capacity && !g.grow(a, budget) {
				return false
			}
			index = g.length
			g.nullIndex = index
		}
		if index == g.length {
			g.length++
			g.keys.AsU64T()[index] = key
			clear(g.counts.AsI64T()[index*g.aggregateCount : (index+1)*g.aggregateCount])
			if g.sums != nil {
				clear(g.sums.AsU64T()[index*g.aggregateCount : (index+1)*g.aggregateCount])
			}
		}
		for i, count := range src.counts.AsI64T()[row*src.aggregateCount : (row+1)*src.aggregateCount] {
			if g.sums != nil && count != 0 {
				at := index*g.aggregateCount + i
				sum := src.sums.AsU64T()[row*src.aggregateCount+i]
				if g.counts.AsI64T()[at] == 0 {
					g.sums.AsU64T()[at] = sum
				} else if g.sumTypes[i] == dtype.FLOAT32T || g.sumTypes[i] == dtype.FLOAT64T {
					g.sums.AsU64T()[at] = math.Float64bits(math.Float64frombits(g.sums.AsU64T()[at]) + math.Float64frombits(sum))
				} else {
					g.sums.AsU64T()[at] += sum
				}
			}
			g.counts.AsI64T()[index*g.aggregateCount+i] += count
		}
		if g.charged() > budget {
			return false
		}
	}
	return true
}

func (g *compactGroupState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
	vectors := make([]store.Vector, 1+g.aggregateCount)
	vectors[0] = store.MakeVector(a, g.length, g.keyType, step.schema.FieldAt(0).Nullable())
	if vectors[0].Kind() == store.VectorInvalid {
		return nil
	}
	for i := range g.length {
		if i == g.nullIndex {
			vectors[0].Validity().Clear(i)
			continue
		}
		bits := g.keys.AsU64T()[i]
		switch g.keyType.ID() {
		case dtype.INT32T, dtype.DATET:
			vectors[0].I32s()[i] = int32(bits)
		case dtype.INT64T, dtype.TIMESTAMPTZT:
			vectors[0].I64s()[i] = int64(bits)
		case dtype.FLOAT32T:
			vectors[0].F32s()[i] = math.Float32frombits(uint32(bits))
		case dtype.FLOAT64T:
			vectors[0].F64s()[i] = math.Float64frombits(bits)
		case dtype.BOOLT:
			if bits != 0 {
				vectors[0].Bools()[i>>3] |= 1 << (i & 7)
			}
		}
	}
	for j, aggregate := range step.aggregates {
		field := step.schema.FieldAt(j + 1)
		vectors[j+1] = store.MakeVector(a, g.length, field.Type(), field.Nullable())
		if vectors[j+1].Kind() == store.VectorInvalid {
			for i := range vectors {
				vectors[i].Release()
			}
			return nil
		}
		for i := range g.length {
			at := i*g.aggregateCount + j
			if aggregate.kind != AggregateSum {
				vectors[j+1].I64s()[i] = g.counts.AsI64T()[at]
			} else if g.counts.AsI64T()[at] == 0 {
				vectors[j+1].Validity().Clear(i)
			} else if field.Type().ID() == dtype.FLOAT64T {
				vectors[j+1].F64s()[i] = math.Float64frombits(g.sums.AsU64T()[at])
			} else {
				vectors[j+1].I64s()[i] = int64(g.sums.AsU64T()[at])
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
