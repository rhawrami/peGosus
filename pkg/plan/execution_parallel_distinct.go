package plan

import (
	"context"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (p *PhysicalPlan) executeParallelDistinct(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool) (ExecutionResult, bool) {
	at, reduceAt := -1, -1
	global := false
	for i, step := range p.steps {
		switch step.operation {
		case physicalDistinct:
			if at >= 0 || step.schema.Len() == 0 {
				return ExecutionResult{}, false
			}
			at = i
		case physicalAggregate:
			if len(step.groupKeys) != 0 || i+1 != len(p.steps) {
				return ExecutionResult{}, false
			}
			if at >= 0 {
				if i != at+1 {
					return ExecutionResult{}, false
				}
				for _, aggregate := range step.aggregates {
					if aggregate.distinct {
						return ExecutionResult{}, false
					}
				}
				reduceAt = i
			} else {
				if len(step.aggregates) != 1 || !step.aggregates[0].distinct {
					return ExecutionResult{}, false
				}
				at, reduceAt, global = i, i, true
			}
		}
	}
	if at < 0 {
		return ExecutionResult{}, false
	}
	local := *p
	local.steps = append([]physicalStep(nil), p.steps...)
	step := p.steps[at]
	group := physicalStep{operation: physicalAggregate, schema: step.schema}
	var reduction physicalStep
	if reduceAt >= 0 {
		reduction = p.steps[reduceAt]
	}
	if global {
		program := step.aggregates[0].program
		if len(program.roots) != 1 {
			return ExecutionResult{}, false
		}
		root := program.nodes[program.roots[0]]
		field := step.schema.FieldAt(0)
		field.dType, field.nullable = root.dType, root.nullable
		group.schema = Schema{valid: true, fields: []Field{field}}
		group.groupKeys = []physicalExprProgram{program}
		aggregate := step.aggregates[0]
		aggregate.distinct = false
		aggregate.program = physicalExprProgram{nodes: []physicalExprNode{{kind: exprColumn, column: 0, dType: root.dType, nullable: root.nullable}}, roots: []int{0}, materialize: []bool{false}}
		reduction.aggregates = []physicalAggregateExpr{aggregate}
	} else {
		group.groupKeys = make([]physicalExprProgram, step.schema.Len())
		for column := range group.groupKeys {
			field := step.schema.FieldAt(column)
			group.groupKeys[column] = physicalExprProgram{nodes: []physicalExprNode{{kind: exprColumn, column: column, dType: field.Type(), nullable: field.Nullable()}}, roots: []int{0}, materialize: []bool{false}}
		}
	}
	local.steps[at] = group
	if reduceAt < 0 {
		return local.executeParallel(ctx, a, options, sink)
	}
	local.steps = local.steps[:at+1]
	// Reduce globally unique rows while partition output is borrowed from its producer.
	scope := mem.MakeAllocationScope(a, options.MemoryBudget)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	states := make([]aggregateValue, len(reduction.aggregates))
	defer func() {
		for i := range states {
			states[i].release()
		}
	}()
	failed := false
	result, parallel := local.executeParallelWithScope(ctx, scoped, options, func(batch *store.Batch) bool {
		if ctx.Err() != nil {
			return false
		}
		if !accumulateAggregates(scoped, batch, reduction, states, nil, options.MemoryBudget) {
			failed = true
			return false
		}
		return ctx.Err() == nil
	}, scope)
	if !parallel {
		return result, false
	}
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}, true
	}
	if failed {
		return ExecutionResult{code: ExecutionResourceExhausted}, true
	}
	if result.code != ExecutionCompleted {
		return result, true
	}
	batch := finalizeAggregates(scoped, reduction, states)
	if batch == nil {
		return ExecutionResult{code: ExecutionResourceExhausted}, true
	}
	defer batch.Release()
	if !sink(batch) {
		return ExecutionResult{code: ExecutionStopped}, true
	}
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}, true
	}
	return ExecutionResult{code: ExecutionCompleted}, true
}
