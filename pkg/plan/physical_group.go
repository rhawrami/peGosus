package plan

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makeGroupState(step physicalStep, scope *mem.AllocationScope) *groupState {
	g := &groupState{compact: makeCompactGroupState(step), scope: scope}
	if g.compact == nil && len(step.groupKeys) == 1 && step.schema.FieldAt(0).Type().ID() == dtype.STRT {
		g.keys.index.stringKeys = true
	}
	g.keys.setScope(scope)
	if g.compact != nil {
		g.compact.scope = scope
		g.compact.index.scope = scope
		g.compact.strings.setScope(scope)
	}
	return g
}

type groupState struct {
	keys       distinctState
	values     [][]aggregateValue
	uniques    [][]*distinctState
	stateBytes int64
	compact    *compactGroupState
	scope      *mem.AllocationScope
}

func (g *groupState) release() {
	if g.compact != nil {
		g.compact.release()
		return
	}
	g.keys.release()
	for _, group := range g.values {
		for i := range group {
			group[i].release()
		}
	}
	for _, group := range g.uniques {
		for _, state := range group {
			if state != nil {
				state.release()
			}
		}
	}
}

func (g *groupState) charged(_ int) int64 {
	if g.compact != nil {
		return g.compact.charged()
	}
	return saturatingAdd(g.keys.charged, g.stateBytes)
}

func (g *groupState) add(a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64) bool {
	if g.compact != nil {
		return g.compact.add(a, batch, step, budget)
	}
	keyValues := make([]physicalExprValues, len(step.groupKeys))
	aggValues := make([]physicalExprValues, len(step.aggregates))
	defer func() {
		for i := range keyValues {
			keyValues[i].release()
		}
		for i := range aggValues {
			aggValues[i].release()
		}
	}()
	keys := make([]store.Vector, len(step.groupKeys))
	for i, program := range step.groupKeys {
		var ok bool
		keyValues[i], ok = executePhysicalExprProgram(a, batch, program)
		if !ok {
			return false
		}
		keys[i] = keyValues[i].vectors[program.roots[0]].Retain()
	}
	keyBatch := store.MakeBatch(keys)
	if keyBatch == nil {
		for i := range keys {
			keys[i].Release()
		}
		return false
	}
	defer keyBatch.Release()
	for i, aggregate := range step.aggregates {
		if aggregate.kind == AggregateCountStar {
			continue
		}
		var ok bool
		aggValues[i], ok = executePhysicalExprProgram(a, batch, aggregate.program)
		if !ok {
			return false
		}
	}
	uniqueBatches := make([]*store.Batch, len(step.aggregates))
	for i, aggregate := range step.aggregates {
		if aggregate.distinct {
			uniqueBatches[i] = store.MakeBatchRetained([]store.Vector{aggValues[i].vectors[aggregate.program.roots[0]]})
		}
	}
	defer func() {
		for _, b := range uniqueBatches {
			if b != nil {
				b.Release()
			}
		}
	}()
	selection := batch.Selection().RetainBitMap(a)
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
		var index int
		var ok bool
		remaining := budget - g.charged(len(step.aggregates)) + g.keys.charged
		if g.keys.index.stringKeys {
			index, ok = g.keys.addString(a, keyBatch, row, remaining)
		} else {
			index, ok = g.keys.add(a, keyBatch, row, remaining)
		}
		if !ok {
			return false
		}
		if len(step.aggregates) == 0 {
			continue
		}
		for len(g.values) <= index {
			g.values = append(g.values, make([]aggregateValue, len(step.aggregates)))
			g.stateBytes = saturatingAdd(g.stateBytes, int64(len(step.aggregates))*64)
			uniques := make([]*distinctState, len(step.aggregates))
			for i, aggregate := range step.aggregates {
				if aggregate.distinct {
					uniques[i] = &distinctState{}
					uniques[i].setScope(g.scope)
				}
			}
			g.uniques = append(g.uniques, uniques)
		}
		states := g.values[index]
		for i, aggregate := range step.aggregates {
			if aggregate.distinct {
				v := uniqueBatches[i].VectorAt(0)
				if v.Validity() != nil && !v.Validity().IsSet(row) {
					continue
				}
				before := g.uniques[index][i].rows.length
				priorCharge := g.uniques[index][i].charged
				_, ok := g.uniques[index][i].add(a, uniqueBatches[i], row, budget-g.charged(len(step.aggregates))+g.uniques[index][i].charged)
				if !ok {
					return false
				}
				g.stateBytes = saturatingAdd(g.stateBytes, g.uniques[index][i].charged-priorCharge)
				if g.uniques[index][i].rows.length == before {
					continue
				}
			}
			before := 0
			if states[i].text != nil {
				before = states[i].text.Len()
			}
			if aggregate.kind == AggregateCountStar {
				if !states[i].add(a, aggregate.kind, Scalar{}) {
					return false
				}
			} else {
				if !states[i].add(a, aggregate.kind, scalarAt(&aggValues[i].vectors[aggregate.program.roots[0]], row)) {
					return false
				}
			}
			after := 0
			if states[i].text != nil {
				after = states[i].text.Len()
			}
			g.stateBytes += int64(after - before)
		}
		if g.charged(len(step.aggregates)) > budget {
			return false
		}
	}
	return true
}

