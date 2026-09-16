package plan

import (
	"fmt"

	"github.com/rhawrami/peGosus/pkg/dtype"
)

// BindLogicalPlan resolves names and types in an unresolved logical plan.
func BindLogicalPlan(logical LogicalPlan) (*BoundPlan, error) {
	if !logical.Valid() {
		return nil, makePlanError(ErrorInvalidPlan, "cannot bind an invalid logical plan")
	}
	b := binder{expressions: []boundExpr{{}}}
	root, err := b.bindLogicalNode(logical.root)
	if err != nil {
		return nil, err
	}
	return &BoundPlan{root: root, expressions: b.expressions}, nil
}

type binder struct {
	expressions []boundExpr
}

func (b *binder) bindLogicalNode(node *logicalNode) (*boundLogicalNode, *PlanError) {
	if node == nil {
		return nil, makePlanError(ErrorInvalidPlan, "logical plan contains an empty node")
	}
	if node.operation == logicalScan {
		if !node.table.Valid() || !node.schema.Valid() || node.table.NColumns() != node.schema.Len() {
			return nil, makePlanError(ErrorInvalidPlan, "scan source and schema do not match")
		}
		for i := range node.schema.Len() {
			if !node.table.TypeAt(i).Equal(node.schema.FieldAt(i).Type()) {
				return nil, makePlanError(ErrorTypeMismatch, "scan source and schema types do not match")
			}
			if node.table.NullableAt(i) && !node.schema.FieldAt(i).Nullable() {
				return nil, makePlanError(ErrorTypeMismatch, "scan schema understates source nullability")
			}
		}
		return &boundLogicalNode{operation: logicalScan, table: node.table, schema: node.schema}, nil
	}

	input, err := b.bindLogicalNode(node.input)
	if err != nil {
		return nil, err
	}
	switch node.operation {
	case logicalFilter:
		predicate, err := b.bindExpr(node.predicate.node, input.schema, dtype.BoolT())
		if err != nil {
			return nil, err
		}
		if !b.expressions[predicate].dType.Equal(dtype.BoolT()) {
			return nil, makePlanError(ErrorTypeMismatch, "filter predicate must have BOOL type")
		}
		return &boundLogicalNode{operation: logicalFilter, input: input, schema: input.schema, predicate: predicate}, nil
	case logicalProject:
		projections := make([]boundExprID, len(node.projections))
		fields := make([]Field, len(node.projections))
		for i, expression := range node.projections {
			id, err := b.bindExpr(expression.node, input.schema, dtype.Type{})
			if err != nil {
				return nil, err
			}
			bound := b.expressions[id]
			name := expression.alias
			if name == "" {
				name = fmt.Sprintf("expr_%d", i+1)
				if bound.kind == exprColumn {
					if offset := input.schema.offsetOf(bound.field); offset >= 0 {
						name = input.schema.FieldAt(offset).Name()
					}
				}
			}
			projections[i] = id
			fields[i] = Field{id: makeFieldID(), name: name, dType: bound.dType, nullable: bound.nullable}
		}
		return &boundLogicalNode{
			operation: logicalProject, input: input, schema: Schema{valid: true, fields: fields}, projections: projections,
		}, nil
	default:
		return nil, makePlanError(ErrorInvalidPlan, "logical plan contains an unknown operation")
	}
}

func (b *binder) bindExpr(node *exprNode, schema Schema, expected dtype.Type) (boundExprID, *PlanError) {
	if node == nil || node.kind == exprInvalid {
		return 0, makePlanError(ErrorInvalidExpression, "expression is invalid")
	}
	switch node.kind {
	case exprColumn:
		if node.column == "" {
			return 0, makePlanError(ErrorInvalidExpression, "column name is empty")
		}
		offset, code := schema.resolve(node.column)
		if code != 0 {
			return 0, makePlanError(code, fmt.Sprintf("cannot resolve column %q", node.column))
		}
		field := schema.FieldAt(offset)
		return b.add(boundExpr{kind: exprColumn, dType: field.Type(), nullable: field.Nullable(), field: field.ID()}), nil
	case exprLiteral:
		value, err := node.literal.bind(expected)
		if err != nil {
			return 0, err
		}
		return b.add(boundExpr{kind: exprLiteral, dType: value.Type(), nullable: value.IsNull(), literal: value}), nil
	case exprCast:
		expected := dtype.Type{}
		if needsTypeContext(node.children[0]) {
			expected = node.cast
		}
		child, err := b.bindExpr(node.children[0], schema, expected)
		if err != nil {
			return 0, err
		}
		return b.addCast(child, node.cast, false)
	case exprUnary:
		return b.bindUnary(node, schema)
	case exprBinary:
		return b.bindBinary(node, schema, expected)
	case exprTernary:
		return b.bindTernary(node, schema, expected)
	default:
		return 0, makePlanError(ErrorInvalidExpression, "expression has an unknown kind")
	}
}

