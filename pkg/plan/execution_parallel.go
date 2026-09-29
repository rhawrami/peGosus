package plan

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/rhawrami/peGosus/pkg/io/csv"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

const (
	// These thresholds keep the measured small-input and cheap table-mapping paths serial.
	parallelMinRows             = 8 << 10
	parallelRowsPerWorker       = 4 << 10
	parallelMinBudgetPerWorker  = 1 << 20
	parallelMinParquetFileBytes = 64 << 10
)

func scanExecutionError(err error, scope *mem.AllocationScope) ExecutionResult {
	if errors.Is(err, errScanFilter) {
		if scope.Exhausted() {
			return ExecutionResult{code: ExecutionResourceExhausted, cause: err}
		}
		return ExecutionResult{code: ExecutionFailed, cause: err}
	}
	var parquetError *parquet.ParquetError
	if errors.As(err, &parquetError) {
		switch parquetError.Code() {
		case parquet.ParquetResourceExhausted:
			return ExecutionResult{code: ExecutionResourceExhausted, cause: err}
		case parquet.ParquetCancelled:
			return ExecutionResult{code: ExecutionCancelled, cause: err}
		}
	}
	var csvError *csv.CSVError
	if errors.As(err, &csvError) {
		switch csvError.Code() {
		case csv.CSVResourceExhausted:
			return ExecutionResult{code: ExecutionResourceExhausted, cause: err}
		case csv.CSVCancelled:
			return ExecutionResult{code: ExecutionCancelled, cause: err}
		}
	}
	return ExecutionResult{code: ExecutionSourceFailure, cause: err}
}

