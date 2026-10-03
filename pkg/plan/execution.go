package plan

import (
	"context"
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

// ExecutionCode identifies the outcome of a physical pipeline.
type ExecutionCode uint8

const (
	ExecutionCompleted ExecutionCode = iota
	ExecutionStopped
	ExecutionCancelled
	ExecutionInvalidInvocation
	ExecutionResourceExhausted
	ExecutionFailed
	ExecutionSourceFailure
)

// ExecutionResult distinguishes completion, sink termination, and failures.
type ExecutionResult struct {
	code  ExecutionCode
	cause error
}

// Code returns the outcome category.
func (r ExecutionResult) Code() ExecutionCode { return r.code }

// Err returns the underlying cancellation error, if any.
func (r ExecutionResult) Err() error { return r.cause }

// ExecutionOptions sets a positive budget and optional worker cap for queries.
// Execution uses a query-local scope plus conservative admission reservations.
// Scoped charges exclude borrowed sources, Go metadata, and unused slab capacity;
// admission reservations may be more conservative.
type ExecutionOptions struct {
	MemoryBudget int64
	// SpillDirectory enables temporary-file execution for eligible blocking
	// operators. Empty keeps execution in memory. Files are query-local.
	SpillDirectory string
	// TryPackedSort opts into exact 64-bit key packing for direct in-memory sorts.
	// A non-packable key or unsupported plan uses the existing sort path.
	TryPackedSort bool
	// Workers is a maximum per query. Zero chooses automatically; one forces
	// serial execution. Plans with blocking or non-partitionable stages fall
	// back to the serial pipeline.
	Workers int
}

// ExecuteWithOptions executes a plan with cooperative cancellation and
// conservative memory reservations. Sink-retained batches become the caller's
// responsibility after the sink returns.
func (p *PhysicalPlan) ExecuteWithOptions(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool) ExecutionResult {
	return p.executeWithScope(ctx, a, options, sink, nil)
}

func (p *PhysicalPlan) executeWithScope(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool, scope *mem.AllocationScope) ExecutionResult {
	if p == nil || p.source == nil || ctx == nil || a == nil || sink == nil || options.MemoryBudget <= 0 || options.Workers < 0 {
		return ExecutionResult{code: ExecutionInvalidInvocation}
	}
	if options.SpillDirectory != "" {
		if scope == nil {
			scope = mem.MakeAllocationScope(a, options.MemoryBudget)
		}
		if result, eligible := p.executeSpilled(ctx, a, options, sink, scope); eligible {
			if result.code == ExecutionFailed && scope.Exhausted() {
				result.code = ExecutionResourceExhausted
			}
			return result
		}
	}
	if result, handled := p.executeGlobalDistinctWithScope(ctx, a, options, sink, scope); handled {
		return result
	}
	if options.Workers != 1 {
		if result, parallel := p.executeParallelDistinctWithScope(ctx, a, options, sink, scope); parallel {
			return result
		}
		if result, parallel := p.executeParallelWithScope(ctx, a, options, sink, scope); parallel {
			return result
		}
	}
	if scope == nil {
		scope = mem.MakeAllocationScope(a, options.MemoryBudget)
	}
	a = mem.MakeAllocatorWithScope(a, scope)
	limitLeft := make([]int64, len(p.steps))
	offsetLeft := make([]int64, len(p.steps))
	for i, step := range p.steps {
		limitLeft[i], offsetLeft[i] = step.limit, step.offset
	}
	aggregateStates := make([][]aggregateValue, len(p.steps))
	aggregateUniques := make([][]*distinctState, len(p.steps))
	groupStates := make([]*groupState, len(p.steps))
	distinctStates := make([]*distinctState, len(p.steps))
	sortStates := make([]*sortState, len(p.steps))
	joinStates := make([]*joinState, len(p.steps))
	defer func() {
		for _, states := range aggregateStates {
			for i := range states {
				states[i].release()
			}
		}
		for _, uniques := range aggregateUniques {
			for _, state := range uniques {
				if state != nil {
					state.release()
				}
			}
		}
		for _, state := range distinctStates {
			if state != nil {
				state.release()
			}
		}
		for _, state := range sortStates {
			if state != nil {
				state.release()
			}
		}
		for _, state := range groupStates {
			if state != nil {
				state.release()
			}
		}
		for _, state := range joinStates {
			if state != nil {
				state.release()
			}
		}
	}()
	for i, step := range p.steps {
		if step.operation == physicalAggregate {
			if len(step.groupKeys) == 0 {
				aggregateStates[i] = make([]aggregateValue, len(step.aggregates))
				aggregateUniques[i] = make([]*distinctState, len(step.aggregates))
				for j, aggregate := range step.aggregates {
					if aggregate.distinct && (len(step.aggregateSources) == 0 || step.aggregateSources[j] == j) {
						aggregateUniques[i][j] = &distinctState{}
						aggregateUniques[i][j].setScope(scope)
					}
				}
			} else {
				groupStates[i] = makeGroupState(step, scope)
			}
		} else if step.operation == physicalDistinct {
			distinctStates[i] = &distinctState{}
			distinctStates[i].setScope(scope)
		} else if step.operation == physicalSort {
			sortStates[i] = &sortState{compact: makeCompactSortState(step), top: makeCompactTopNState(step)}
			if options.TryPackedSort && i == 0 && p.source.table != nil {
				packed, ok := makePackedSortState(ctx, a, p.source.table, step, scope)
				if err := ctx.Err(); err != nil {
					return ExecutionResult{code: ExecutionCancelled, cause: err}
				}
				if !ok && scope.Exhausted() {
					return ExecutionResult{code: ExecutionResourceExhausted}
				}
				if ok {
					sortStates[i].packed = packed
					sortStates[i].compact = nil
					sortStates[i].top = nil
				}
			}
			if sortStates[i].compact != nil {
				sortStates[i].compact.scope, sortStates[i].compact.ctx = scope, ctx
				sortStates[i].compact.workers = compactSortWorkers(options.Workers)
			}
			if sortStates[i].top != nil {
				sortStates[i].top.scope, sortStates[i].top.rows.scope = scope, scope
			}
		} else if step.operation == physicalJoin {
			joinStates[i] = &joinState{}
			joinStates[i].scope = scope
			joinStates[i].keys.setScope(scope)
		}
	}
	for i, step := range p.steps {
		if step.operation == physicalJoin {
			result := joinStates[i].build(ctx, a, step.join, options.MemoryBudget-aggregateReservation(aggregateStates)-aggregateUniqueReservation(aggregateUniques)-distinctReservation(distinctStates)-sortReservation(sortStates)-groupReservation(groupStates, p.steps)-joinReservation(joinStates))
			if result.Code() != ExecutionCompleted {
				return result
			}
		}
	}
	runtimeFilter, runtimeSortAt := makeTopNRuntimeFilter(p.source, p.steps)
	if runtimeFilter == nil {
		for i, step := range p.steps {
			if step.operation == physicalJoin {
				var err error
				runtimeFilter, err = makeJoinRuntimeFilter(ctx, p.source, p.steps, i, joinStates[i])
				if err != nil {
					return ExecutionResult{code: ExecutionCancelled, cause: err}
				}
				if runtimeFilter != nil {
					break
				}
			}
		}
	}
	cursor := scanCursor{source: p.source, runtime: runtimeFilter}
	defer cursor.close()
	sourceDone := false
	for {
		if err := ctx.Err(); err != nil {
			return ExecutionResult{code: ExecutionCancelled, cause: err}
		}
		if streamingLimitDone(p.steps, limitLeft, 0) {
			break
		}
		for j, step := range p.steps {
			if step.operation == physicalAggregate || step.operation == physicalDistinct || step.operation == physicalSort || step.operation == physicalJoin && step.join.kind == JoinFull {
				break
			}
			if step.operation == physicalLimit && limitLeft[j] == 0 {
				sourceDone = true
				break
			}
		}
		if sourceDone {
			break
		}
		batch, done, err := cursor.next(ctx, a)
		if err != nil {
			return scanExecutionError(err, scope)
		}
		if done {
			break
		}
		if p.batchReservation(batch) > options.MemoryBudget-aggregateReservation(aggregateStates)-aggregateUniqueReservation(aggregateUniques)-distinctReservation(distinctStates)-sortReservation(sortStates)-groupReservation(groupStates, p.steps)-joinReservation(joinStates) {
			batch.Release()
			return ExecutionResult{code: ExecutionResourceExhausted}
		}
		result := p.executeSteps(ctx, a, batch, 0, limitLeft, offsetLeft, aggregateStates, aggregateUniques, distinctStates, sortStates, groupStates, joinStates, options.MemoryBudget, sink)
		if runtimeSortAt >= 0 && sortStates[runtimeSortAt] != nil {
			publishTopNRuntimeFilter(runtimeFilter, sortStates[runtimeSortAt].top, p.steps[runtimeSortAt])
		}
		if result.code == ExecutionFailed && scope.Exhausted() {
			return ExecutionResult{code: ExecutionResourceExhausted}
		}
		if result.code != ExecutionCompleted {
			return result
		}
		if aggregateReservation(aggregateStates)+aggregateUniqueReservation(aggregateUniques)+distinctReservation(distinctStates)+sortReservation(sortStates)+groupReservation(groupStates, p.steps)+joinReservation(joinStates) > options.MemoryBudget {
			return ExecutionResult{code: ExecutionResourceExhausted}
		}
	}
	for i, step := range p.steps {
		if step.operation != physicalAggregate && step.operation != physicalDistinct && step.operation != physicalSort && (step.operation != physicalJoin || step.join.kind != JoinFull) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return ExecutionResult{code: ExecutionCancelled, cause: err}
		}
		if aggregateReservation(aggregateStates)+aggregateUniqueReservation(aggregateUniques)+distinctReservation(distinctStates)+sortReservation(sortStates)+groupReservation(groupStates, p.steps)+joinReservation(joinStates) > options.MemoryBudget {
			return ExecutionResult{code: ExecutionResourceExhausted}
		}
		var outputBound int64
		switch step.operation {
		case physicalAggregate:
			if groupStates[i] == nil {
				outputBound = int64(len(step.aggregates)) * 64
			} else {
				outputBound = groupStates[i].charged(len(step.aggregates))
			}
		case physicalDistinct:
			outputBound = distinctStates[i].charged
		case physicalSort:
			outputBound = sortStates[i].charged
		}
		used := saturatingAdd(aggregateReservation(aggregateStates), aggregateUniqueReservation(aggregateUniques))
		used = saturatingAdd(used, distinctReservation(distinctStates))
		used = saturatingAdd(used, sortReservation(sortStates))
		used = saturatingAdd(used, groupReservation(groupStates, p.steps))
		used = saturatingAdd(used, joinReservation(joinStates))
		if outputBound > options.MemoryBudget-used {
			return ExecutionResult{code: ExecutionResourceExhausted}
		}
		var batch *store.Batch
		if step.operation == physicalAggregate {
			if groupStates[i] == nil {
				batch = finalizeAggregates(a, step, aggregateStates[i])
			} else {
				if groupStates[i].compact != nil && groupStates[i].compact.length == 0 || groupStates[i].compact == nil && len(groupStates[i].values) == 0 {
					continue
				}
				batch = groupStates[i].finish(a, step)
			}
		} else if step.operation == physicalDistinct {
			if distinctStates[i].rows.length == 0 {
				continue
			}
			batch = distinctStates[i].rows.makeBatch(a, step.schema)
		} else if step.operation == physicalSort {
			if sortStates[i].packed != nil && sortStates[i].packed.length == 0 || sortStates[i].compact != nil && sortStates[i].compact.length == 0 || sortStates[i].top != nil && sortStates[i].top.length == 0 || sortStates[i].packed == nil && sortStates[i].compact == nil && sortStates[i].top == nil && len(sortStates[i].rows) == 0 {
				continue
			}
			batch = sortStates[i].finish(a, step)
		} else {
			if streamingLimitDone(p.steps, limitLeft, i+1) {
				continue
			}
			result := joinStates[i].finish(ctx, a, step, options.MemoryBudget-aggregateReservation(aggregateStates)-aggregateUniqueReservation(aggregateUniques)-distinctReservation(distinctStates)-sortReservation(sortStates)-groupReservation(groupStates, p.steps)-joinReservation(joinStates)+joinStates[i].charged+joinStates[i].keys.charged, func(output *store.Batch) ExecutionResult {
				return p.executeSteps(ctx, a, output, i+1, limitLeft, offsetLeft, aggregateStates, aggregateUniques, distinctStates, sortStates, groupStates, joinStates, options.MemoryBudget, sink)
			})
			if result.code != ExecutionCompleted {
				return result
			}
			continue
		}
		if batch == nil {
			if err := ctx.Err(); err != nil {
				return ExecutionResult{code: ExecutionCancelled, cause: err}
			}
			if scope.Exhausted() {
				return ExecutionResult{code: ExecutionResourceExhausted}
			}
			return ExecutionResult{code: ExecutionFailed}
		}
		result := p.executeSteps(ctx, a, batch, i+1, limitLeft, offsetLeft, aggregateStates, aggregateUniques, distinctStates, sortStates, groupStates, joinStates, options.MemoryBudget, sink)
		if result.code == ExecutionFailed && scope.Exhausted() {
			return ExecutionResult{code: ExecutionResourceExhausted}
		}
		if result.code != ExecutionCompleted {
			return result
		}
	}
	return ExecutionResult{code: ExecutionCompleted}
}

