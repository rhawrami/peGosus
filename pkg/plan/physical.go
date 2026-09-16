package plan

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/op/cmpop"
	"github.com/rhawrami/peGosus/pkg/store"
)

type physicalOp uint8

const (
	physicalInvalid physicalOp = iota
	physicalFilter
	physicalProject
)

type physicalStep struct {
	operation physicalOp
	filter    physicalFilterData
	project   []int
}

type physicalFilterData struct {
	column      int
	dType       dtype.Type
	operation   exprOp
	literal     Scalar
	alwaysFalse bool
}

// MakePhysicalPlan binds and lowers a logical scan/filter/project plan.
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
		return nil, err
	}
	return &PhysicalPlan{source: source.Retain(), schema: schema, steps: steps}, nil
}

// PhysicalPlan is a bound, reusable push pipeline description. Release
// requires all Execute and Schema calls to have completed.
type PhysicalPlan struct {
	source *store.Table
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
	if p == nil || p.source == nil || a == nil || sink == nil {
		return false
	}
	for i := range p.source.NBatches() {
		batch := p.source.BatchAt(i).Retain()
		for _, step := range p.steps {
			switch step.operation {
			case physicalFilter:
				applyPhysicalFilter(a, batch, step.filter)
			case physicalProject:
				projected := batch.Project(step.project)
				batch.Release()
				batch = projected
			}
		}
		keepGoing := func() bool {
			defer batch.Release()
			return sink(batch)
		}()
		if !keepGoing {
			return false
		}
	}
	return true
}

// Release releases source ownership held by the physical plan.
func (p *PhysicalPlan) Release() {
	if p == nil || p.source == nil {
		return
	}
	p.source.Release()
	p.source = nil
	p.schema = Schema{}
	p.steps = nil
}

func lowerBoundNode(plan *BoundPlan, node *boundLogicalNode) (*store.Table, Schema, []physicalStep, *PlanError) {
	if node == nil {
		return nil, Schema{}, nil, makePlanError(ErrorInvalidPlan, "bound plan contains an empty node")
	}
	if node.operation == logicalScan {
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
		return node.table, node.schema, nil, nil
	}
	source, schema, steps, err := lowerBoundNode(plan, node.input)
	if err != nil {
		return nil, Schema{}, nil, err
	}

	switch node.operation {
	case logicalFilter:
		filter, err := lowerPhysicalFilter(plan, node.predicate, schema)
		if err != nil {
			return nil, Schema{}, nil, err
		}
		return source, node.schema, append(steps, physicalStep{operation: physicalFilter, filter: filter}), nil
	case logicalProject:
		offsets := make([]int, len(node.projections))
		for i, id := range node.projections {
			expression := plan.expression(id)
			if expression == nil || expression.kind != exprColumn {
				return nil, Schema{}, nil, makePlanError(ErrorUnsupportedOperation, "computed projection execution is not implemented")
			}
			offset := schema.offsetOf(expression.field)
			if offset < 0 {
				return nil, Schema{}, nil, makePlanError(ErrorInvalidPlan, "projected field is absent from its input")
			}
			offsets[i] = offset
		}
		return source, node.schema, append(steps, physicalStep{operation: physicalProject, project: offsets}), nil
	default:
		return nil, Schema{}, nil, makePlanError(ErrorUnsupportedOperation, "physical operation is not implemented")
	}
}

func lowerPhysicalFilter(plan *BoundPlan, id boundExprID, schema Schema) (physicalFilterData, *PlanError) {
	expression := plan.expression(id)
	if expression == nil || expression.kind != exprBinary || !isComparison(expression.operation) {
		return physicalFilterData{}, makePlanError(ErrorUnsupportedOperation, "physical filters currently require one comparison")
	}
	left, right := plan.expression(expression.children[0]), plan.expression(expression.children[1])
	if left == nil || right == nil {
		return physicalFilterData{}, makePlanError(ErrorInvalidPlan, "comparison contains an invalid child")
	}
	operation := expression.operation
	column := left
	literalID := expression.children[1]
	if column.kind != exprColumn {
		column = right
		literalID = expression.children[0]
		operation = reverseComparison(operation)
	}
	if column.kind != exprColumn {
		return physicalFilterData{}, makePlanError(ErrorUnsupportedOperation, "comparison execution requires one direct column operand")
	}
	offset := schema.offsetOf(column.field)
	if offset < 0 {
		return physicalFilterData{}, makePlanError(ErrorInvalidPlan, "comparison field is absent from its input")
	}
	literal, ok := boundScalarValue(plan, literalID)
	if !ok {
		return physicalFilterData{}, makePlanError(ErrorUnsupportedOperation, "comparison execution requires one literal operand")
	}
	if !literal.Type().Equal(column.dType) {
		return physicalFilterData{}, makePlanError(ErrorUnsupportedOperation, "comparison requires an unimplemented vector cast")
	}
	if !physicalComparisonSupported(column.dType) {
		return physicalFilterData{}, makePlanError(ErrorUnsupportedOperation, "comparison type has no physical kernel")
	}
	return physicalFilterData{
		column: offset, dType: column.dType, operation: operation, literal: literal, alwaysFalse: literal.IsNull(),
	}, nil
}

