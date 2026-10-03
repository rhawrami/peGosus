package plan

import (
	"context"
	"fmt"
	"os"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type spillAggregateLane struct {
	indices  []int
	distinct bool
	column   int
	schema   Schema
	path     string
}

func spillColumnProgram(column int, field Field) physicalExprProgram {
	return physicalExprProgram{nodes: []physicalExprNode{{kind: exprColumn, column: column, dType: field.Type(), nullable: field.Nullable()}}, roots: []int{0}, materialize: []bool{false}}
}

func (p *PhysicalPlan) executeSpilledGroup(ctx context.Context, a *mem.Allocator, options ExecutionOptions, sink func(*store.Batch) bool, scope *mem.AllocationScope) (ExecutionResult, bool) {
	at := -1
	for i, step := range p.steps {
		if step.operation == physicalFilter || step.operation == physicalPushedFilter || step.operation == physicalProject {
			continue
		}
		if step.operation == physicalDistinct && step.schema.Len() > 0 {
			local := *p
			local.steps = append([]physicalStep(nil), p.steps...)
			grouped := physicalStep{operation: physicalAggregate, schema: step.schema}
			for col, field := range step.schema.fields {
				grouped.groupKeys = append(grouped.groupKeys, spillColumnProgram(col, field))
			}
			local.steps[i] = grouped
			return local.executeSpilledGroup(ctx, a, options, sink, scope)
		}
		if step.operation == physicalAggregate {
			at = i
		}
		break
	}
	if at < 0 {
		return ExecutionResult{}, false
	}
	if scope == nil {
		scope = mem.MakeAllocationScope(a, options.MemoryBudget)
	}
	a = mem.MakeAllocatorWithScope(a, scope)
	directory, err := os.MkdirTemp(options.SpillDirectory, "peg-group-")
	if err != nil {
		return spillExecutionError(ctx, err), true
	}
	defer os.RemoveAll(directory)
	workspace := spillWorkspace{directory: directory}
	step := p.steps[at]
	keys := len(step.groupKeys)
	input := Schema{valid: true, fields: append([]Field(nil), step.schema.fields[:keys]...)}
	columns := make([]int, len(step.aggregates))
	programs := append([]physicalExprProgram(nil), step.groupKeys...)
	for i, aggregate := range step.aggregates {
		columns[i] = -1
		if aggregate.kind == AggregateCountStar {
			continue
		}
		if len(aggregate.program.roots) != 1 {
			return ExecutionResult{}, false
		}
		columns[i] = len(programs)
		programs = append(programs, aggregate.program)
		root := aggregate.program.nodes[aggregate.program.roots[0]]
		input.fields = append(input.fields, Field{dType: root.dType, nullable: root.nullable})
	}
	ordinal := input.Len()
	input.fields = append(input.fields, Field{dType: dtype.Int64T()})
	file, err := workspace.create()
	if err != nil {
		return spillExecutionError(ctx, err), true
	}
	writer := makeSpillStream(ctx, a, file, options.MemoryBudget)
	if writer == nil {
		file.Close()
		return spillExecutionError(ctx, errSpillBudget), true
	}
	var failure error
	var sequence int64
	prefix := *p
	outcome := p.spillInput(ctx, a, options, at, &prefix, func(batch *store.Batch) bool {
		values := make([]physicalExprValues, len(programs))
		vectors := make([]store.Vector, input.Len())
		defer func() {
			for i := range values {
				values[i].release()
			}
			for i := range vectors {
				vectors[i].Release()
			}
		}()
		for i, program := range programs {
			var ok bool
			values[i], ok = executePhysicalExprProgram(a, batch, program)
			if !ok {
				failure = errSpillBudget
				return false
			}
			vectors[i] = values[i].vectors[program.roots[0]].Retain()
		}
		vectors[ordinal] = store.MakeVector(a, batch.Len(), dtype.Int64T(), false)
		if vectors[ordinal].Kind() == store.VectorInvalid {
			failure = errSpillBudget
			return false
		}
		for row := range batch.Len() {
			vectors[ordinal].I64s()[row] = sequence
			sequence++
		}
		computed := store.MakeBatch(vectors)
		if computed == nil {
			failure = errSpillBudget
			return false
		}
		vectors = nil
		defer computed.Release()
		mask, bitmap := batch.Selection().AsBitMap()
		selected, offsets := batch.Selection().AsSelVec()
		cursor := 0
		for row := range batch.Len() {
			if bitmap && !mask.IsSet(row) {
				continue
			}
			if offsets {
				for cursor < len(selected.Offsets()) && int(selected.Offsets()[cursor]) < row {
					cursor++
				}
				if cursor == len(selected.Offsets()) || int(selected.Offsets()[cursor]) != row {
					continue
				}
			}
			if failure = writer.writeRow(computed, row); failure != nil {
				return false
			}
		}
		return true
	})
	if failure == nil && outcome.code == ExecutionCompleted {
		failure = writer.flush()
	}
	path := file.Name()
	writer.release()
	if failure != nil {
		return spillExecutionError(ctx, fmt.Errorf("spill aggregate input: %w", failure)), true
	}
	if outcome.code != ExecutionCompleted {
		if outcome.cause != nil {
			outcome.cause = fmt.Errorf("spill aggregate prefix: %w", outcome.cause)
		}
		return outcome, true
	}
	configuration := makeGroupState(step, scope)
	uniqueSources := configuration.uniqueSources
	configuration.release()
	lanes := []spillAggregateLane{{column: -1}}
	for i, aggregate := range step.aggregates {
		if !aggregate.distinct || aggregate.kind == AggregateMin || aggregate.kind == AggregateMax {
			lanes[0].indices = append(lanes[0].indices, i)
			continue
		}
		source := uniqueSources[i]
		found := -1
		for j := 1; j < len(lanes); j++ {
			if lanes[j].column == columns[source] {
				found = j
				break
			}
		}
		if found < 0 {
			lanes = append(lanes, spillAggregateLane{distinct: true, column: columns[source]})
			found = len(lanes) - 1
		}
		lanes[found].indices = append(lanes[found].indices, i)
	}
	for i := range lanes {
		lane := &lanes[i]
		lane.schema = Schema{valid: true, fields: append([]Field(nil), step.schema.fields[:keys]...)}
		for _, index := range lane.indices {
			lane.schema.fields = append(lane.schema.fields, step.schema.FieldAt(keys+index))
		}
		if len(lane.indices) == 0 {
			lane.schema.fields = append(lane.schema.fields, Field{dType: dtype.Int64T()})
		}
		if failure = p.spillAggregateLane(ctx, a, scope, options, &workspace, path, input, ordinal, step, columns, lane); failure != nil {
			return spillExecutionError(ctx, fmt.Errorf("spill aggregate lane %d: %w", i, failure)), true
		}
	}
	merged := lanes[0]
	positions := make([]int, len(step.aggregates))
	for i := range positions {
		positions[i] = -1
	}
	for col, index := range merged.indices {
		positions[index] = keys + col
	}
	for i := 1; i < len(lanes); i++ {
		next := lanes[i]
		combined := Schema{valid: true, fields: append([]Field(nil), merged.schema.fields...)}
		combined.fields = append(combined.fields, next.schema.fields[keys:]...)
		joined, err := mergeSpillAggregateLanes(ctx, a, options, &workspace, merged, next, keys, combined)
		if err != nil {
			return spillExecutionError(ctx, err), true
		}
		for col, index := range next.indices {
			positions[index] = merged.schema.Len() + col
		}
		os.Remove(merged.path)
		os.Remove(next.path)
		merged.schema, merged.path = combined, joined
	}
	source := &scanSource{spillPaths: []string{merged.path}, spillBudget: options.MemoryBudget, schema: merged.schema}
	projected := physicalExprProgram{}
	for col := range keys {
		projected.nodes = append(projected.nodes, spillColumnProgram(col, merged.schema.FieldAt(col)).nodes[0])
		projected.roots = append(projected.roots, col)
		projected.materialize = append(projected.materialize, false)
	}
	for _, col := range positions {
		projected.roots = append(projected.roots, len(projected.nodes))
		projected.nodes = append(projected.nodes, spillColumnProgram(col, merged.schema.FieldAt(col)).nodes[0])
		projected.materialize = append(projected.materialize, false)
	}
	tail := PhysicalPlan{source: source, schema: p.schema, steps: append([]physicalStep{{operation: physicalProject, program: projected, schema: step.schema}}, p.steps[at+1:]...)}
	result := tail.executeWithScope(ctx, a, options, sink, scope)
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}, true
	}
	return result, true
}

