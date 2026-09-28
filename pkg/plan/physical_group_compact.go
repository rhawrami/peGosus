package plan

import (
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makeCompactGroupCountState(step physicalStep) *compactGroupCountState {
	if len(step.groupKeys) != 1 || len(step.aggregates) == 0 {
		return nil
	}
	switch step.schema.FieldAt(0).Type().ID() {
	case dtype.INT32T, dtype.INT64T, dtype.FLOAT32T, dtype.FLOAT64T, dtype.DATET, dtype.TIMESTAMPTZT, dtype.BOOLT:
	default:
		return nil
	}
	for _, aggregate := range step.aggregates {
		if aggregate.distinct || aggregate.kind != AggregateCount && aggregate.kind != AggregateCountStar {
			return nil
		}
	}
	return &compactGroupCountState{keyType: step.schema.FieldAt(0).Type(), aggregateCount: len(step.aggregates), nullIndex: -1}
}

// compactGroupCountState stores fixed-width group keys and COUNT accumulators
// in allocator-backed columns, indexed by a single group-ID map.
type compactGroupCountState struct {
	index          u64Index
	keys           *mem.Segment
	counts         *mem.Segment
	keyType        dtype.Type
	aggregateCount int
	nullIndex      int
	length         int
	capacity       int
	scope          *mem.AllocationScope
}

func (g *compactGroupCountState) release() {
	g.index.release()
	if g.keys != nil {
		g.keys.Dec()
	}
	if g.counts != nil {
		g.counts.Dec()
	}
	*g = compactGroupCountState{}
}

func (g *compactGroupCountState) charged() int64 {
	return int64(g.capacity)*int64(g.aggregateCount+1)*8 + g.index.bytes()
}

func (g *compactGroupCountState) grow(a *mem.Allocator, budget int64) bool {
	if g.capacity > int(^uint(0)>>1)/2 {
		return false
	}
	capacity := g.capacity * 2
	if capacity == 0 {
		capacity = 16
	}
	width := int64(g.aggregateCount+1) * 8
	if width <= 0 || int64(capacity) > int64(int(^uint(0)>>1))/width || int64(capacity) > (budget-g.index.bytes())/width-int64(g.capacity) {
		return false
	}
	var newKeys, newCounts *mem.Segment
	if g.scope != nil {
		var ok bool
		newKeys, ok = g.scope.AllocSeg(capacity * 8)
		if !ok {
			return false
		}
		newCounts, ok = g.scope.AllocSeg(capacity * g.aggregateCount * 8)
		if !ok {
			newKeys.Dec()
			return false
		}
	} else {
		newKeys = a.AllocSeg(capacity * 8)
		newCounts = a.AllocSeg(capacity * g.aggregateCount * 8)
	}
	if g.keys != nil {
		copy(newKeys.AsBytes(), g.keys.AsBytes())
		g.keys.Dec()
		copy(newCounts.AsBytes(), g.counts.AsBytes())
		g.counts.Dec()
	}
	g.keys, g.counts, g.capacity = newKeys, newCounts, capacity
	return true
}

func (g *compactGroupCountState) add(a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64) bool {
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
		if aggregate.kind != AggregateCount {
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
			if !isNull && !g.index.put(a, lookup, index, budget-int64(g.capacity)*int64(g.aggregateCount+1)*8) {
				return false
			}
			g.length++
			g.keys.AsU64T()[index] = original
			counts := g.counts.AsI64T()[index*g.aggregateCount : (index+1)*g.aggregateCount]
			clear(counts)
			if isNull {
				g.nullIndex = index
			}
		}
		counts := g.counts.AsI64T()[index*g.aggregateCount : (index+1)*g.aggregateCount]
		for i, aggregate := range step.aggregates {
			if aggregate.kind == AggregateCountStar {
				counts[i]++
			} else {
				v := &values[i].vectors[aggregate.program.roots[0]]
				if v.Validity() == nil || v.Validity().IsSet(row) {
					counts[i]++
				}
			}
		}
	}
	return true
}

func (g *compactGroupCountState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
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
	for j := range step.aggregates {
		vectors[j+1] = store.MakeVector(a, g.length, dtype.Int64T(), false)
		if vectors[j+1].Kind() == store.VectorInvalid {
			for i := range vectors {
				vectors[i].Release()
			}
			return nil
		}
		for i := range g.length {
			vectors[j+1].I64s()[i] = g.counts.AsI64T()[i*g.aggregateCount+j]
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
