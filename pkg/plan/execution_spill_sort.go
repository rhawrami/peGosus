package plan

import (
	"context"
	"errors"
	"os"
	"sort"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

const spillBatchRows = 128

type spillWorkspace struct{ directory string }

func (w *spillWorkspace) create() (*os.File, error) { return os.CreateTemp(w.directory, "run-") }

func (p *PhysicalPlan) executeSpilled(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool, scope *mem.AllocationScope) (ExecutionResult, bool) {
	if result, handled := p.executeSpilledJoin(ctx, a, options, sink, scope); handled {
		return result, true
	}
	if result, handled := p.executeSpilledGroup(ctx, a, options, sink, scope); handled {
		return result, true
	}
	at := -1
	for i, step := range p.steps {
		switch step.operation {
		case physicalFilter, physicalPushedFilter, physicalProject:
			continue
		case physicalSort:
			at = i
		}
		break
	}
	if at < 0 {
		return ExecutionResult{}, false
	}
	for _, step := range p.steps[at+1:] {
		if step.operation != physicalFilter && step.operation != physicalProject && step.operation != physicalLimit {
			return ExecutionResult{}, false
		}
	}
	if scope == nil {
		scope = mem.MakeAllocationScope(a, options.MemoryBudget)
	}
	a = mem.MakeAllocatorWithScope(a, scope)
	directory, err := os.MkdirTemp(options.SpillDirectory, "peg-query-")
	if err != nil {
		return spillExecutionError(ctx, err), true
	}
	defer os.RemoveAll(directory)
	workspace := spillWorkspace{directory: directory}
	step := p.steps[at]
	extended := step.schema
	extended.fields = append([]Field(nil), step.schema.fields...)
	for _, order := range step.order {
		node := order.program.nodes[order.program.roots[0]]
		extended.fields = append(extended.fields, Field{dType: node.dType, nullable: node.nullable})
	}
	keys := len(step.order)
	run := packedRows{scope: scope}
	defer run.release()
	paths := make([]string, 0)
	flush := func() error {
		if run.length == 0 {
			return nil
		}
		file, err := workspace.create()
		if err != nil {
			return err
		}
		stream := makeSpillStream(ctx, a, file, options.MemoryBudget)
		if stream == nil {
			file.Close()
			return errSpillBudget
		}
		defer stream.release()
		indices := a.AllocSegTemp(run.length * 8)
		if indices == nil {
			return errSpillBudget
		}
		defer indices.Dec()
		ids := indices.AsU64T()
		for i := range ids {
			ids[i] = uint64(i)
		}
		sort.Slice(ids, func(i, j int) bool {
			for k, order := range step.order {
				l, r := run.at(int(ids[i]), step.schema.Len()+k), run.at(int(ids[j]), step.schema.Len()+k)
				if l.IsNull() || r.IsNull() {
					if l.IsNull() != r.IsNull() {
						return l.IsNull() == order.nullsFirst
					}
					continue
				}
				cmp := compareOrdered(l, r)
				if cmp != 0 {
					if order.descending {
						return cmp > 0
					}
					return cmp < 0
				}
			}
			return ids[i] < ids[j]
		})
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := run.makeBatch(a, extended)
		if batch == nil {
			return errSpillBudget
		}
		defer batch.Release()
		for _, row := range ids {
			if err := stream.writeRow(batch, int(row)); err != nil {
				return err
			}
		}
		if err := stream.flush(); err != nil {
			return err
		}
		paths = append(paths, file.Name())
		run.release()
		run.scope = scope
		return nil
	}
	var failure error
	prefix := *p
	prefix.steps = p.steps[:at]
	consume := func(batch *store.Batch) bool {
		values := make([]physicalExprValues, keys)
		vectors := make([]store.Vector, extended.Len())
		defer func() {
			for i := range values {
				values[i].release()
			}
			for i := range vectors {
				vectors[i].Release()
			}
		}()
		for column := range batch.NVectors() {
			vectors[column] = batch.VectorAt(column).Retain()
		}
		for k, order := range step.order {
			var ok bool
			values[k], ok = executePhysicalExprProgram(a, batch, order.program)
			if !ok {
				failure = errSpillBudget
				return false
			}
			vectors[step.schema.Len()+k] = values[k].vectors[order.program.roots[0]].Retain()
		}
		keyed := store.MakeBatch(vectors)
		if keyed == nil {
			failure = errSpillBudget
			return false
		}
		vectors = nil
		defer keyed.Release()
		mask, selected := batch.Selection().AsBitMap()
		selection, offsets := batch.Selection().AsSelVec()
		cursor := 0
		for row := range batch.Len() {
			if selected && !mask.IsSet(row) {
				continue
			}
			if offsets {
				for cursor < len(selection.Offsets()) && int(selection.Offsets()[cursor]) < row {
					cursor++
				}
				if cursor == len(selection.Offsets()) || int(selection.Offsets()[cursor]) != row {
					continue
				}
			}
			if err := ctx.Err(); err != nil {
				failure = err
				return false
			}
			needed := int64(extended.Len() * 96)
			for c := range keyed.NVectors() {
				v := keyed.VectorAt(c)
				if v.Type().ID() == 0 {
					failure = errSpillFormat
					return false
				}
				value := scalarAt(v, row)
				needed = saturatingAdd(needed, int64(len(value.text))*4)
			}
			if run.length != 0 && (run.bytes()*3+needed > options.MemoryBudget/8 || needed > scope.Limit()-scope.Live()) {
				if failure = flush(); failure != nil {
					return false
				}
			}
			if !run.append(a, keyed, row, min(options.MemoryBudget/8, scope.Limit()-scope.Live()+run.bytes())) {
				failure = errSpillBudget
				return false
			}
		}
		return true
	}
	outcome := p.spillInput(ctx, a, options, at, &prefix, consume)
	if failure != nil {
		return spillExecutionError(ctx, failure), true
	}
	if outcome.code != ExecutionCompleted {
		return outcome, true
	}
	if err := flush(); err != nil {
		return spillExecutionError(ctx, err), true
	}
	if len(paths) == 0 {
		return ExecutionResult{code: ExecutionCompleted}, true
	}
	fanIn := int(max(int64(2), min(int64(16), options.MemoryBudget/(64<<10))))
	for len(paths) > fanIn {
		next := make([]string, 0, (len(paths)+fanIn-1)/fanIn)
		for first := 0; first < len(paths); first += fanIn {
			group := paths[first:min(first+fanIn, len(paths))]
			file, err := workspace.create()
			if err != nil {
				return spillExecutionError(ctx, err), true
			}
			writer := makeSpillStream(ctx, a, file, options.MemoryBudget)
			if writer == nil {
				file.Close()
				return spillExecutionError(ctx, errSpillBudget), true
			}
			result := mergeSpillRuns(ctx, a, options, group, extended, step, func(batch *store.Batch) bool {
				for row := range batch.Len() {
					if failure = writer.writeRow(batch, row); failure != nil {
						return false
					}
				}
				return true
			})
			if result.code == ExecutionCompleted {
				failure = writer.flush()
			}
			name := file.Name()
			writer.release()
			if failure != nil {
				return spillExecutionError(ctx, failure), true
			}
			if result.code != ExecutionCompleted {
				return result, true
			}
			next = append(next, name)
			for _, path := range group {
				if err := os.Remove(path); err != nil {
					return spillExecutionError(ctx, err), true
				}
			}
		}
		paths = next
	}
	tail := *p
	tail.steps = p.steps[at+1:]
	limits, offset := make([]int64, len(tail.steps)), make([]int64, len(tail.steps))
	for i, s := range tail.steps {
		limits[i], offset[i] = s.limit, s.offset
	}
	columns := make([]int, step.schema.Len())
	for i := range columns {
		columns[i] = i
	}
	limited := false
	result := mergeSpillRuns(ctx, a, options, paths, extended, step, func(batch *store.Batch) bool {
		output := batch.Project(columns)
		if output == nil {
			failure = errSpillBudget
			return false
		}
		outcome = tail.executeSteps(ctx, a, output, 0, limits, offset, nil, nil, nil, nil, nil, nil, options.MemoryBudget, sink)
		if outcome.code != ExecutionCompleted {
			return false
		}
		limited = streamingLimitDone(tail.steps, limits, 0)
		return !limited
	})
	if failure != nil {
		return spillExecutionError(ctx, failure), true
	}
	if outcome.code != ExecutionCompleted {
		return outcome, true
	}
	if limited && result.code == ExecutionStopped {
		result.code = ExecutionCompleted
	}
	if err := ctx.Err(); err != nil {
		return spillExecutionError(ctx, err), true
	}
	return result, true
}

func (p *PhysicalPlan) spillInput(ctx context.Context, a *mem.Allocator, options ExecutionOptions, at int, prefix *PhysicalPlan, sink func(*store.Batch) bool) ExecutionResult {
	source := *p.source
	source.filters = nil
	if source.parquetOptions.BatchSize == 0 || source.parquetOptions.BatchSize > spillBatchRows {
		source.parquetOptions.BatchSize = spillBatchRows
	}
	if source.csvOptions.BatchSize == 0 || source.csvOptions.BatchSize > spillBatchRows {
		source.csvOptions.BatchSize = spillBatchRows
	}
	cursor := scanCursor{source: &source}
	defer cursor.close()
	steps := make([]physicalStep, 0, len(source.filters)+at)
	for i, program := range p.source.filters {
		if i < len(p.source.parquetFilterHandled) && p.source.parquetFilterHandled[i] {
			continue
		}
		steps = append(steps, physicalStep{operation: physicalFilter, program: program})
	}
	steps = append(steps, p.steps[:at]...)
	prefix.steps = steps
	schema := p.source.schema
	if p.source.projection != nil {
		schema = schema.project(p.source.projection)
	}
	ids := a.AllocSegTemp(spillBatchRows * 8)
	if ids == nil {
		return spillExecutionError(ctx, errSpillBudget)
	}
	defer ids.Dec()
	for {
		if err := ctx.Err(); err != nil {
			return spillExecutionError(ctx, err)
		}
		batch, done, err := cursor.next(ctx, a)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, errSpillBudget) || errors.Is(err, errSpillFormat) {
				return spillExecutionError(ctx, err)
			}
			return scanExecutionError(err, nil)
		}
		if done {
			break
		}
		mask, bitmap := batch.Selection().AsBitMap()
		selection, selected := batch.Selection().AsSelVec()
		on, selAt := 0, 0
		gather := compactSortState{ctx: ctx, batches: []*store.Batch{batch}}
		result := ExecutionResult{code: ExecutionCompleted}
		send := func() bool {
			if on == 0 {
				return true
			}
			chunk := gather.gather(a, schema, ids.AsU64T()[:on])
			if chunk == nil {
				result = spillExecutionError(ctx, errSpillBudget)
				return false
			}
			on = 0
			result = prefix.executeSteps(ctx, a, chunk, 0, nil, nil, nil, nil, nil, nil, nil, nil, options.MemoryBudget, sink)
			return result.code == ExecutionCompleted
		}
		for row := range batch.Len() {
			if bitmap && !mask.IsSet(row) {
				continue
			}
			if selected {
				for selAt < len(selection.Offsets()) && int(selection.Offsets()[selAt]) < row {
					selAt++
				}
				if selAt == len(selection.Offsets()) || int(selection.Offsets()[selAt]) != row {
					continue
				}
			}
			ids.AsU64T()[on] = uint64(row)
			on++
			if on == spillBatchRows && !send() {
				break
			}
		}
		if result.code == ExecutionCompleted {
			send()
		}
		batch.Release()
		if result.code != ExecutionCompleted {
			return result
		}
	}
	return ExecutionResult{code: ExecutionCompleted}
}