func distinctReservation(states []*distinctState) int64 {
	var total int64
	for _, state := range states {
		if state != nil {
			total = saturatingAdd(total, state.charged)
		}
	}
	return total
}

func sortReservation(states []*sortState) int64 {
	var total int64
	for _, state := range states {
		if state != nil {
			total = saturatingAdd(total, state.charged)
		}
	}
	return total
}

func groupReservation(states []*groupState, steps []physicalStep) int64 {
	var total int64
	for i, state := range states {
		if state != nil {
			total = saturatingAdd(total, state.charged(len(steps[i].aggregates)))
		}
	}
	return total
}

func joinReservation(states []*joinState) int64 {
	var total int64
	for _, state := range states {
		if state != nil {
			total = saturatingAdd(total, saturatingAdd(state.charged, state.keys.charged))
		}
	}
	return total
}

func aggregateReservation(states [][]aggregateValue) int64 {
	var total int64
	for _, values := range states {
		total = saturatingAdd(total, int64(len(values))*64)
		for _, value := range values {
			if value.text != nil {
				total = saturatingAdd(total, int64(value.text.Len()))
			}
		}
	}
	return total
}

func aggregateUniqueReservation(states [][]*distinctState) int64 {
	var total int64
	for _, uniques := range states {
		for _, state := range uniques {
			if state != nil {
				total = saturatingAdd(total, state.charged)
			}
		}
	}
	return total
}

