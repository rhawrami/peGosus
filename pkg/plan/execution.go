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
	if p == nil || p.source == nil || ctx == nil || a == nil || sink == nil || options.MemoryBudget <= 0 || options.Workers < 0 {
		return ExecutionResult{code: ExecutionInvalidInvocation}
	}
	if options.Workers != 1 {
		if result, parallel := p.executeParallel(ctx, a, options, sink); parallel {
			return result
		}
	}
	scope := mem.MakeAllocationScope(a, options.MemoryBudget)
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
					if aggregate.distinct {
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
				sortStates[i].compact.scope = scope
			}
			if sortStates[i].top != nil {
				sortStates[i].top.scope, sortStates[i].top.rows.scope = scope, scope
			}
		} else if step.operation == physicalJoin {
			joinStates[i] = &joinState{}
			joinStates[i].scope = scope
			joinStates[i].keys.setScope(scope)
			joinStates[i].rightRows.scope = scope
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
	cursor := scanCursor{source: p.source}
	defer cursor.close()
	sourceDone := false
	for {
		if err := ctx.Err(); err != nil {
			return ExecutionResult{code: ExecutionCancelled, cause: err}
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
			var ok bool
			batch, ok = joinStates[i].finish(a, step, options.MemoryBudget-aggregateReservation(aggregateStates)-aggregateUniqueReservation(aggregateUniques)-distinctReservation(distinctStates)-sortReservation(sortStates)-groupReservation(groupStates, p.steps)-joinReservation(joinStates)+joinStates[i].charged+joinStates[i].keys.charged)
			if !ok {
				return ExecutionResult{code: ExecutionResourceExhausted}
			}
			if batch == nil {
				continue
			}
		}
		if batch == nil {
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
			joined, result := joins[j].probe(a, batch, step, budget-aggregateReservation(aggregates)-aggregateUniqueReservation(uniques)-distinctReservation(distinct)-sortReservation(sorts)-groupReservation(groups, p.steps)-joinReservation(joins)+joins[j].charged+joins[j].keys.charged)
			if result.Code() != ExecutionCompleted {
				return result
			}
			if joined == nil {
				return result
			}
			batch.Release()
			batch = joined
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
	perRow := int64(2)
	maxStringLen := int64(0)
	for column := range batch.NVectors() {
		v := batch.VectorAt(column)
		perRow = saturatingAdd(perRow, int64((v.Type().SlotBits()+7)/8)+1)
		if v.TypeID() == dtype.STRT {
			strings := v.Strings()
			if dictionary := v.Dictionary(); dictionary != nil {
				strings = dictionary.Strings()
			}
			for _, value := range strings {
				if n := int64(len(value.View())); n > maxStringLen {
					maxStringLen = n
				}
			}
		}
	}
	caseDepth := int64(0)
	stringNodes := int64(0)
	for _, step := range p.steps {
		programs := []physicalExprProgram{step.program}
		for _, aggregate := range step.aggregates {
			programs = append(programs, aggregate.program)
		}
		programs = append(programs, step.groupKeys...)
		for _, key := range step.order {
			programs = append(programs, key.program)
		}
		if step.join != nil {
			programs = append(programs, step.join.leftKeys...)
			if step.join.residual != nil {
				programs = append(programs, *step.join.residual)
			}
		}
		for _, program := range programs {
			for _, node := range program.nodes {
				perRow = saturatingAdd(perRow, int64((node.dType.SlotBits()+7)/8)+1)
				if node.kind == exprTernary && node.operation == exprOpCase {
					caseDepth++
				}
				if node.dType.ID() == dtype.STRT {
					stringNodes++
					if node.kind == exprLiteral && int64(len(node.literal.stringValue())) > maxStringLen {
						maxStringLen = int64(len(node.literal.stringValue()))
					}
				}
			}
		}
	}
	if maxStringLen != 0 {
		if stringNodes+int64(batch.NVectors()) > math.MaxInt64/maxStringLen {
			return math.MaxInt64
		}
		perRow = saturatingAdd(perRow, (stringNodes+int64(batch.NVectors()))*maxStringLen)
	}
	if caseDepth > 0 {
		if perRow > math.MaxInt64/(caseDepth+2) {
			return math.MaxInt64
		}
		perRow *= caseDepth + 2
	}
	if rows > 0 && perRow > math.MaxInt64/rows {
		return math.MaxInt64
	}
	return rows * perRow
}

func saturatingAdd(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}