func (p *PhysicalPlan) spillAggregateLane(ctx context.Context, a *mem.Allocator, scope *mem.AllocationScope, options ExecutionOptions, workspace *spillWorkspace, path string, input Schema, ordinal int, step physicalStep, columns []int, lane *spillAggregateLane) error {
	keys := len(step.groupKeys)
	source := &scanSource{spillPaths: []string{path}, spillBudget: options.MemoryBudget, schema: input}
	order := make([]physicalOrderKey, keys)
	for i := range keys {
		order[i] = physicalOrderKey{program: spillColumnProgram(i, input.FieldAt(i))}
	}
	if lane.distinct {
		file, err := workspace.create()
		if err != nil {
			return err
		}
		writer := makeSpillStream(ctx, a, file, options.MemoryBudget)
		if writer == nil {
			file.Close()
			return errSpillBudget
		}
		distinctOrder := append(append([]physicalOrderKey(nil), order...), physicalOrderKey{program: spillColumnProgram(lane.column, input.FieldAt(lane.column))})
		sortPlan := PhysicalPlan{source: source, schema: input, steps: []physicalStep{{operation: physicalSort, schema: input, order: distinctOrder}}}
		prior := packedRows{}
		priorValues := make([]Scalar, keys+1)
		var failure error
		result := sortPlan.executeWithScope(ctx, a, options, func(batch *store.Batch) bool {
			for row := range batch.Len() {
				equal := prior.length != 0
				if equal {
					for col := range keys {
						if !spillScalarEqual(prior.at(0, col), scalarAt(batch.VectorAt(col), row)) {
							equal = false
							break
						}
					}
				}
				if equal {
					equal = spillScalarEqual(prior.at(0, keys), scalarAt(batch.VectorAt(lane.column), row))
				}
				if equal {
					continue
				}
				prior.length, prior.stringBytes = 0, 0
				for col := range keys {
					priorValues[col] = scalarAt(batch.VectorAt(col), row)
				}
				priorValues[keys] = scalarAt(batch.VectorAt(lane.column), row)
				ok := prior.appendScalars(a, priorValues, options.MemoryBudget/8)
				clear(priorValues)
				if !ok {
					failure = errSpillBudget
					return false
				}
				if failure = writer.writeRow(batch, row); failure != nil {
					return false
				}
			}
			return true
		}, scope)
		prior.release()
		if failure == nil && result.code == ExecutionCompleted {
			failure = writer.flush()
		}
		dedupPath := file.Name()
		writer.release()
		if failure != nil {
			return failure
		}
		if result.code != ExecutionCompleted {
			if result.cause != nil {
				return result.cause
			}
			return errSpillBudget
		}
		defer os.Remove(dedupPath)
		source = &scanSource{spillPaths: []string{dedupPath}, spillBudget: options.MemoryBudget, schema: input}
		order = append(order, physicalOrderKey{program: spillColumnProgram(ordinal, input.FieldAt(ordinal))})
	}
	file, err := workspace.create()
	if err != nil {
		return err
	}
	writer := makeSpillStream(ctx, a, file, options.MemoryBudget)
	if writer == nil {
		file.Close()
		return errSpillBudget
	}
	defer writer.release()
	lane.path = file.Name()
	aggregates := make([]physicalAggregateExpr, len(lane.indices))
	for i, index := range lane.indices {
		aggregates[i] = step.aggregates[index]
		aggregates[i].distinct = false
		if columns[index] >= 0 {
			aggregates[i].program = spillColumnProgram(columns[index], input.FieldAt(columns[index]))
		}
	}
	if len(aggregates) == 0 {
		aggregates = []physicalAggregateExpr{{kind: AggregateCountStar}}
	}
	states := make([]aggregateValue, len(aggregates))
	group := packedRows{}
	groupValues := make([]Scalar, keys)
	haveGroup := false
	defer func() {
		for i := range states {
			states[i].release()
		}
		group.release()
	}()
	reduction := physicalStep{operation: physicalAggregate, aggregates: aggregates, schema: Schema{valid: true, fields: append([]Field(nil), lane.schema.fields[keys:]...)}}
	emit := func() error {
		values := finalizeAggregates(a, reduction, states)
		if values == nil {
			return errSpillBudget
		}
		defer values.Release()
		return writer.writeAggregateRow(&group, values)
	}
	var failure error
	consume := func(batch *store.Batch) bool {
		for row := range batch.Len() {
			equal := haveGroup
			if equal {
				for col := range keys {
					if !spillScalarEqual(group.at(0, col), scalarAt(batch.VectorAt(col), row)) {
						equal = false
						break
					}
				}
			}
			if !equal {
				if haveGroup {
					if failure = emit(); failure != nil {
						return false
					}
				}
				for i := range states {
					states[i].release()
					states[i] = aggregateValue{}
				}
				group.length, group.stringBytes = 0, 0
				if keys != 0 {
					for col := range keys {
						groupValues[col] = scalarAt(batch.VectorAt(col), row)
					}
					ok := group.appendScalars(a, groupValues, options.MemoryBudget/8)
					clear(groupValues)
					if !ok {
						failure = errSpillBudget
						return false
					}
				}
				haveGroup = true
			}
			for i, aggregate := range aggregates {
				value := Scalar{}
				if aggregate.kind != AggregateCountStar {
					value = scalarAt(batch.VectorAt(aggregate.program.nodes[0].column), row)
				}
				if !states[i].add(a, aggregate.kind, value) {
					failure = errSpillBudget
					return false
				}
			}
		}
		return true
	}
	sortPlan := PhysicalPlan{source: source, schema: input, steps: []physicalStep{{operation: physicalSort, schema: input, order: order}}}
	var result ExecutionResult
	if len(order) == 0 {
		sortPlan.steps = nil
		child := options
		child.Workers = 1
		child.SpillDirectory = ""
		result = sortPlan.executeWithScope(ctx, a, child, consume, scope)
	} else {
		result = sortPlan.executeWithScope(ctx, a, options, consume, scope)
	}
	if failure != nil {
		return failure
	}
	if result.code != ExecutionCompleted {
		if result.cause != nil {
			return result.cause
		}
		return fmt.Errorf("spill lane execution code %d: %w", result.code, errSpillBudget)
	}
	if haveGroup || keys == 0 {
		if err := emit(); err != nil {
			return err
		}
	}
	return writer.flush()
}

