package plan

import (
	"context"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (p *PhysicalPlan) executeParallelDistinct(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool) (ExecutionResult, bool) {
	return p.executeParallelDistinctWithScope(ctx, a, options, sink, nil)
}

func (p *PhysicalPlan) executeParallelDistinctWithScope(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool, scope *mem.AllocationScope) (ExecutionResult, bool) {
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
			if len(step.groupKeys) != 0 {
				return p.executeParallelWithScope(ctx, a, options, sink, scope)
			}
			if i+1 != len(p.steps) {
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
				hasDistinct := false
				for _, aggregate := range step.aggregates {
					hasDistinct = hasDistinct || aggregate.distinct
				}
				if !hasDistinct {
					return ExecutionResult{}, false
				}
				if len(step.aggregates) == 1 && (step.aggregates[0].kind == AggregateMin || step.aggregates[0].kind == AggregateMax) {
					local := *p
					local.steps = append([]physicalStep(nil), p.steps...)
					local.steps[i].aggregates = append([]physicalAggregateExpr(nil), step.aggregates...)
					local.steps[i].aggregates[0].distinct = false
					return local.executeParallelWithScope(ctx, a, options, sink, scope)
				}
				if len(step.aggregates) != 1 {
					return p.executeGlobalDistinctGrouped(ctx, a, options, sink, i, scope, true)
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
		return local.executeParallelWithScope(ctx, a, options, sink, scope)
	}
	local.steps = local.steps[:at+1]
	// Reduce globally unique rows while partition output is borrowed from its producer.
	if scope == nil {
		scope = mem.MakeAllocationScope(a, options.MemoryBudget)
	}
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

func (p *PhysicalPlan) executeGlobalDistinctWithScope(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool, scope *mem.AllocationScope) (ExecutionResult, bool) {
	for i, step := range p.steps {
		if step.operation != physicalAggregate || len(step.groupKeys) != 0 || len(step.aggregates) < 2 || i+1 != len(p.steps) {
			continue
		}
		hasDistinct := false
		for _, aggregate := range step.aggregates {
			hasDistinct = hasDistinct || aggregate.distinct
		}
		if hasDistinct {
			return p.executeGlobalDistinctGrouped(ctx, a, options, sink, i, scope, false)
		}
	}
	return ExecutionResult{}, false
}

func (p *PhysicalPlan) executeGlobalDistinctGrouped(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool, at int, scope *mem.AllocationScope, parallelOnly bool) (ExecutionResult, bool) {
	local := *p
	local.steps = append([]physicalStep(nil), p.steps...)
	step := p.steps[at]
	step.schema = Schema{valid: true, fields: append([]Field{{name: "_distinct_group", dType: dtype.Int64T()}}, step.schema.fields...)}
	step.groupKeys = []physicalExprProgram{{nodes: []physicalExprNode{{kind: exprLiteral, dType: dtype.Int64T(), literal: MakeI64Scalar(1)}}, roots: []int{0}, materialize: []bool{false}}}
	step.aggregateSources = nil
	local.steps[at] = step
	emitted := false
	consume := func(batch *store.Batch) bool {
		emitted = true
		vectors := make([]store.Vector, batch.NVectors()-1)
		for i := range vectors {
			vectors[i] = batch.VectorAt(i + 1).Retain()
		}
		output := store.MakeBatch(vectors)
		if output == nil {
			for i := range vectors {
				vectors[i].Release()
			}
			return false
		}
		defer output.Release()
		return sink(output)
	}
	if scope == nil {
		scope = mem.MakeAllocationScope(a, options.MemoryBudget)
	}
	var result ExecutionResult
	parallel := true
	if parallelOnly {
		result, parallel = local.executeParallelWithScope(ctx, a, options, consume, scope)
	} else {
		result = local.executeWithScope(ctx, a, options, consume, scope)
	}
	if !parallel || emitted || result.code != ExecutionCompleted {
		return result, parallel
	}
	scoped := mem.MakeAllocatorWithScope(a, scope)
	output := finalizeAggregates(scoped, p.steps[at], make([]aggregateValue, len(step.aggregates)))
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
	return result, true
}