func (b *binder) bindUnary(node *exprNode, schema Schema) (boundExprID, *PlanError) {
	expected := dtype.Type{}
	if (node.operation == exprOpIsNull || node.operation == exprOpIsNotNull) && needsTypeContext(node.children[0]) {
		expected = dtype.BoolT()
	}
	child, err := b.bindExpr(node.children[0], schema, expected)
	if err != nil {
		return 0, err
	}
	input := b.expressions[child]
	result := boundExpr{kind: exprUnary, operation: node.operation, children: [3]boundExprID{child}, childCount: 1}
	switch node.operation {
	case exprOpAbs, exprOpNeg:
		if !input.dType.IsNumericType() {
			return 0, makePlanError(ErrorTypeMismatch, "numeric unary expression requires a numeric input")
		}
		result.dType, result.nullable = input.dType, input.nullable
	case exprOpSqrt:
		if !input.dType.IsNumericType() {
			return 0, makePlanError(ErrorTypeMismatch, "Sqrt requires a numeric input")
		}
		if input.dType.ID() == dtype.INT32T || input.dType.ID() == dtype.FLOAT32T {
			result.dType = dtype.Float32T()
		} else {
			result.dType = dtype.Float64T()
		}
		result.nullable = input.nullable
	case exprOpNot:
		if input.dType.ID() != dtype.BOOLT {
			return 0, makePlanError(ErrorTypeMismatch, "NOT requires a BOOL input")
		}
		result.dType, result.nullable = dtype.BoolT(), input.nullable
	case exprOpIsNull, exprOpIsNotNull:
		result.dType, result.nullable = dtype.BoolT(), false
	default:
		return 0, makePlanError(ErrorInvalidExpression, "unsupported unary expression")
	}
	return b.add(result), nil
}

func (b *binder) bindBinary(node *exprNode, schema Schema, expected dtype.Type) (boundExprID, *PlanError) {
	if node.operation == exprOpCoalesce {
		return b.bindCoalesce(node, schema, expected)
	}
	operands, err := b.bindOperands(node.children[:2], schema, dtype.Type{})
	if err != nil {
		return 0, err
	}
	left, right := operands[0], operands[1]
	lhs, rhs := b.expressions[left], b.expressions[right]
	result := boundExpr{kind: exprBinary, operation: node.operation, children: [3]boundExprID{left, right}, childCount: 2}

	if isArithmetic(node.operation) {
		if node.operation == exprOpSub && lhs.dType.ID() == dtype.DATET && rhs.dType.ID() == dtype.DATET {
			result.dType = dtype.Int32T()
			result.nullable = lhs.nullable || rhs.nullable
			return b.add(result), nil
		}
		common, ok := dtype.CommonNumericType(lhs.dType, rhs.dType)
		if !ok {
			return 0, makePlanError(ErrorTypeMismatch, "arithmetic requires numeric operands")
		}
		left, err = b.addCast(left, common, true)
		if err != nil {
			return 0, err
		}
		right, err = b.addCast(right, common, true)
		if err != nil {
			return 0, err
		}
		result.children = [3]boundExprID{left, right}
		result.dType = common
		if node.operation == exprOpDiv && common.IsIntegral() {
			if common.ID() == dtype.INT32T {
				result.dType = dtype.Float32T()
			} else {
				result.dType = dtype.Float64T()
			}
		}
		result.nullable = b.expressions[left].nullable || b.expressions[right].nullable
		return b.add(result), nil
	}

	if isComparison(node.operation) {
		left, right, err = b.coerceComparable(left, right, node.operation)
		if err != nil {
			return 0, err
		}
		result.children = [3]boundExprID{left, right}
		result.dType = dtype.BoolT()
		result.nullable = b.expressions[left].nullable || b.expressions[right].nullable
		return b.add(result), nil
	}

	if node.operation == exprOpAnd || node.operation == exprOpOr {
		if lhs.dType.ID() != dtype.BOOLT || rhs.dType.ID() != dtype.BOOLT {
			return 0, makePlanError(ErrorTypeMismatch, "boolean logic requires BOOL operands")
		}
		result.dType = dtype.BoolT()
		result.nullable = lhs.nullable || rhs.nullable
		return b.add(result), nil
	}
	return 0, makePlanError(ErrorInvalidExpression, "unsupported binary expression")
}

