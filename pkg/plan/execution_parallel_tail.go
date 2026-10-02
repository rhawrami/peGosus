package plan

import (
	"context"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makeParallelAggregateTail(ctx context.Context, a *mem.Allocator, scope *mem.AllocationScope, steps []physicalStep, aggregateAt int, budget int64, sink func(*store.Batch) bool) *parallelAggregateTail {
	tail := &parallelAggregateTail{ctx: ctx, allocator: a, scope: scope, budget: budget, sink: sink}
	if aggregateAt+1 >= len(steps) {
		return tail
	}
	tail.step = steps[aggregateAt+1]
	tail.sorter = &sortState{compact: makeCompactSortState(tail.step), top: makeCompactTopNState(tail.step)}
	if tail.sorter.compact != nil {
		tail.sorter.compact.scope, tail.sorter.compact.ctx = scope, ctx
	}
	if tail.sorter.top != nil {
		tail.sorter.top.scope, tail.sorter.top.rows.scope = scope, scope
	}
	if aggregateAt+2 < len(steps) {
		tail.limit = steps[aggregateAt+2]
		tail.hasLimit = true
	}
	return tail
}

type parallelAggregateTail struct {
	ctx       context.Context
	allocator *mem.Allocator
	scope     *mem.AllocationScope
	sink      func(*store.Batch) bool
	sorter    *sortState
	step      physicalStep
	limit     physicalStep
	budget    int64
	hasLimit  bool
	failed    bool
}

func (t *parallelAggregateTail) release() {
	if t.sorter != nil {
		t.sorter.release()
	}
}

func (t *parallelAggregateTail) consume(batch *store.Batch) bool {
	if t.sorter == nil {
		return t.sink(batch)
	}
	if t.ctx.Err() != nil {
		return false
	}
	if !t.sorter.add(t.allocator, batch, t.step, t.budget) {
		t.failed = true
		return false
	}
	return true
}

func (t *parallelAggregateTail) stopped() ExecutionResult {
	if err := t.ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}
	}
	if t.failed {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	return ExecutionResult{code: ExecutionStopped}
}

func (t *parallelAggregateTail) finish() ExecutionResult {
	if t.sorter == nil {
		return ExecutionResult{code: ExecutionCompleted}
	}
	if err := t.ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}
	}
	if t.hasLimit && t.limit.limit == 0 {
		return ExecutionResult{code: ExecutionCompleted}
	}
	if t.sorter.packed != nil && t.sorter.packed.length == 0 || t.sorter.compact != nil && t.sorter.compact.length == 0 || t.sorter.top != nil && t.sorter.top.length == 0 || t.sorter.packed == nil && t.sorter.compact == nil && t.sorter.top == nil && len(t.sorter.rows) == 0 {
		return ExecutionResult{code: ExecutionCompleted}
	}
	batch := t.sorter.finish(t.allocator, t.step)
	if batch == nil {
		if err := t.ctx.Err(); err != nil {
			return ExecutionResult{code: ExecutionCancelled, cause: err}
		}
		if t.scope.Exhausted() {
			return ExecutionResult{code: ExecutionResourceExhausted}
		}
		return ExecutionResult{code: ExecutionFailed}
	}
	defer batch.Release()
	if t.hasLimit {
		if !executePhysicalLimit(t.allocator, batch, &t.limit.limit, &t.limit.offset) {
			if t.scope.Exhausted() {
				return ExecutionResult{code: ExecutionResourceExhausted}
			}
			return ExecutionResult{code: ExecutionFailed}
		}
	}
	if err := t.ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}
	}
	if !t.sink(batch) {
		return t.stopped()
	}
	if err := t.ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}
	}
	return ExecutionResult{code: ExecutionCompleted}
}
