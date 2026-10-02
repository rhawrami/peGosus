package plan

import (
	"context"
	"math"

	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type physicalOp uint8

const (
	physicalInvalid physicalOp = iota
	physicalFilter
	physicalProject
	physicalLimit
	physicalAggregate
	physicalDistinct
	physicalSort
	physicalJoin
	physicalPushedFilter
)

// MakePhysicalPlan binds and lowers a supported logical plan.
func MakePhysicalPlan(logical LogicalPlan) (*PhysicalPlan, error) {
	bound, err := BindLogicalPlan(logical)
	if err != nil {
		return nil, err
	}
	return MakePhysicalPlanFromBound(bound)
}

// MakePhysicalPlanFromBound lowers a bound logical plan.
func MakePhysicalPlanFromBound(bound *BoundPlan) (*PhysicalPlan, error) {
	if !bound.Valid() {
		return nil, makePlanError(ErrorInvalidPlan, "cannot lower an invalid bound plan")
	}
	source, schema, steps, err := lowerBoundNode(bound, bound.root)
	if err != nil {
		releaseJoinSteps(steps)
		return nil, err
	}
	steps = foldAggregateProjection(steps)
	pruneScanProjection(source, steps)
	pushScanFilters(source, steps)
	return &PhysicalPlan{source: source.Retain(), schema: schema, steps: steps}, nil
}

type physicalStep struct {
	operation  physicalOp
	program    physicalExprProgram
	limit      int64
	offset     int64
	aggregates []physicalAggregateExpr
	groupKeys  []physicalExprProgram
	schema     Schema
	order      []physicalOrderKey
	topN       int64
	hasTopN    bool
	join       *physicalJoinSpec
}

// PhysicalPlan is a bound, reusable batched pipeline description. Release
// requires all Execute and Schema calls to have completed.
type PhysicalPlan struct {
	source *scanSource
	schema Schema
	steps  []physicalStep
}

// Schema returns the physical output schema.
func (p *PhysicalPlan) Schema() Schema {
	if p == nil {
		return Schema{}
	}
	return p.schema
}

// Execute pushes each result batch synchronously into `sink`. Returning false
// from the sink stops execution. Sink batches are borrowed for the call and
// source-backed vector and validity payloads must remain immutable.
func (p *PhysicalPlan) Execute(a *mem.Allocator, sink func(*store.Batch) bool) bool {
	return p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{MemoryBudget: math.MaxInt64}, sink).Code() == ExecutionCompleted
}

// Release releases source ownership held by the physical plan.
func (p *PhysicalPlan) Release() {
	if p == nil || p.source == nil {
		return
	}
	p.source.Release()
	releaseJoinSteps(p.steps)
	p.source = nil
	p.schema = Schema{}
	p.steps = nil
}