func (b *binder) bindCoalesce(node *exprNode, schema Schema, expected dtype.Type) (boundExprID, *PlanError) {
	nodes := make([]*exprNode, 0, 2)
	var collect func(*exprNode)
	collect = func(current *exprNode) {
		if current != nil && current.kind == exprBinary && current.operation == exprOpCoalesce {
			collect(current.children[0])
			collect(current.children[1])
			return
		}
		nodes = append(nodes, current)
	}
	collect(node)
	operands, err := b.bindOperands(nodes, schema, expected)
	if err != nil {
		return 0, err
	}
	common := b.expressions[operands[0]].dType
	for _, operand := range operands[1:] {
		var ok bool
		common, ok = commonValueType(common, b.expressions[operand].dType)
		if !ok {
			return 0, makePlanError(ErrorTypeMismatch, "Coalesce operands have incompatible types")
		}
	}
	for i, operand := range operands {
		operands[i], err = b.addCast(operand, common, true)
		if err != nil {
			return 0, err
		}
	}
	result := operands[0]
	for _, operand := range operands[1:] {
		result = b.add(boundExpr{
			kind: exprBinary, operation: exprOpCoalesce, dType: common,
			nullable: b.expressions[result].nullable && b.expressions[operand].nullable,
			children: [3]boundExprID{result, operand}, childCount: 2,
		})
	}
	return result, nil
}

func (b *binder) bindTernary(node *exprNode, schema Schema, expected dtype.Type) (boundExprID, *PlanError) {
	if node.operation == exprOpCase {
		condition, err := b.bindExpr(node.children[0], schema, dtype.BoolT())
		if err != nil {
			return 0, err
		}
		if b.expressions[condition].dType.ID() != dtype.BOOLT {
			return 0, makePlanError(ErrorTypeMismatch, "Case condition must have BOOL type")
		}
		branches, err := b.bindOperands(node.children[1:3], schema, expected)
		if err != nil {
			return 0, err
		}
		whenTrue, whenFalse := branches[0], branches[1]
		common, ok := commonValueType(b.expressions[whenTrue].dType, b.expressions[whenFalse].dType)
		if !ok {
			return 0, makePlanError(ErrorTypeMismatch, "Case branches have incompatible types")
		}
		whenTrue, err = b.addCast(whenTrue, common, true)
		if err != nil {
			return 0, err
		}
		whenFalse, err = b.addCast(whenFalse, common, true)
		if err != nil {
			return 0, err
		}
		return b.add(boundExpr{
			kind: exprTernary, operation: exprOpCase, dType: common,
			nullable: b.expressions[whenTrue].nullable || b.expressions[whenFalse].nullable,
			children: [3]boundExprID{condition, whenTrue, whenFalse}, childCount: 3,
		}), nil
	}
	operands, err := b.bindOperands(node.children[:], schema, dtype.Type{})
	if err != nil {
		return 0, err
	}
	first, second, third := operands[0], operands[1], operands[2]
	common, ok := commonTernaryType(b.expressions[first].dType, b.expressions[second].dType, b.expressions[third].dType)
	if !ok {
		return 0, makePlanError(ErrorTypeMismatch, "ternary expression operands have incompatible types")
	}
	first, err = b.addCast(first, common, true)
	if err != nil {
		return 0, err
	}
	second, err = b.addCast(second, common, true)
	if err != nil {
		return 0, err
	}
	third, err = b.addCast(third, common, true)
	if err != nil {
		return 0, err
	}
	result := boundExpr{
		kind: exprTernary, operation: node.operation, children: [3]boundExprID{first, second, third}, childCount: 3,
	}
	if node.operation == exprOpBetween || node.operation == exprOpNotBetween {
		if !common.CanOrder() {
			return 0, makePlanError(ErrorTypeMismatch, "Between requires orderable operands")
		}
		result.dType = dtype.BoolT()
	} else if node.operation == exprOpClip {
		if !common.IsNumericType() {
			return 0, makePlanError(ErrorTypeMismatch, "Clip requires numeric operands")
		}
		result.dType = common
	} else {
		return 0, makePlanError(ErrorInvalidExpression, "unsupported ternary expression")
	}
	result.nullable = b.expressions[first].nullable || b.expressions[second].nullable || b.expressions[third].nullable
	return b.add(result), nil
}

