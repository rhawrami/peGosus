package plan

import (
	"context"
	"hash/maphash"
	"io"
	"os"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

const spillJoinPartitions = 8

type spillJoinPartition struct {
	path string
	rows int64
}

type spillJoinFlags struct {
	file   *os.File
	buffer *mem.Segment
	offset int64
	length int
	dirty  bool
}

func (f *spillJoinFlags) flush() error {
	if !f.dirty {
		return nil
	}
	n, err := f.file.WriteAt(f.buffer.AsBytes()[:f.length], f.offset)
	if err == nil && n != f.length {
		err = io.ErrShortWrite
	}
	if err == nil {
		f.dirty = false
	}
	return err
}

func (f *spillJoinFlags) at(row int64, set bool) (bool, error) {
	if f.length == 0 || row < f.offset || row >= f.offset+int64(f.length) {
		if err := f.flush(); err != nil {
			return false, err
		}
		f.offset = row / int64(f.buffer.Len()) * int64(f.buffer.Len())
		n, err := f.file.ReadAt(f.buffer.AsBytes(), f.offset)
		if err != nil && err != io.EOF {
			return false, err
		}
		f.length = n
		if n == 0 {
			return false, io.ErrUnexpectedEOF
		}
	}
	at := int(row - f.offset)
	matched := f.buffer.AsBytes()[at] != 0
	if set {
		f.buffer.AsBytes()[at] = 1
		f.dirty = true
	}
	return matched, nil
}

func (p *PhysicalPlan) executeSpilledJoin(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool, scope *mem.AllocationScope) (ExecutionResult, bool) {
	at := -1
	aggregateTail := false
	for i, step := range p.steps {
		if at < 0 {
			if step.operation == physicalJoin {
				at = i
				continue
			}
			if step.operation != physicalFilter && step.operation != physicalPushedFilter && step.operation != physicalProject {
				return ExecutionResult{}, false
			}
		} else {
			if step.operation == physicalAggregate && i == len(p.steps)-1 {
				for _, aggregate := range step.aggregates {
					if aggregate.distinct {
						return ExecutionResult{}, false
					}
				}
				aggregateTail = true
				continue
			}
			if step.operation != physicalFilter && step.operation != physicalPushedFilter && step.operation != physicalProject {
				return ExecutionResult{}, false
			}
		}
	}
	if at < 0 {
		return ExecutionResult{}, false
	}
	step := p.steps[at]
	right := step.join.right
	for _, step := range right.steps {
		if step.operation != physicalFilter && step.operation != physicalPushedFilter && step.operation != physicalProject {
			return ExecutionResult{}, false
		}
	}
	if err := ctx.Err(); err != nil {
		return spillExecutionError(ctx, err), true
	}
	if scope == nil {
		scope = mem.MakeAllocationScope(a, options.MemoryBudget)
	}
	a = mem.MakeAllocatorWithScope(a, scope)
	directory, err := os.MkdirTemp(options.SpillDirectory, "peg-join-")
	if err != nil {
		return spillExecutionError(ctx, err), true
	}
	defer os.RemoveAll(directory)
	workspace := spillWorkspace{directory: directory}
	seed := maphash.MakeSeed()
	leftParts, result := p.partitionSpillJoin(ctx, a, options, at, step.join.leftKeys, workspace, seed)
	if result.code != ExecutionCompleted {
		return result, true
	}
	rightParts, result := right.partitionSpillJoin(ctx, a, options, len(right.steps), step.join.rightKeys, workspace, seed)
	if result.code != ExecutionCompleted {
		return result, true
	}
	leftSchema := Schema{valid: true, fields: append([]Field(nil), step.schema.fields[:step.join.leftColumns]...)}
	tail := *p
	tail.steps = p.steps[at+1:]
	var output *spillStream
	var outputPath string
	if aggregateTail {
		file, err := workspace.create()
		if err != nil {
			return spillExecutionError(ctx, err), true
		}
		outputPath = file.Name()
		output = makeSpillStream(ctx, a, file, options.MemoryBudget)
		if output == nil {
			file.Close()
			return spillExecutionError(ctx, errSpillBudget), true
		}
		defer output.release()
	}
	emit := func(batch *store.Batch) ExecutionResult {
		if output == nil {
			return tail.executeSteps(ctx, a, batch, 0, nil, nil, nil, nil, nil, nil, nil, nil, options.MemoryBudget, sink)
		}
		defer batch.Release()
		mask, bitmap := batch.Selection().AsBitMap()
		selection, selected := batch.Selection().AsSelVec()
		cursor := 0
		for row := range batch.Len() {
			if bitmap && !mask.IsSet(row) {
				continue
			}
			if selected {
				for cursor < len(selection.Offsets()) && int(selection.Offsets()[cursor]) < row {
					cursor++
				}
				if cursor == len(selection.Offsets()) || int(selection.Offsets()[cursor]) != row {
					continue
				}
			}
			if err := output.writeRow(batch, row); err != nil {
				return spillExecutionError(ctx, err)
			}
		}
		return ExecutionResult{code: ExecutionCompleted}
	}
	for partition := range spillJoinPartitions {
		result := p.executeSpillJoinPartition(ctx, a, scope, options, step, leftSchema, leftParts[partition], rightParts[partition], workspace, emit)
		if result.code != ExecutionCompleted {
			return result, true
		}
	}
	if err := ctx.Err(); err != nil {
		return spillExecutionError(ctx, err), true
	}
	if output == nil {
		return ExecutionResult{code: ExecutionCompleted}, true
	}
	if err := output.flush(); err != nil {
		return spillExecutionError(ctx, err), true
	}
	output.release()
	tail.source = &scanSource{schema: step.schema, spillPaths: []string{outputPath}, spillBudget: options.MemoryBudget}
	options.Workers = 1
	return tail.executeWithScope(ctx, a, options, sink, scope), true
}

func (p *PhysicalPlan) partitionSpillJoin(ctx context.Context, a *mem.Allocator, options ExecutionOptions, at int, programs []physicalExprProgram, workspace spillWorkspace, seed maphash.Seed) ([]spillJoinPartition, ExecutionResult) {
	parts := make([]spillJoinPartition, spillJoinPartitions)
	writers := make([]*spillStream, spillJoinPartitions)
	defer func() {
		for _, writer := range writers {
			if writer != nil {
				writer.release()
			}
		}
	}()
	for i := range writers {
		file, err := workspace.create()
		if err != nil {
			return nil, spillExecutionError(ctx, err)
		}
		parts[i].path = file.Name()
		writers[i] = makeSpillStream(ctx, a, file, options.MemoryBudget)
		if writers[i] == nil {
			file.Close()
			return nil, spillExecutionError(ctx, errSpillBudget)
		}
	}
	var scratch distinctState
	defer scratch.release()
	var failure error
	prefix := *p
	result := p.spillInput(ctx, a, options, at, &prefix, func(batch *store.Batch) bool {
		keyBatch, values := makeJoinKeyBatch(a, batch, programs)
		if keyBatch == nil {
			failure = errSpillBudget
			return false
		}
		defer keyBatch.Release()
		defer func() {
			for i := range values {
				values[i].release()
			}
		}()
		mask, bitmap := batch.Selection().AsBitMap()
		selection, selected := batch.Selection().AsSelVec()
		cursor := 0
		for row := range batch.Len() {
			if bitmap && !mask.IsSet(row) {
				continue
			}
			if selected {
				for cursor < len(selection.Offsets()) && int(selection.Offsets()[cursor]) < row {
					cursor++
				}
				if cursor == len(selection.Offsets()) || int(selection.Offsets()[cursor]) != row {
					continue
				}
			}
			encoded := scratch.keyBuffer(a, encodedRowKeyLength(keyBatch, row), options.MemoryBudget/8)
			if encoded == nil {
				failure = errSpillBudget
				return false
			}
			on := encodeRowKey(encoded, keyBatch, row)
			partition := int(maphash.Bytes(seed, encoded[:on]) & (spillJoinPartitions - 1))
			if failure = writers[partition].writeRow(batch, row); failure != nil {
				return false
			}
			parts[partition].rows++
		}
		return true
	})
	if failure != nil {
		return nil, spillExecutionError(ctx, failure)
	}
	if result.code != ExecutionCompleted {
		return nil, result
	}
	for _, writer := range writers {
		if err := writer.flush(); err != nil {
			return nil, spillExecutionError(ctx, err)
		}
	}
	return parts, ExecutionResult{code: ExecutionCompleted}
}

func (p *PhysicalPlan) executeSpillJoinPartition(ctx context.Context, a *mem.Allocator, scope *mem.AllocationScope, options ExecutionOptions, step physicalStep, leftSchema Schema, left, right spillJoinPartition, workspace spillWorkspace, emit func(*store.Batch) ExecutionResult) ExecutionResult {
	flagsFile, err := workspace.create()
	if err != nil {
		return spillExecutionError(ctx, err)
	}
	defer flagsFile.Close()
	if err := flagsFile.Truncate(left.rows); err != nil {
		return spillExecutionError(ctx, err)
	}
	flags := spillJoinFlags{file: flagsFile, buffer: a.AllocSegTemp(int(min(int64(4096), max(int64(256), options.MemoryBudget/128))))}
	if flags.buffer == nil {
		return spillExecutionError(ctx, errSpillBudget)
	}
	defer flags.buffer.Dec()
	rightFile, err := os.Open(right.path)
	if err != nil {
		return spillExecutionError(ctx, err)
	}
	reader := makeSpillStream(ctx, a, rightFile, options.MemoryBudget)
	if reader == nil {
		rightFile.Close()
		return spillExecutionError(ctx, errSpillBudget)
	}
	defer reader.release()
	run := packedRows{scope: scope}
	defer run.release()
	var pending *store.Batch
	defer func() {
		if pending != nil {
			pending.Release()
		}
	}()
	eof := false
	for !eof || pending != nil {
		for run.length < 512 && !eof {
			row := pending
			pending = nil
			if row == nil {
				row, err = reader.readRow(a, step.join.right.schema, options.MemoryBudget)
				if err == io.EOF {
					eof = true
					break
				}
				if err != nil {
					return spillExecutionError(ctx, err)
				}
			}
			needed := int64(row.NVectors() * 48)
			for column := range row.NVectors() {
				needed = saturatingAdd(needed, int64(len(scalarAt(row.VectorAt(column), 0).text))*4)
			}
			if run.length != 0 && run.bytes()*2+needed > options.MemoryBudget/32 {
				pending = row
				break
			}
			if !run.append(a, row, 0, options.MemoryBudget/16) {
				row.Release()
				return spillExecutionError(ctx, errSpillBudget)
			}
			row.Release()
		}
		if run.length == 0 {
			break
		}
		batch := run.makeBatch(a, step.join.right.schema)
		if batch == nil {
			return spillExecutionError(ctx, errSpillBudget)
		}
		run.release()
		run.scope = scope
		table := store.MakeTable([]*store.Batch{batch})
		batch.Release()
		if table == nil {
			return spillExecutionError(ctx, errSpillBudget)
		}
		chunkPlan := PhysicalPlan{source: &scanSource{table: table, schema: step.join.right.schema}, schema: step.join.right.schema}
		spec := *step.join
		spec.right = &chunkPlan
		spec.kind = JoinFull
		build := joinState{scope: scope}
		build.keys.setScope(scope)
		result := build.build(ctx, a, &spec, options.MemoryBudget/2)
		table.Release()
		if result.code != ExecutionCompleted {
			build.release()
			return result
		}
		probe := step
		probeSpec := *step.join
		probeSpec.kind = JoinInner
		probe.join = &probeSpec
		probe.schema = Schema{valid: true, fields: append(append([]Field(nil), leftSchema.fields...), step.join.right.schema.fields...)}
		result = p.scanSpillJoinLeft(ctx, a, options, left, leftSchema, &flags, func(row *store.Batch, index int64, matched bool) ExecutionResult {
			if matched && (step.join.kind == JoinSemi || step.join.kind == JoinAnti) {
				return ExecutionResult{code: ExecutionCompleted}
			}
			return build.probe(ctx, a, row, probe, options.MemoryBudget, func(batch *store.Batch) ExecutionResult {
				if _, err := flags.at(index, true); err != nil {
					batch.Release()
					return spillExecutionError(ctx, err)
				}
				if step.join.kind == JoinSemi || step.join.kind == JoinAnti {
					batch.Release()
					return ExecutionResult{code: ExecutionCompleted}
				}
				return emit(batch)
			})
		})
		if result.code == ExecutionCompleted && step.join.kind == JoinFull {
			result = build.finish(ctx, a, step, options.MemoryBudget, emit)
		}
		build.release()
		if result.code != ExecutionCompleted {
			return result
		}
	}
	if step.join.kind == JoinInner {
		return ExecutionResult{code: ExecutionCompleted}
	}
	return p.scanSpillJoinLeft(ctx, a, options, left, leftSchema, &flags, func(row *store.Batch, index int64, matched bool) ExecutionResult {
		pass := !matched
		if step.join.kind == JoinSemi {
			pass = matched
		}
		if !pass {
			return ExecutionResult{code: ExecutionCompleted}
		}
		if step.join.kind == JoinSemi || step.join.kind == JoinAnti {
			return emit(row.Retain())
		}
		var empty joinState
		batch := empty.gather(ctx, a, row, step.schema, step.join.leftColumns, []uint64{0, joinNullRow}, options.MemoryBudget)
		if batch == nil {
			return spillExecutionError(ctx, errSpillBudget)
		}
		return emit(batch)
	})
}

func (p *PhysicalPlan) scanSpillJoinLeft(ctx context.Context, a *mem.Allocator, options ExecutionOptions, part spillJoinPartition, schema Schema, flags *spillJoinFlags, consume func(*store.Batch, int64, bool) ExecutionResult) ExecutionResult {
	file, err := os.Open(part.path)
	if err != nil {
		return spillExecutionError(ctx, err)
	}
	reader := makeSpillStream(ctx, a, file, options.MemoryBudget)
	if reader == nil {
		file.Close()
		return spillExecutionError(ctx, errSpillBudget)
	}
	defer reader.release()
	for index := int64(0); index < part.rows; index++ {
		row, err := reader.readRow(a, schema, options.MemoryBudget)
		if err != nil {
			return spillExecutionError(ctx, err)
		}
		matched, err := flags.at(index, false)
		if err != nil {
			row.Release()
			return spillExecutionError(ctx, err)
		}
		result := consume(row, index, matched)
		row.Release()
		if result.code != ExecutionCompleted {
			return result
		}
	}
	return ExecutionResult{code: ExecutionCompleted}
}