func mergeSpillRuns(ctx context.Context, a *mem.Allocator, options ExecutionOptions, paths []string, schema Schema, step physicalStep, sink func(*store.Batch) bool) ExecutionResult {
	readers := make([]*spillStream, len(paths))
	heads := make([]bool, len(paths))
	defer func() {
		for _, reader := range readers {
			if reader != nil {
				reader.release()
			}
		}
	}()
	for i, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return spillExecutionError(ctx, err)
		}
		readers[i] = makeSpillStream(ctx, a, file, options.MemoryBudget)
		if readers[i] == nil {
			file.Close()
			return spillExecutionError(ctx, errSpillBudget)
		}
		heads[i], err = readers[i].nextRecord(a, schema, options.MemoryBudget)
		if err != nil {
			return spillExecutionError(ctx, err)
		}
	}
	output := packedRows{}
	defer output.release()
	emit := func() ExecutionResult {
		if output.length == 0 {
			return ExecutionResult{code: ExecutionCompleted}
		}
		batch := output.makeBatch(a, schema)
		if batch == nil {
			return spillExecutionError(ctx, errSpillBudget)
		}
		output.release()
		defer batch.Release()
		if !sink(batch) {
			return ExecutionResult{code: ExecutionStopped}
		}
		if err := ctx.Err(); err != nil {
			return spillExecutionError(ctx, err)
		}
		return ExecutionResult{code: ExecutionCompleted}
	}
	for {
		if err := ctx.Err(); err != nil {
			return spillExecutionError(ctx, err)
		}
		best := -1
		for i, ready := range heads {
			if !ready {
				continue
			}
			if best < 0 {
				best = i
				continue
			}
			less := false
			for k, order := range step.order {
				l, r := readers[i].recordValue(schema, step.schema.Len()+k), readers[best].recordValue(schema, step.schema.Len()+k)
				if l.IsNull() || r.IsNull() {
					if l.IsNull() != r.IsNull() {
						less = l.IsNull() == order.nullsFirst
						break
					}
					continue
				}
				cmp := compareOrdered(l, r)
				if cmp != 0 {
					if order.descending {
						less = cmp > 0
					} else {
						less = cmp < 0
					}
					break
				}
			}
			if less {
				best = i
			}
		}
		if best < 0 {
			return emit()
		}
		charge := int64(schema.Len() * 96)
		for column := range schema.Len() {
			charge = saturatingAdd(charge, int64(len(readers[best].recordValue(schema, column).text))*4)
		}
		if output.length != 0 && (output.length >= spillBatchRows || output.bytes()*3+charge > options.MemoryBudget/8) {
			if result := emit(); result.code != ExecutionCompleted {
				return result
			}
		}
		if !output.appendRecord(a, readers[best], schema, options.MemoryBudget/8) {
			return spillExecutionError(ctx, errSpillBudget)
		}
		var err error
		heads[best], err = readers[best].nextRecord(a, schema, options.MemoryBudget)
		if err != nil {
			return spillExecutionError(ctx, err)
		}
	}
}