func (b *binder) bindOperands(nodes []*exprNode, schema Schema, expected dtype.Type) ([]boundExprID, *PlanError) {
	operands := make([]boundExprID, len(nodes))
	target := dtype.Type{}
	for i, node := range nodes {
		if isUntypedLiteral(node) || needsTypeContext(node) {
			continue
		}
		operand, err := b.bindExpr(node, schema, dtype.Type{})
		if err != nil {
			return nil, err
		}
		operands[i] = operand
		operandType := b.expressions[operand].dType
		if !target.Valid() {
			target = operandType
		} else if target.IsNumericType() && operandType.IsNumericType() {
			target, _ = dtype.CommonNumericType(target, operandType)
		}
	}
	if !target.Valid() {
		target = expected
	}
	if !target.Valid() {
		for _, node := range nodes {
			if node != nil && node.kind == exprLiteral && node.literal.kind == literalUntypedInteger {
				target = dtype.Int64T()
				break
			}
		}
	}
	if !target.Valid() {
		return nil, makePlanError(ErrorTypeMismatch, "cannot infer a type for NULL operands")
	}
	for i, node := range nodes {
		if !isUntypedLiteral(node) && !needsTypeContext(node) {
			continue
		}
		operand, err := b.bindExpr(node, schema, target)
		if err != nil && node.kind == exprLiteral && node.literal.kind == literalUntypedInteger {
			operand, err = b.bindExpr(node, schema, dtype.Type{})
		}
		if err != nil {
			return nil, err
		}
		operands[i] = operand
	}
	return operands, nil
}

func (b *binder) coerceComparable(left, right boundExprID, operation exprOp) (boundExprID, boundExprID, *PlanError) {
	lhs, rhs := b.expressions[left], b.expressions[right]
	if lhs.dType.IsNumericType() && rhs.dType.IsNumericType() {
		common, _ := dtype.CommonNumericType(lhs.dType, rhs.dType)
		var err *PlanError
		left, err = b.addCast(left, common, true)
		if err != nil {
			return 0, 0, err
		}
		right, err = b.addCast(right, common, true)
		return left, right, err
	}
	if !lhs.dType.Equal(rhs.dType) {
		return 0, 0, makePlanError(ErrorTypeMismatch, "comparison operands have incompatible types")
	}
	if operation == exprOpEQ || operation == exprOpNE {
		if !lhs.dType.CanCompareEquality() {
			return 0, 0, makePlanError(ErrorTypeMismatch, "operands do not support equality comparison")
		}
		return left, right, nil
	}
	if !lhs.dType.CanOrder() {
		return 0, 0, makePlanError(ErrorTypeMismatch, "operands do not support ordered comparison")
	}
	return left, right, nil
}

func (b *binder) addCast(child boundExprID, target dtype.Type, implicit bool) (boundExprID, *PlanError) {
	input := b.expressions[child]
	if input.dType.Equal(target) {
		return child, nil
	}
	if !input.dType.IsNumericType() || !target.IsNumericType() {
		return 0, makePlanError(ErrorImpossibleCoercion, "cannot cast "+input.dType.String()+" to "+target.String())
	}
	if implicit {
		common, ok := dtype.CommonNumericType(input.dType, target)
		if !ok || !common.Equal(target) {
			return 0, makePlanError(ErrorImpossibleCoercion, "implicit cast would narrow "+input.dType.String()+" to "+target.String())
		}
	}
	nullable := input.nullable
	if input.dType.IsFloating() && target.IsIntegral() {
		nullable = true
	}
	return b.add(boundExpr{
		kind: exprCast, dType: target, nullable: nullable, children: [3]boundExprID{child}, childCount: 1, target: target,
		implicit: implicit,
	}), nil
}

func (b *binder) add(expression boundExpr) boundExprID {
	id := boundExprID(len(b.expressions))
	b.expressions = append(b.expressions, expression)
	return id
}

func isUntypedLiteral(node *exprNode) bool {
	return node != nil && node.kind == exprLiteral &&
		(node.literal.kind == literalUntypedInteger || node.literal.kind == literalUntypedNull)
}

func needsTypeContext(node *exprNode) bool {
	if node == nil {
		return false
	}
	if node.kind == exprLiteral {
		return node.literal.kind == literalUntypedNull
	}
	if node.kind == exprBinary && node.operation == exprOpCoalesce {
		return needsTypeContext(node.children[0]) && needsTypeContext(node.children[1])
	}
	if node.kind == exprTernary && node.operation == exprOpCase {
		return needsTypeContext(node.children[1]) && needsTypeContext(node.children[2])
	}
	return false
}

func isArithmetic(operation exprOp) bool {
	return operation >= exprOpAdd && operation <= exprOpDiv
}

func isComparison(operation exprOp) bool {
	return operation >= exprOpEQ && operation <= exprOpGE
}

func commonTernaryType(first, second, third dtype.Type) (dtype.Type, bool) {
	if first.IsNumericType() && second.IsNumericType() && third.IsNumericType() {
		common, _ := dtype.CommonNumericType(first, second)
		return dtype.CommonNumericType(common, third)
	}
	return first, first.Equal(second) && first.Equal(third)
}

func commonValueType(left, right dtype.Type) (dtype.Type, bool) {
	if left.IsNumericType() && right.IsNumericType() {
		return dtype.CommonNumericType(left, right)
	}
	return left, left.Equal(right)
}