func boundScalarValue(plan *BoundPlan, id boundExprID) (Scalar, bool) {
	expression := plan.expression(id)
	if expression == nil {
		return Scalar{}, false
	}
	if expression.kind == exprLiteral {
		return expression.literal, true
	}
	if expression.kind != exprCast || expression.childCount != 1 {
		return Scalar{}, false
	}
	value, ok := boundScalarValue(plan, expression.children[0])
	if !ok {
		return Scalar{}, false
	}
	return value.cast(expression.target)
}

func reverseComparison(operation exprOp) exprOp {
	switch operation {
	case exprOpLT:
		return exprOpGT
	case exprOpLE:
		return exprOpGE
	case exprOpGT:
		return exprOpLT
	case exprOpGE:
		return exprOpLE
	default:
		return operation
	}
}

func physicalComparisonSupported(t dtype.Type) bool {
	switch t.ID() {
	case dtype.INT32T, dtype.INT64T, dtype.FLOAT32T, dtype.FLOAT64T, dtype.DATET, dtype.TIMESTAMPTZT:
		return true
	default:
		return false
	}
}

func applyPhysicalFilter(a *mem.Allocator, batch *store.Batch, filter physicalFilterData) {
	mask := store.MakeBitMapTemp(a, batch.Len())
	vector := batch.VectorAt(filter.column)
	if batch.Len() != 0 && !filter.alwaysFalse {
		evaluateComparison(vector, mask.Bytes(), filter)
	}
	mask.RecalcNiN()
	if vector.Validity() != nil {
		mask.ANDInPlaceViN(vector.Validity())
	}
	if batch.Selection() != nil {
		prior := batch.Selection().MakeBitMapTemp(a)
		mask.ANDInPlaceViN(prior)
		prior.Release()
	}
	batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
}

func evaluateComparison(vector *store.Vector, dst []byte, filter physicalFilterData) {
	switch filter.dType.ID() {
	case dtype.INT32T, dtype.DATET:
		evaluateI32Comparison(vector.I32s(), dst, filter.operation, filter.literal.i32())
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		evaluateI64Comparison(vector.I64s(), dst, filter.operation, filter.literal.i64())
	case dtype.FLOAT32T:
		evaluateF32Comparison(vector.F32s(), dst, filter.operation, filter.literal.f32())
	case dtype.FLOAT64T:
		evaluateF64Comparison(vector.F64s(), dst, filter.operation, filter.literal.f64())
	}
}

func evaluateI32Comparison(src []int32, dst []byte, operation exprOp, literal int32) {
	switch operation {
	case exprOpEQ:
		cmpop.EqI32(src, dst, literal)
	case exprOpNE:
		cmpop.NeqI32(src, dst, literal)
	case exprOpLT:
		cmpop.LtI32(src, dst, literal)
	case exprOpLE:
		cmpop.LeI32(src, dst, literal)
	case exprOpGT:
		cmpop.GtI32(src, dst, literal)
	case exprOpGE:
		cmpop.GeI32(src, dst, literal)
	}
}

func evaluateI64Comparison(src []int64, dst []byte, operation exprOp, literal int64) {
	switch operation {
	case exprOpEQ:
		cmpop.EqI64(src, dst, literal)
	case exprOpNE:
		cmpop.NeqI64(src, dst, literal)
	case exprOpLT:
		cmpop.LtI64(src, dst, literal)
	case exprOpLE:
		cmpop.LeI64(src, dst, literal)
	case exprOpGT:
		cmpop.GtI64(src, dst, literal)
	case exprOpGE:
		cmpop.GeI64(src, dst, literal)
	}
}

func evaluateF32Comparison(src []float32, dst []byte, operation exprOp, literal float32) {
	switch operation {
	case exprOpEQ:
		cmpop.EqF32(src, dst, literal)
	case exprOpNE:
		cmpop.NeqF32(src, dst, literal)
	case exprOpLT:
		cmpop.LtF32(src, dst, literal)
	case exprOpLE:
		cmpop.LeF32(src, dst, literal)
	case exprOpGT:
		cmpop.GtF32(src, dst, literal)
	case exprOpGE:
		cmpop.GeF32(src, dst, literal)
	}
}

func evaluateF64Comparison(src []float64, dst []byte, operation exprOp, literal float64) {
	switch operation {
	case exprOpEQ:
		cmpop.EqF64(src, dst, literal)
	case exprOpNE:
		cmpop.NeqF64(src, dst, literal)
	case exprOpLT:
		cmpop.LtF64(src, dst, literal)
	case exprOpLE:
		cmpop.LeF64(src, dst, literal)
	case exprOpGT:
		cmpop.GtF64(src, dst, literal)
	case exprOpGE:
		cmpop.GeF64(src, dst, literal)
	}
}