func (p *PhysicalPlan) executeParallel(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool) (ExecutionResult, bool) {
	if p.source.table == nil && p.source.parquetPath == "" {
		return ExecutionResult{}, false
	}
	aggregateAt := -1
	for i, step := range p.steps {
		switch step.operation {
		case physicalPushedFilter, physicalFilter, physicalProject:
			if aggregateAt >= 0 {
				return ExecutionResult{}, false
			}
		case physicalAggregate:
			if aggregateAt >= 0 {
				return ExecutionResult{}, false
			}
			for _, aggregate := range step.aggregates {
				if aggregate.distinct {
					return ExecutionResult{}, false
				}
			}
			aggregateAt = i
		case physicalSort:
			if aggregateAt < 0 || i != aggregateAt+1 {
				return ExecutionResult{}, false
			}
		case physicalLimit:
			if aggregateAt < 0 || i != aggregateAt+2 || p.steps[i-1].operation != physicalSort {
				return ExecutionResult{}, false
			}
		default:
			return ExecutionResult{}, false
		}
	}
	if options.Workers == 0 && p.source.table != nil && aggregateAt < 0 {
		return ExecutionResult{}, false
	}
	sampleGroup := options.Workers == 0 && aggregateAt >= 0 && len(p.steps[aggregateAt].groupKeys) != 0 && makeCompactGroupCountState(p.steps[aggregateAt]) == nil
	if sampleGroup && p.source.table == nil {
		return ExecutionResult{}, false
	}
	workers := runtime.GOMAXPROCS(0)
	if options.Workers > 1 {
		workers = min(workers, options.Workers)
	}
	if workers < 2 {
		return ExecutionResult{}, false
	}
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}, true
	}
	if p.source.parquetPath != "" && options.Workers == 0 {
		info, err := os.Stat(p.source.parquetPath)
		if err != nil || info.Size() < parallelMinParquetFileBytes {
			return ExecutionResult{}, false
		}
	}
	scope := mem.MakeAllocationScope(a, options.MemoryBudget)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	var cursor scanCursor
	var tasks int
	var nonemptyTasks int
	var totalRows int64
	if p.source.table != nil {
		tasks = p.source.table.NBatches()
		for i := range tasks {
			rows := p.source.table.BatchAt(i).ActiveLen()
			if rows > 0 {
				nonemptyTasks++
			}
			totalRows = saturatingAdd(totalRows, int64(rows))
		}
	} else {
		cursor.source = p.source
		defer cursor.close()
		if err := cursor.openParquet(scoped); err != nil {
			return scanExecutionError(err, scope), true
		}
		tasks = cursor.parquet.RowGroupCount()
		for i := range tasks {
			rows := cursor.parquet.RowGroupRows(i)
			if rows > 0 {
				nonemptyTasks++
			}
			totalRows = saturatingAdd(totalRows, rows)
		}
	}
	if nonemptyTasks < 2 {
		return ExecutionResult{}, false
	}
	workers = min(workers, nonemptyTasks)
	if options.Workers == 0 {
		if totalRows < parallelMinRows {
			return ExecutionResult{}, false
		}
		if rowsPerWorker := totalRows / parallelRowsPerWorker; rowsPerWorker < int64(workers) {
			workers = int(rowsPerWorker)
		}
		workers = min(workers, 8)
		if totalRows < 128<<10 {
			workers = min(workers, 4)
		}
	}
	if budgetWorkers := options.MemoryBudget / parallelMinBudgetPerWorker; budgetWorkers < int64(workers) {
		workers = int(budgetWorkers)
	}
	if workers < 2 {
		return ExecutionResult{}, false
	}
	if sampleGroup && !p.sampleGroupCardinality(scoped, aggregateAt) {
		return ExecutionResult{}, false
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var next atomic.Int64
	var resultMu sync.Mutex
	result := ExecutionResult{code: ExecutionCompleted}
	record := func(failure ExecutionResult) {
		if failure.code == ExecutionCompleted {
			return
		}
		resultMu.Lock()
		if result.code == ExecutionCompleted {
			result = failure
			cancel()
		}
		resultMu.Unlock()
	}
	var sinkMu sync.Mutex
	workerStates := make([][]aggregateValue, workers)
	workerGroups := make([]*groupState, workers)
	defer func() {
		for _, states := range workerStates {
			for i := range states {
				states[i].release()
			}
		}
		for _, state := range workerGroups {
			if state != nil {
				state.release()
			}
		}
	}()
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			var aggregates [][]aggregateValue
			var uniques [][]*distinctState
			var distinct []*distinctState
			var sorts []*sortState
			var groups []*groupState
			var joins []*joinState
			if aggregateAt >= 0 {
				step := p.steps[aggregateAt]
				if len(step.groupKeys) != 0 {
					workerGroups[worker] = makeGroupState(step, scope)
					groups = make([]*groupState, len(p.steps))
					groups[aggregateAt] = workerGroups[worker]
				} else {
					workerStates[worker] = make([]aggregateValue, len(step.aggregates))
					aggregates = make([][]aggregateValue, len(p.steps))
					aggregates[aggregateAt] = workerStates[worker]
					groups = make([]*groupState, len(p.steps))
				}
				uniques = make([][]*distinctState, len(p.steps))
				distinct = make([]*distinctState, len(p.steps))
				sorts = make([]*sortState, len(p.steps))
				joins = make([]*joinState, len(p.steps))
			}
			consume := func(batch *store.Batch) bool {
				if err := ctx.Err(); err != nil {
					batch.Release()
					record(ExecutionResult{code: ExecutionCancelled, cause: err})
					return false
				}
				if err := runCtx.Err(); err != nil {
					batch.Release()
					return false
				}
				if p.batchReservation(batch) > options.MemoryBudget {
					batch.Release()
					record(ExecutionResult{code: ExecutionResourceExhausted})
					return false
				}
				output := func(current *store.Batch) bool {
					sinkMu.Lock()
					defer sinkMu.Unlock()
					if runCtx.Err() != nil {
						return false
					}
					if !sink(current) {
						record(ExecutionResult{code: ExecutionStopped})
						return false
					}
					return true
				}
				outcome := p.executeSteps(runCtx, scoped, batch, 0, nil, nil, aggregates, uniques, distinct, sorts, groups, joins, options.MemoryBudget, output)
				if outcome.code == ExecutionFailed && scope.Exhausted() {
					outcome = ExecutionResult{code: ExecutionResourceExhausted}
				}
				if outcome.code == ExecutionStopped && ctx.Err() != nil {
					outcome = ExecutionResult{code: ExecutionCancelled, cause: ctx.Err()}
				}
				record(outcome)
				return outcome.code == ExecutionCompleted
			}
			for runCtx.Err() == nil {
				at := int(next.Add(1) - 1)
				if at >= tasks {
					return
				}
				if p.source.table != nil {
					local := scanCursor{source: p.source, index: at}
					batch, done, err := local.next(runCtx, scoped)
					if err != nil {
						record(scanExecutionError(err, scope))
						return
					}
					if done || !consume(batch) {
						return
					}
					continue
				}
				reader := cursor.parquet.MakeRowGroupReader(at, scoped)
				if reader == nil {
					record(ExecutionResult{code: ExecutionFailed})
					return
				}
				for runCtx.Err() == nil {
					batch, err := reader.Next(runCtx)
					if err != nil {
						record(scanExecutionError(err, scope))
						break
					}
					if batch == nil {
						break
					}
					if !cursor.applyFilters(scoped, batch) {
						batch.Release()
						record(scanExecutionError(errScanFilter, scope))
						break
					}
					if !consume(batch) {
						break
					}
				}
				reader.Close()
			}
			if err := ctx.Err(); err != nil {
				record(ExecutionResult{code: ExecutionCancelled, cause: err})
			}
		}(worker)
	}
	wg.Wait()
	if result.code != ExecutionCompleted {
		return result, true
	}
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}, true
	}
	if aggregateAt >= 0 {
		step := p.steps[aggregateAt]
		tail := makeParallelAggregateTail(ctx, scoped, scope, p.steps, aggregateAt, options.MemoryBudget, sink)
		defer tail.release()
		if len(step.groupKeys) != 0 {
			if result, partitioned := p.executeGroupedShards(ctx, scoped, scope, step, workerGroups, options.MemoryBudget, totalRows, tail.consume); partitioned {
				if result.code == ExecutionStopped {
					return tail.stopped(), true
				}
				if result.code != ExecutionCompleted {
					return result, true
				}
				return tail.finish(), true
			}
			var remaining int64
			for _, state := range workerGroups {
				if state != nil {
					remaining = saturatingAdd(remaining, state.charged(len(step.aggregates)))
				}
			}
			if remaining > options.MemoryBudget {
				return ExecutionResult{code: ExecutionResourceExhausted}, true
			}
			var merged *groupState
			for i, state := range workerGroups {
				if state == nil {
					continue
				}
				charge := state.charged(len(step.aggregates))
				if merged == nil {
					merged = state
					workerGroups[i] = nil
					remaining -= charge
					continue
				}
				if err := ctx.Err(); err != nil {
					merged.release()
					return ExecutionResult{code: ExecutionCancelled, cause: err}, true
				}
				if !merged.merge(scoped, state, step, options.MemoryBudget-remaining) {
					merged.release()
					return ExecutionResult{code: ExecutionResourceExhausted}, true
				}
				remaining -= charge
				state.release()
				workerGroups[i] = nil
			}
			if merged == nil {
				return tail.finish(), true
			}
			defer merged.release()
			length := len(merged.values)
			if merged.compact != nil {
				length = merged.compact.length
			}
			if length == 0 {
				return tail.finish(), true
			}
			if merged.charged(len(step.aggregates)) > options.MemoryBudget-merged.charged(len(step.aggregates)) {
				return ExecutionResult{code: ExecutionResourceExhausted}, true
			}
			batch := merged.finish(scoped, step)
			if batch == nil {
				if scope.Exhausted() {
					return ExecutionResult{code: ExecutionResourceExhausted}, true
				}
				return ExecutionResult{code: ExecutionFailed}, true
			}
			defer batch.Release()
			if err := ctx.Err(); err != nil {
				return ExecutionResult{code: ExecutionCancelled, cause: err}, true
			}
			if !tail.consume(batch) {
				return tail.stopped(), true
			}
			return tail.finish(), true
		}
		merged := make([]aggregateValue, len(step.aggregates))
		defer func() {
			for i := range merged {
				merged[i].release()
			}
		}()
		for _, states := range workerStates {
			for i, aggregate := range step.aggregates {
				mergeAggregate(&merged[i], &states[i], aggregate.kind)
			}
		}
		batch := finalizeAggregates(scoped, step, merged)
		if batch == nil {
			if scope.Exhausted() {
				return ExecutionResult{code: ExecutionResourceExhausted}, true
			}
			return ExecutionResult{code: ExecutionFailed}, true
		}
		defer batch.Release()
		if err := ctx.Err(); err != nil {
			return ExecutionResult{code: ExecutionCancelled, cause: err}, true
		}
		if !tail.consume(batch) {
			return tail.stopped(), true
		}
		return tail.finish(), true
	}
	return ExecutionResult{code: ExecutionCompleted}, true
}