func spillScalarEqual(left, right Scalar) bool {
	if left.IsNull() || right.IsNull() {
		return left.IsNull() == right.IsNull()
	}
	if left.Type().ID() == dtype.BOOLT {
		return left.bits == right.bits
	}
	return compareOrdered(left, right) == 0
}

func mergeSpillAggregateLanes(ctx context.Context, a *mem.Allocator, options ExecutionOptions, workspace *spillWorkspace, left, right spillAggregateLane, keys int, schema Schema) (string, error) {
	leftFile, err := os.Open(left.path)
	if err != nil {
		return "", err
	}
	leftReader := makeSpillStream(ctx, a, leftFile, options.MemoryBudget)
	if leftReader == nil {
		leftFile.Close()
		return "", errSpillBudget
	}
	defer leftReader.release()
	rightFile, err := os.Open(right.path)
	if err != nil {
		return "", err
	}
	rightReader := makeSpillStream(ctx, a, rightFile, options.MemoryBudget)
	if rightReader == nil {
		rightFile.Close()
		return "", errSpillBudget
	}
	defer rightReader.release()
	file, err := workspace.create()
	if err != nil {
		return "", err
	}
	writer := makeSpillStream(ctx, a, file, options.MemoryBudget)
	if writer == nil {
		file.Close()
		return "", errSpillBudget
	}
	defer writer.release()
	for {
		l, le := leftReader.nextRecord(a, left.schema, options.MemoryBudget)
		r, re := rightReader.nextRecord(a, right.schema, options.MemoryBudget)
		if le != nil {
			return "", le
		}
		if re != nil {
			return "", re
		}
		if !l && !r {
			break
		}
		if l != r {
			return "", errSpillFormat
		}
		for col := range keys {
			if !spillScalarEqual(leftReader.recordValue(left.schema, col), rightReader.recordValue(right.schema, col)) {
				return "", errSpillFormat
			}
		}
		if err := writer.writeJoinedRecords(leftReader, rightReader, keys); err != nil {
			return "", err
		}
	}
	if err := writer.flush(); err != nil {
		return "", err
	}
	return file.Name(), nil
}
