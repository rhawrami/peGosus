package plan

import (
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type groupState struct {
	keys       distinctState
	values     [][]aggregateValue
	uniques    [][]*distinctState
	stateBytes int64
	compact    *compactGroupCountState
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
	selection := batch.Selection().MakeBitMapTemp(a)
	if batch.Selection() != nil && selection == nil {
		return false
	}
	if selection != nil {
		defer selection.Release()
	}
	local := make(map[int][]aggregateValue)
	defer func() {
		for _, values := range local {
			for i := range values {
				values[i].release()
			}
		}
	}()
	for row := range batch.Len() {
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		index, ok := g.keys.add(a, keyBatch, row, budget-g.charged(len(step.aggregates))+g.keys.charged)
		if !ok {
			return false
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
		states, exists := local[index]
		if !exists {
			states = make([]aggregateValue, len(step.aggregates))
			local[index] = states
		}
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
			if aggregate.kind == AggregateCountStar {
				if !states[i].add(a, aggregate.kind, Scalar{}) {
					return false
				}
			} else {
				if !states[i].add(a, aggregate.kind, scalarAt(&aggValues[i].vectors[aggregate.program.roots[0]], row)) {
					return false
				}
			}
		}
		if g.charged(len(step.aggregates)) > budget {
			return false
		}
	}
	for index, states := range local {
		for i, aggregate := range step.aggregates {
			priorLength := 0
			if g.values[index][i].text != nil {
				priorLength = g.values[index][i].text.Len()
			}
			mergeAggregate(&g.values[index][i], &states[i], aggregate.kind)
			if g.values[index][i].text != nil {
				g.stateBytes += int64(g.values[index][i].text.Len() - priorLength)
			} else {
				g.stateBytes -= int64(priorLength)
			}
		}
	}
	return true
}

func (g *groupState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
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
