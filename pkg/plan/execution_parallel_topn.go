package plan

import (
	"context"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (p *PhysicalPlan) finishParallelTopN(ctx context.Context, a *mem.Allocator, scope *mem.AllocationScope, workers []*sortState, at int, budget int64, sink func(*store.Batch) bool) ExecutionResult {
	var remaining int64
	for _, worker := range workers {
		remaining = saturatingAdd(remaining, worker.top.charged())
	}
	if remaining > budget {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	merged := workers[0]
	workers[0] = nil
	remaining -= merged.top.charged()
	defer merged.release()
	for i := 1; i < len(workers); i++ {
		worker := workers[i]
		if !merged.top.merge(ctx, a, worker.top, budget-remaining) {
			if err := ctx.Err(); err != nil {
				return ExecutionResult{code: ExecutionCancelled, cause: err}
			}
			return ExecutionResult{code: ExecutionResourceExhausted}
		}
		remaining -= worker.top.charged()
		worker.release()
		workers[i] = nil
	}
	merged.charged = merged.top.charged()
	tail := parallelAggregateTail{ctx: ctx, allocator: a, scope: scope, sink: sink, sorter: merged, step: p.steps[at], limit: p.steps[at+1], hasLimit: true, budget: budget}
	return tail.finish()
}

func (s *compactTopNState) merge(ctx context.Context, a *mem.Allocator, other *compactTopNState, budget int64) bool {
	if s.rows.types == nil {
		s.rows.types = other.rows.types
	}
	columns := len(s.rows.types)
	for i, item := range other.slice()[:other.length] {
		if i&255 == 0 && ctx.Err() != nil {
			return false
		}
		source := other.rows.slots()[int(item.row)*columns : (int(item.row)+1)*columns]
		if s.length == s.limit {
			worst := s.slice()[0]
			if s.compare(item, worst) >= 0 {
				continue
			}
			item.row = worst.row
			copy(s.rows.slots()[int(item.row)*columns:(int(item.row)+1)*columns], source)
			s.slice()[0] = item
			s.siftDown(0)
			continue
		}
		if s.length == s.capacity && !s.grow(a, budget) {
			return false
		}
		if !s.rows.reserve(a, 0, budget-int64(s.capacity)*int64(compactTopNItemBytes)) {
			return false
		}
		item.row = uint64(s.rows.length)
		copy(s.rows.slots()[s.rows.length*columns:(s.rows.length+1)*columns], source)
		s.rows.length++
		s.slice()[s.length] = item
		s.length++
		s.siftUp(s.length - 1)
	}
	return true
}
