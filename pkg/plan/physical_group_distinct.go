package plan

import (
	"context"
	"slices"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func (g *groupState) mergeUnique(a *mem.Allocator, src *groupState, step physicalStep, budget int64, targetRow, sourceRow, aggregateIndex int, ctx context.Context) bool {
	return g.mergeDistinctSet(a, src.uniques[sourceRow][aggregateIndex], step, budget, targetRow, aggregateIndex, ctx)
}

func (g *groupState) mergeDistinctSet(a *mem.Allocator, source *distinctState, step physicalStep, budget int64, targetRow, aggregateIndex int, ctx context.Context) bool {
	target := g.uniques[targetRow][aggregateIndex]
	if source == nil {
		return true
	}
	if source.keyOnly && !target.keyOnly {
		return false
	}
	if target.keyOnly {
		for i, aggregate := range step.aggregates {
			if aggregate.distinct && g.uniqueSources[i] == aggregateIndex && aggregate.kind != AggregateCount && aggregate.kind != AggregateMin && aggregate.kind != AggregateMax {
				return false
			}
		}
	}
	values := make([]Scalar, 1)
	for row := range source.length() {
		if ctx != nil && row&1023 == 0 && ctx.Err() != nil {
			return false
		}
		before, prior := target.length(), target.charged
		_, ok := target.addEncoded(a, source.keys.key(row), &source.rows, row, values, budget-g.charged(len(step.aggregates))+target.charged)
		if !ok {
			return false
		}
		g.stateBytes = saturatingAdd(g.stateBytes, target.charged-prior)
		if before == target.length() {
			continue
		}
		value := MakeI64Scalar(1)
		if target.keyOnly {
			if source.keys.key(row)[0] == 0 {
				value = MakeNullScalar(value.Type())
			}
		} else {
			value = source.rows.at(row, 0)
		}
		for i, aggregate := range step.aggregates {
			if !aggregate.distinct || aggregate.kind == AggregateMin || aggregate.kind == AggregateMax || g.uniqueSources[i] != aggregateIndex {
				continue
			}
			state := &g.values[targetRow][i]
			priorText := 0
			if state.text != nil {
				priorText = state.text.Len()
			}
			if !state.add(a, aggregate.kind, value) {
				return false
			}
			if state.text != nil {
				g.stateBytes += int64(state.text.Len() - priorText)
			} else {
				g.stateBytes -= int64(priorText)
			}
		}
	}
	return g.charged(len(step.aggregates)) <= budget
}

func (g *groupState) mergeOwnedRow(a *mem.Allocator, src *groupState, step physicalStep, budget int64, row int, keyValues []Scalar) bool {
	if g.compact != nil || src.compact != nil || len(step.aggregates) == 0 {
		return g.mergeRow(a, src, step, budget, row, keyValues)
	}
	key := src.keys.keys.key(row)
	_, needsMerge := g.keys.index.lookup(key, &g.keys.keys)
	charged := int64(len(step.aggregates)) * 64
	if !needsMerge {
		for i, state := range src.uniques[row] {
			if state == nil {
				continue
			}
			if state.keyOnly != g.uniqueKeyOnly[i] {
				needsMerge = true
				break
			}
			charged = saturatingAdd(charged, state.charged)
		}
	}
	if needsMerge {
		if !g.mergeRow(a, src, step, budget, row, keyValues) {
			return false
		}
		for i := range src.values[row] {
			src.values[row][i].release()
		}
		for _, state := range src.uniques[row] {
			if state != nil {
				state.release()
			}
		}
		src.values[row], src.uniques[row] = nil, nil
		return true
	}
	_, ok := g.keys.addEncoded(a, key, &src.keys.rows, row, keyValues, budget-g.charged(len(step.aggregates))+g.keys.charged)
	if !ok {
		return false
	}
	for _, state := range src.values[row] {
		if state.text != nil {
			charged = saturatingAdd(charged, int64(state.text.Len()))
		}
	}
	g.values = append(g.values, src.values[row])
	g.uniques = append(g.uniques, src.uniques[row])
	src.values[row], src.uniques[row] = nil, nil
	g.stateBytes = saturatingAdd(g.stateBytes, charged)
	// Source state accounting is discarded after all shard workers finish.
	return g.charged(len(step.aggregates)) <= budget
}

func (g *groupState) configureDistinct(step physicalStep) {
	g.uniqueSources = make([]int, len(step.aggregates))
	for i, aggregate := range step.aggregates {
		g.uniqueSources[i] = i
		if !aggregate.distinct {
			continue
		}
		for j := 0; j < i; j++ {
			other := step.aggregates[j]
			left, right := aggregate.program, other.program
			if other.distinct && left.outputRoots == right.outputRoots && slices.Equal(left.nodes, right.nodes) && slices.Equal(left.roots, right.roots) && slices.Equal(left.materialize, right.materialize) {
				g.uniqueSources[i] = g.uniqueSources[j]
				break
			}
		}
	}
	g.uniqueKeyOnly = make([]bool, len(step.aggregates))
	for i, aggregate := range step.aggregates {
		g.uniqueKeyOnly[i] = aggregate.distinct
	}
	for i, aggregate := range step.aggregates {
		if aggregate.distinct && aggregate.kind != AggregateCount && aggregate.kind != AggregateMin && aggregate.kind != AggregateMax {
			g.uniqueKeyOnly[g.uniqueSources[i]] = false
		}
	}
	for i := range step.aggregates {
		g.uniqueKeyOnly[i] = g.uniqueKeyOnly[g.uniqueSources[i]]
	}
	g.constantKeys = len(step.groupKeys) > 0
	for _, program := range step.groupKeys {
		if len(program.roots) != 1 || program.nodes[program.roots[0]].kind != exprLiteral {
			g.constantKeys = false
			break
		}
	}
}