func (g *groupState) merge(a *mem.Allocator, src *groupState, step physicalStep, budget int64) bool {
	if g.compact != nil || src.compact != nil {
		return g.compact != nil && src.compact != nil && g.compact.merge(a, src.compact, budget)
	}
	keyValues := make([]Scalar, len(src.keys.rows.types))
	for row := range src.groupCount() {
		if !g.mergeRow(a, src, step, budget, row, keyValues) {
			return false
		}
	}
	return true
}

func (g *groupState) mergeRow(a *mem.Allocator, src *groupState, step physicalStep, budget int64, row int, keyValues []Scalar) bool {
	if g.compact != nil || src.compact != nil {
		return g.compact != nil && src.compact != nil && g.compact.mergeRow(a, src.compact, budget, row, keyValues)
	}
	key := src.keys.keys.key(row)
	index, ok := g.keys.addEncoded(a, key, &src.keys.rows, row, keyValues, budget-g.charged(len(step.aggregates))+g.keys.charged)
	if !ok {
		return false
	}
	if len(step.aggregates) == 0 {
		return true
	}
	for len(g.values) <= index {
		g.values = append(g.values, make([]aggregateValue, len(step.aggregates)))
		g.uniques = append(g.uniques, make([]*distinctState, len(step.aggregates)))
		g.stateBytes = saturatingAdd(g.stateBytes, int64(len(step.aggregates))*64)
	}
	for i, aggregate := range step.aggregates {
		before := 0
		if text := g.values[index][i].text; text != nil {
			before = text.Len()
		}
		mergeAggregate(&g.values[index][i], &src.values[row][i], aggregate.kind)
		after := 0
		if text := g.values[index][i].text; text != nil {
			after = text.Len()
		}
		g.stateBytes += int64(after - before)
	}
	return g.charged(len(step.aggregates)) <= budget
}

func (g *groupState) groupCount() int {
	if g.compact != nil {
		return g.compact.length
	}
	return g.keys.rows.length
}

func (g *groupState) encodedKey(row int) []byte {
	if g.compact != nil {
		return g.compact.strings.keys.key(row)
	}
	return g.keys.keys.key(row)
}

func (g *groupState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
	if g.compact == nil && len(step.aggregates) == 0 {
		return g.keys.rows.makeBatch(a, step.schema)
	}
	if g.compact != nil {
		return g.compact.finish(a, step)
	}
	rows := make([]distinctRow, len(g.values))
	for i, state := range g.values {
		values := make([]Scalar, step.schema.Len())
		for j := range step.groupKeys {
			values[j] = g.keys.rows.at(i, j)
		}
		for j, aggregate := range step.aggregates {
			field := step.schema.FieldAt(len(step.groupKeys) + j)
			value := state[j].value
			if aggregate.kind == AggregateCountStar || aggregate.kind == AggregateCount {
				value = MakeI64Scalar(state[j].count)
			} else if state[j].count == 0 {
				value = MakeNullScalar(field.Type())
			} else if aggregate.kind == AggregateAvg {
				value = MakeF64Scalar(state[j].value.f64() / float64(state[j].count))
			}
			values[len(step.groupKeys)+j] = value
		}
		rows[i] = distinctRow{values: values}
	}
	return makeDistinctBatch(a, step.schema, rows)
}