func (p *PhysicalPlan) executeSteps(ctx context.Context, a *mem.Allocator, batch *store.Batch, start int, limits, offsets []int64, aggregates [][]aggregateValue, uniques [][]*distinctState, distinct []*distinctState, sorts []*sortState, groups []*groupState, joins []*joinState, budget int64, sink func(*store.Batch) bool) ExecutionResult {
	defer func() { batch.Release() }()
	for j := start; j < len(p.steps); j++ {
		if err := ctx.Err(); err != nil {
			return ExecutionResult{code: ExecutionCancelled, cause: err}
		}
		step := p.steps[j]
		switch step.operation {
		case physicalPushedFilter:
			continue
		case physicalJoin:
			limited := false
			result := joins[j].probe(ctx, a, batch, step, budget-aggregateReservation(aggregates)-aggregateUniqueReservation(uniques)-distinctReservation(distinct)-sortReservation(sorts)-groupReservation(groups, p.steps)-joinReservation(joins)+joins[j].charged+joins[j].keys.charged, func(output *store.Batch) ExecutionResult {
				result := p.executeSteps(ctx, a, output, j+1, limits, offsets, aggregates, uniques, distinct, sorts, groups, joins, budget, sink)
				if result.code == ExecutionCompleted && streamingLimitDone(p.steps, limits, j+1) {
					limited = true
					return ExecutionResult{code: ExecutionStopped}
				}
				return result
			})
			if limited {
				return ExecutionResult{code: ExecutionCompleted}
			}
			return result
		case physicalLimit:
			if limits[j] == 0 {
				return ExecutionResult{code: ExecutionCompleted}
			}
			if !executePhysicalLimit(a, batch, &limits[j], &offsets[j]) {
				return ExecutionResult{code: ExecutionFailed}
			}
		case physicalFilter:
			if !executePhysicalFilter(a, batch, step.program) {
				return ExecutionResult{code: ExecutionFailed}
			}
		case physicalProject:
			projected := executePhysicalProject(a, batch, step.program)
			if projected == nil {
				return ExecutionResult{code: ExecutionFailed}
			}
			batch.Release()
			batch = projected
		case physicalAggregate:
			if groups[j] != nil {
				if !groups[j].add(a, batch, step, budget-aggregateReservation(aggregates)-aggregateUniqueReservation(uniques)-distinctReservation(distinct)-sortReservation(sorts)-groupReservation(groups, p.steps)+groups[j].charged(len(step.aggregates))-joinReservation(joins)) {
					return ExecutionResult{code: ExecutionResourceExhausted}
				}
			} else if !accumulateAggregates(a, batch, step, aggregates[j], uniques[j], budget-aggregateReservation(aggregates)-aggregateUniqueReservation(uniques)-distinctReservation(distinct)-sortReservation(sorts)-groupReservation(groups, p.steps)-joinReservation(joins)) {
				return ExecutionResult{code: ExecutionResourceExhausted}
			}
			return ExecutionResult{code: ExecutionCompleted}
		case physicalDistinct:
			selection := batch.Selection().RetainBitMap(a)
			if batch.Selection() != nil && selection == nil {
				return ExecutionResult{code: ExecutionResourceExhausted}
			}
			for row := range batch.Len() {
				if selection != nil && !selection.IsSet(row) {
					continue
				}
				if _, ok := distinct[j].add(a, batch, row, budget-aggregateReservation(aggregates)-aggregateUniqueReservation(uniques)-distinctReservation(distinct)+distinct[j].charged-sortReservation(sorts)-groupReservation(groups, p.steps)-joinReservation(joins)); !ok {
					if selection != nil {
						selection.Release()
					}
					return ExecutionResult{code: ExecutionResourceExhausted}
				}
			}
			if selection != nil {
				selection.Release()
			}
			return ExecutionResult{code: ExecutionCompleted}
		case physicalSort:
			if !sorts[j].add(a, batch, step, budget-aggregateReservation(aggregates)-aggregateUniqueReservation(uniques)-distinctReservation(distinct)-sortReservation(sorts)+sorts[j].charged-groupReservation(groups, p.steps)-joinReservation(joins)) {
				return ExecutionResult{code: ExecutionResourceExhausted}
			}
			return ExecutionResult{code: ExecutionCompleted}
		default:
			return ExecutionResult{code: ExecutionFailed}
		}
	}
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}
	}
	if !sink(batch) {
		return ExecutionResult{code: ExecutionStopped}
	}
	return ExecutionResult{code: ExecutionCompleted}
}