func lowerBoundNode(plan *BoundPlan, node *boundLogicalNode) (*scanSource, Schema, []physicalStep, *PlanError) {
	if node == nil {
		return nil, Schema{}, nil, makePlanError(ErrorInvalidPlan, "bound plan contains an empty node")
	}
	if node.operation == logicalScan {
		if node.csvPath != "" {
			if !node.schema.Valid() {
				return nil, Schema{}, nil, makePlanError(ErrorInvalidPlan, "invalid CSV scan schema")
			}
			return &scanSource{csvPath: node.csvPath, csvOptions: node.csvOptions, schema: node.schema}, node.schema, nil, nil
		}
		if node.parquetPath != "" {
			return &scanSource{parquetPath: node.parquetPath, parquetOptions: node.parquetOptions, parquetMetadata: parquet.MakeParquetMetadataCache(), schema: node.schema}, node.schema, nil, nil
		}
		if !node.table.Valid() || node.table.NColumns() != node.schema.Len() {
			return nil, Schema{}, nil, makePlanError(ErrorInvalidPlan, "bound scan source is no longer valid")
		}
		for i := range node.schema.Len() {
			if !node.table.TypeAt(i).Equal(node.schema.FieldAt(i).Type()) {
				return nil, Schema{}, nil, makePlanError(ErrorTypeMismatch, "bound scan source types changed")
			}
			if node.table.NullableAt(i) && !node.schema.FieldAt(i).Nullable() {
				return nil, Schema{}, nil, makePlanError(ErrorTypeMismatch, "bound scan source nullability changed")
			}
		}
		return &scanSource{table: node.table, schema: node.schema}, node.schema, nil, nil
	}
	source, schema, steps, err := lowerBoundNode(plan, node.input)
	if err != nil {
		return nil, Schema{}, steps, err
	}

	switch node.operation {
	case logicalJoin:
		rightSource, rightSchema, rightSteps, err := lowerBoundNode(plan, node.right)
		if err != nil {
			releaseJoinSteps(rightSteps)
			return nil, Schema{}, steps, err
		}
		rightSteps = foldAggregateProjection(rightSteps)
		pruneScanProjection(rightSource, rightSteps)
		pushScanFilters(rightSource, rightSteps)
		spec := &physicalJoinSpec{right: &PhysicalPlan{source: rightSource.Retain(), schema: rightSchema, steps: rightSteps}, kind: node.joinKind, leftColumns: schema.Len(), rightColumns: rightSchema.Len()}
		for i := range node.leftKeys {
			leftProgram, err := makePhysicalExprProgram(plan, []boundExprID{node.leftKeys[i]}, schema, false)
			if err != nil {
				spec.right.Release()
				return nil, Schema{}, steps, err
			}
			rightProgram, err := makePhysicalExprProgram(plan, []boundExprID{node.rightKeys[i]}, rightSchema, false)
			if err != nil {
				spec.right.Release()
				return nil, Schema{}, steps, err
			}
			spec.leftKeys = append(spec.leftKeys, leftProgram)
			spec.rightKeys = append(spec.rightKeys, rightProgram)
		}
		if node.residual != 0 {
			mergedFields := append(append([]Field(nil), schema.fields...), rightSchema.fields...)
			program, err := makePhysicalExprProgram(plan, []boundExprID{node.residual}, Schema{valid: true, fields: mergedFields}, false)
			if err != nil {
				spec.right.Release()
				return nil, Schema{}, steps, err
			}
			spec.residual = &program
		}
		return source, node.schema, append(steps, physicalStep{operation: physicalJoin, join: spec, schema: node.schema}), nil
	case logicalAlias:
		return source, node.schema, steps, nil
	case logicalSort:
		order := make([]physicalOrderKey, len(node.order))
		for i, key := range node.order {
			program, err := makePhysicalExprProgram(plan, []boundExprID{key.input}, schema, false)
			if err != nil {
				return nil, Schema{}, steps, err
			}
			order[i] = physicalOrderKey{program: program, descending: key.descending, nullsFirst: key.nullsFirst}
		}
		return source, node.schema, append(steps, physicalStep{operation: physicalSort, order: order, schema: node.schema}), nil
	case logicalDistinct:
		return source, node.schema, append(steps, physicalStep{operation: physicalDistinct, schema: node.schema}), nil
	case logicalAggregate:
		keys := make([]physicalExprProgram, len(node.groupKeys))
		for i, id := range node.groupKeys {
			program, err := makePhysicalExprProgram(plan, []boundExprID{id}, schema, false)
			if err != nil {
				return nil, Schema{}, steps, err
			}
			keys[i] = program
		}
		aggregates := make([]physicalAggregateExpr, len(node.aggregates))
		for i, aggregate := range node.aggregates {
			aggregates[i].kind = aggregate.kind
			aggregates[i].distinct = aggregate.distinct
			if aggregate.kind != AggregateCountStar {
				program, err := makePhysicalExprProgram(plan, []boundExprID{aggregate.input}, schema, false)
				if err != nil {
					return nil, Schema{}, steps, err
				}
				aggregates[i].program = program
			}
		}
		return source, node.schema, append(steps, physicalStep{operation: physicalAggregate, groupKeys: keys, aggregates: aggregates, schema: node.schema}), nil
	case logicalLimit:
		if len(steps) != 0 && steps[len(steps)-1].operation == physicalSort && node.offset <= math.MaxInt64-node.limit {
			steps[len(steps)-1].topN = node.offset + node.limit
			steps[len(steps)-1].hasTopN = true
		}
		return source, node.schema, append(steps, physicalStep{operation: physicalLimit, limit: node.limit, offset: node.offset}), nil
	case logicalFilter:
		program, err := makePhysicalExprProgram(plan, []boundExprID{node.predicate}, schema, false)
		if err != nil {
			return nil, Schema{}, steps, err
		}
		return source, node.schema, append(steps, physicalStep{operation: physicalFilter, program: program}), nil
	case logicalProject:
		program, err := makePhysicalExprProgram(plan, node.projections, schema, true)
		if err != nil {
			return nil, Schema{}, steps, err
		}
		return source, node.schema, append(steps, physicalStep{operation: physicalProject, program: program}), nil
	default:
		return nil, Schema{}, steps, makePlanError(ErrorUnsupportedOperation, "physical operation is not implemented")
	}
}

func releaseJoinSteps(steps []physicalStep) {
	for _, step := range steps {
		if step.join != nil {
			step.join.right.Release()
		}
	}
}
