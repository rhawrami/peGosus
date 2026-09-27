package plan

import (
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type groupState struct {
	keys    distinctState
	values  [][]aggregateValue
	uniques [][]*distinctState
}

func (g *groupState) release() {
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

func (g *groupState) charged(aggregates int) int64 {
	total := saturatingAdd(g.keys.charged, int64(len(g.values)*aggregates)*64)
	for _, group := range g.values {
		for _, value := range group {
			if value.text != nil {
				total = saturatingAdd(total, int64(value.text.Len()))
			}
		}
	}
	for _, group := range g.uniques {
		for _, state := range group {
			if state != nil {
				total = saturatingAdd(total, state.charged)
			}
		}
	}
	return total
}

func (g *groupState) add(a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64) bool {
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
			uniques := make([]*distinctState, len(step.aggregates))
			for i, aggregate := range step.aggregates {
				if aggregate.distinct {
					uniques[i] = &distinctState{}
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
				before := len(g.uniques[index][i].rows)
				_, ok := g.uniques[index][i].add(a, uniqueBatches[i], row, budget-g.charged(len(step.aggregates))+g.uniques[index][i].charged)
				if !ok {
					return false
				}
				if len(g.uniques[index][i].rows) == before {
					continue
				}
			}
			if aggregate.kind == AggregateCountStar {
				states[i].add(a, aggregate.kind, Scalar{})
			} else {
				states[i].add(a, aggregate.kind, scalarAt(&aggValues[i].vectors[aggregate.program.roots[0]], row))
			}
		}
		if g.charged(len(step.aggregates)) > budget {
			return false
		}
	}
	for index, states := range local {
		for i, aggregate := range step.aggregates {
			mergeAggregate(&g.values[index][i], &states[i], aggregate.kind)
		}
	}
	return true
}

func (g *groupState) finish(a *mem.Allocator, step physicalStep) *store.Batch {
	rows := make([]distinctRow, len(g.values))
	for i, state := range g.values {
		values := make([]Scalar, step.schema.Len())
		copy(values, g.keys.rows[i].values)
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