func streamingLimitDone(steps []physicalStep, limits []int64, start int) bool {
	if limits == nil {
		return false
	}
	for i := start; i < len(steps); i++ {
		switch steps[i].operation {
		case physicalAggregate, physicalDistinct, physicalSort:
			return false
		case physicalLimit:
			if limits[i] == 0 {
				return true
			}
		}
	}
	return false
}

func executePhysicalLimit(a *mem.Allocator, batch *store.Batch, remaining, skipped *int64) bool {
	if batch.ActiveLen() == 0 {
		return true
	}
	mask := store.MakeBitMap(a, batch.Len())
	if mask == nil {
		return false
	}
	var existing *store.BitMap
	if batch.Selection() != nil {
		existing = batch.Selection().RetainBitMap(a)
		if existing == nil {
			mask.Release()
			return false
		}
		defer existing.Release()
	}
	for row := range batch.Len() {
		if existing != nil && !existing.IsSet(row) {
			continue
		}
		if *skipped > 0 {
			*skipped--
			continue
		}
		if *remaining > 0 {
			mask.Set(row)
			*remaining--
		}
	}
	if !batch.SetSelection(store.MakeRowSelectionFromBitMap(mask)) {
		mask.Release()
		return false
	}
	return true
}

func (p *PhysicalPlan) batchReservation(batch *store.Batch) int64 {
	rows := int64(batch.Len())
	input := int64(2)
	maxStringLen := int64(0)
	stringColumns := int64(0)
	for column := range batch.NVectors() {
		v := batch.VectorAt(column)
		input = saturatingAdd(input, int64((v.Type().SlotBits()+7)/8)+1)
		if v.TypeID() == dtype.STRT {
			stringColumns++
			strings := v.Strings()
			if dictionary := v.Dictionary(); dictionary != nil {
				strings = dictionary.Strings()
			}
			for _, value := range strings {
				maxStringLen = max(maxStringLen, int64(len(value.View())))
			}
		}
	}
	input = saturatingAdd(input, saturatingMultiply(stringColumns, maxStringLen))
	perRow := input
	for _, step := range p.steps {
		stage := step.program.reservation(input, maxStringLen)
		for _, program := range step.groupKeys {
			stage = saturatingAdd(stage, program.reservation(input, maxStringLen))
		}
		// Global aggregate inputs run sequentially; grouped inputs remain live together.
		aggregatePeak := int64(0)
		for _, aggregate := range step.aggregates {
			if len(step.groupKeys) == 0 {
				aggregatePeak = max(aggregatePeak, aggregate.program.reservation(input, maxStringLen))
			} else {
				aggregatePeak = saturatingAdd(aggregatePeak, aggregate.program.reservation(input, maxStringLen))
			}
		}
		stage = saturatingAdd(stage, aggregatePeak)
		for _, key := range step.order {
			stage = saturatingAdd(stage, key.program.reservation(input, maxStringLen))
		}
		if step.join != nil {
			for _, program := range step.join.leftKeys {
				stage = saturatingAdd(stage, program.reservation(input, maxStringLen))
			}
			if step.join.residual != nil {
				stage = saturatingAdd(stage, step.join.residual.reservation(input, maxStringLen))
			}
		}
		perRow = saturatingAdd(perRow, stage)
	}
	return saturatingMultiply(rows, perRow)
}

