package plan

import (
	"context"
	"runtime"
	"sync"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (p *PhysicalPlan) executeGlobalDistinctMerge(ctx context.Context, a *mem.Allocator, scope *mem.AllocationScope, step physicalStep, sources []*groupState, budget int64, sink func(*store.Batch) bool) (ExecutionResult, bool) {
	if len(step.groupKeys) != 1 || len(step.groupKeys[0].roots) != 1 || step.groupKeys[0].nodes[step.groupKeys[0].roots[0]].kind != exprLiteral {
		return ExecutionResult{}, false
	}
	hasDistinct := false
	for _, aggregate := range step.aggregates {
		hasDistinct = hasDistinct || aggregate.distinct
	}
	if !hasDistinct {
		return ExecutionResult{}, false
	}
	populated := make([]*groupState, 0, len(sources))
	for _, source := range sources {
		if source == nil || source.groupCount() == 0 {
			continue
		}
		if source.groupCount() != 1 || source.compact != nil {
			return ExecutionResult{}, false
		}
		populated = append(populated, source)
	}
	if len(populated) == 0 {
		return ExecutionResult{}, false
	}
	merged := makeGroupState(step, scope)
	defer merged.release()
	merged.keys, populated[0].keys = populated[0].keys, distinctState{}
	merged.values = [][]aggregateValue{make([]aggregateValue, len(step.aggregates))}
	merged.uniques = [][]*distinctState{make([]*distinctState, len(step.aggregates))}
	merged.stateBytes = int64(len(step.aggregates)) * 64
	states := make([]*groupState, len(step.aggregates))
	lanes := make([][]*distinctState, len(step.aggregates))
	defer func() {
		for i, state := range states {
			if state != nil {
				state.release()
			}
			for _, source := range lanes[i] {
				if source != nil {
					source.release()
				}
			}
		}
	}()
	// MIN/MAX must inspect every worker, including representatives omitted by deduplication.
	for _, source := range populated {
		for i, aggregate := range step.aggregates {
			if aggregate.distinct && aggregate.kind != AggregateMin && aggregate.kind != AggregateMax {
				continue
			}
			prior := 0
			if merged.values[0][i].text != nil {
				prior = merged.values[0][i].text.Len()
			}
			mergeAggregate(&merged.values[0][i], &source.values[0][i], aggregate.kind)
			after := 0
			if merged.values[0][i].text != nil {
				after = merged.values[0][i].text.Len()
			}
			merged.stateBytes += int64(after - prior)
		}
	}
	for i, aggregate := range step.aggregates {
		if !aggregate.distinct || merged.uniqueSources[i] != i {
			continue
		}
		ordered := false
		for j, other := range step.aggregates {
			if !other.distinct || merged.uniqueSources[j] != i {
				continue
			}
			id := other.program.nodes[other.program.roots[0]].dType.ID()
			ordered = ordered || other.kind == AggregateAvg || other.kind == AggregateSum && (id == dtype.FLOAT32T || id == dtype.FLOAT64T)
		}
		seed := 0
		if !ordered {
			for j := 1; j < len(populated); j++ {
				if populated[j].uniques[0][i].length() > populated[seed].uniques[0][i].length() {
					seed = j
				}
			}
		}
		state := &groupState{scope: scope, uniqueSources: merged.uniqueSources, values: [][]aggregateValue{make([]aggregateValue, len(step.aggregates))}, uniques: [][]*distinctState{make([]*distinctState, len(step.aggregates))}}
		states[i] = state
		state.uniques[0][i] = populated[seed].uniques[0][i]
		state.stateBytes = state.uniques[0][i].charged
		lanes[i] = make([]*distinctState, len(populated))
		for j, source := range populated {
			if j != seed {
				lanes[i][j] = source.uniques[0][i]
			}
			source.uniques[0][i] = nil
		}
		for j, other := range step.aggregates {
			if other.distinct && other.kind != AggregateMin && other.kind != AggregateMax && merged.uniqueSources[j] == i {
				state.values[0][j], populated[seed].values[0][j] = populated[seed].values[0][j], aggregateValue{}
			}
		}
	}
	// Detached lanes own their sets; no merge worker mutates source group accounting.
	for i, source := range sources {
		if source != nil {
			source.release()
			sources[i] = nil
		}
	}
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, max(1, min(len(sources), runtime.GOMAXPROCS(0))))
	failed := make([]bool, len(step.aggregates))
	for i, state := range states {
		if state == nil {
			continue
		}
		semaphore <- struct{}{}
		wg.Add(1)
		go func(i int, state *groupState) {
			defer wg.Done()
			defer func() { <-semaphore }()
			for j, source := range lanes[i] {
				if ctx.Err() != nil {
					return
				}
				if source == nil {
					continue
				}
				source.index.release()
				if source.scratch != nil {
					source.scratch.Dec()
					source.scratch = nil
				}
				source.charged = source.keys.bytes() + source.rows.bytes()
				ok := state.mergeDistinctSet(a, source, step, budget, 0, i, ctx)
				source.release()
				lanes[i][j] = nil
				if !ok {
					failed[i] = true
					return
				}
			}
		}(i, state)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}, true
	}
	for _, fail := range failed {
		if fail {
			return ExecutionResult{code: ExecutionResourceExhausted}, true
		}
	}
	for i, state := range states {
		if state == nil {
			continue
		}
		for j, aggregate := range step.aggregates {
			if aggregate.distinct && aggregate.kind != AggregateMin && aggregate.kind != AggregateMax && merged.uniqueSources[j] == i {
				merged.values[0][j], state.values[0][j] = state.values[0][j], aggregateValue{}
			}
		}
		// Final aggregate scalars no longer borrow the distinct payload.
		state.uniques[0][i].release()
		state.uniques[0][i] = nil
	}
	output := merged.finish(a, step)
	if output == nil {
		return ExecutionResult{code: ExecutionResourceExhausted}, true
	}
	defer output.Release()
	if !sink(output) {
		return ExecutionResult{code: ExecutionStopped}, true
	}
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}, true
	}
	return ExecutionResult{code: ExecutionCompleted}, true
}