func (p physicalExprProgram) reservation(input, maxStringLen int64) int64 {
	depths := make([]int64, len(p.nodes))
	depth, perRow := int64(0), int64(0)
	for i, node := range p.nodes {
		if node.dType.ID() == dtype.STRT && node.kind == exprLiteral {
			maxStringLen = max(maxStringLen, int64(len(node.literal.stringValue())))
		}
		for child := range int(node.childCount) {
			depths[i] = max(depths[i], depths[node.children[child]])
		}
		if node.kind == exprTernary && node.operation == exprOpCase {
			depths[i]++
		}
		depth = max(depth, depths[i])
	}
	for _, node := range p.nodes {
		perRow = saturatingAdd(perRow, int64((node.dType.SlotBits()+7)/8)+1)
		if node.dType.ID() == dtype.STRT {
			perRow = saturatingAdd(perRow, maxStringLen)
		}
	}
	// Each nested CASE can keep a gathered input and its row indices live.
	return saturatingAdd(saturatingMultiply(perRow, depth+1), saturatingMultiply(saturatingAdd(input, 8), depth))
}

func saturatingMultiply(a, b int64) int64 {
	if a != 0 && b > math.MaxInt64/a {
		return math.MaxInt64
	}
	return a * b
}

func saturatingAdd(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}
