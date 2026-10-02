package plan

import (
	"math"
	"runtime"
	"strings"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/op/boolop"
	"github.com/rhawrami/peGosus/pkg/op/cmpop"
	"github.com/rhawrami/peGosus/pkg/op/numop"
	"github.com/rhawrami/peGosus/pkg/store"
)

type physicalExprNode struct {
	kind       exprKind
	operation  exprOp
	dType      dtype.Type
	nullable   bool
	square     bool
	clipI64F64 bool
	children   [3]int
	childCount uint8
	column     int
	literal    Scalar
}

type physicalExprProgram struct {
	nodes       []physicalExprNode
	roots       []int
	materialize []bool
	outputRoots bool
}

type physicalExprValues struct {
	vectors []store.Vector
	set     []bool
}

func makePhysicalExprProgram(plan *BoundPlan, roots []boundExprID, schema Schema, outputRoots bool) (physicalExprProgram, *PlanError) {
	program := physicalExprProgram{roots: make([]int, len(roots)), outputRoots: outputRoots}
	compiled := make(map[boundExprID]int)
	var compile func(boundExprID) (int, *PlanError)
	compile = func(id boundExprID) (int, *PlanError) {
		if index, ok := compiled[id]; ok {
			return index, nil
		}
		expression := plan.expression(id)
		if expression == nil {
			return 0, makePlanError(ErrorInvalidPlan, "physical expression contains an invalid node")
		}
		if scalar, ok := boundScalarValue(plan, id); ok {
			index := len(program.nodes)
			program.nodes = append(program.nodes, physicalExprNode{
				kind: exprLiteral, dType: scalar.Type(), nullable: scalar.IsNull(), literal: scalar,
			})
			compiled[id] = index
			return index, nil
		}
		if err := validatePhysicalExpression(expression); err != nil {
			return 0, err
		}
		node := physicalExprNode{
			kind: expression.kind, operation: expression.operation, dType: expression.dType,
			nullable: expression.nullable, childCount: expression.childCount, literal: expression.literal,
		}
		if expression.kind == exprColumn {
			node.column = schema.offsetOf(expression.field)
			if node.column < 0 {
				return 0, makePlanError(ErrorInvalidPlan, "physical expression field is absent from its input")
			}
		}
		for i := range int(expression.childCount) {
			child, err := compile(expression.children[i])
			if err != nil {
				return 0, err
			}
			node.children[i] = child
		}
		if node.kind == exprBinary && node.operation == exprOpMul && node.childCount == 2 {
			left := plan.expression(expression.children[0])
			right := plan.expression(expression.children[1])
			node.square = expression.square || left.kind == exprColumn && right.kind == exprColumn && left.field == right.field
		}
		if node.kind == exprTernary && node.operation == exprOpClip && node.dType.ID() == dtype.FLOAT64T && program.nodes[node.children[0]].kind == exprCast &&
			program.nodes[node.children[1]].kind == exprLiteral && program.nodes[node.children[2]].kind == exprLiteral &&
			!math.IsNaN(program.nodes[node.children[1]].literal.f64()) && !math.IsNaN(program.nodes[node.children[2]].literal.f64()) {
			cast := plan.expression(expression.children[0])
			if cast.implicit && plan.expression(cast.children[0]).dType.ID() == dtype.INT64T && program.nodes[program.nodes[node.children[0]].children[0]].kind != exprLiteral {
				node.clipI64F64 = true
			}
		}
		index := len(program.nodes)
		program.nodes = append(program.nodes, node)
		compiled[id] = index
		return index, nil
	}
	for i, root := range roots {
		index, err := compile(root)
		if err != nil {
			return physicalExprProgram{}, err
		}
		program.roots[i] = index
	}
	program.materialize = make([]bool, len(program.nodes))
	for _, root := range program.roots {
		program.materialize[root] = true
	}
	for _, node := range program.nodes {
		if node.square {
			if program.nodes[node.children[0]].kind == exprLiteral {
				program.materialize[node.children[0]] = true
			}
			continue
		}
		literalChildren := 0
		for i := range int(node.childCount) {
			if program.nodes[node.children[i]].kind == exprLiteral {
				literalChildren++
			}
		}
		useScalar := literalChildren == 1 && physicalBinaryScalarSupported(node, program.nodes)
		if node.kind == exprTernary && (node.operation == exprOpBetween || node.operation == exprOpNotBetween || node.operation == exprOpClip) {
			if program.nodes[node.children[0]].kind == exprLiteral {
				program.materialize[node.children[0]] = true
			}
			continue
		}
		if node.kind == exprTernary && (node.operation == exprOpReplace || node.operation == exprOpSlice) || node.kind == exprBinary && node.operation == exprOpConcat {
			if program.nodes[node.children[0]].kind == exprLiteral {
				program.materialize[node.children[0]] = true
			}
			continue
		}
		if useScalar {
			continue
		}
		for i := range int(node.childCount) {
			if program.nodes[node.children[i]].kind == exprLiteral {
				program.materialize[node.children[i]] = true
			}
		}
	}
	return program, nil
}

func physicalBinaryScalarSupported(node physicalExprNode, nodes []physicalExprNode) bool {
	if node.kind != exprBinary {
		return false
	}
	if node.operation == exprOpLike || node.operation == exprOpContains {
		return true
	}
	if isArithmetic(node.operation) {
		left, right := nodes[node.children[0]].dType, nodes[node.children[1]].dType
		return (left.IsNumericType() && right.IsNumericType()) ||
			(node.operation == exprOpSub && left.ID() == dtype.DATET && right.ID() == dtype.DATET)
	}
	if isComparison(node.operation) {
		left := nodes[node.children[0]].dType.ID()
		return left == dtype.INT32T || left == dtype.INT64T || left == dtype.FLOAT32T || left == dtype.FLOAT64T ||
			left == dtype.DATET || left == dtype.TIMESTAMPTZT || left == dtype.STRT || left == dtype.BOOLT
	}
	return false
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

func validatePhysicalExpression(expression *boundExpr) *PlanError {
	switch expression.kind {
	case exprColumn, exprLiteral:
		return nil
	case exprCast:
		if expression.dType.IsNumericType() {
			return nil
		}
	case exprUnary:
		switch expression.operation {
		case exprOpAbs, exprOpNeg, exprOpSqrt, exprOpNot, exprOpIsNull, exprOpIsNotNull, exprOpUpper, exprOpLower, exprOpExtractYear, exprOpExtractMonth, exprOpExtractDay, exprOpTruncateYear, exprOpTruncateMonth:
			return nil
		}
	case exprBinary:
		if isArithmetic(expression.operation) || isComparison(expression.operation) ||
			expression.operation == exprOpAnd || expression.operation == exprOpOr || expression.operation == exprOpCoalesce || expression.operation == exprOpConcat || expression.operation == exprOpLike || expression.operation == exprOpContains {
			return nil
		}
	case exprTernary:
		switch expression.operation {
		case exprOpBetween, exprOpNotBetween, exprOpClip, exprOpCase, exprOpReplace, exprOpSlice:
			return nil
		}
	}
	return makePlanError(ErrorUnsupportedOperation, "physical expression is not implemented")
}

func executePhysicalProject(a *mem.Allocator, batch *store.Batch, program physicalExprProgram) *store.Batch {
	values, ok := executePhysicalExprProgram(a, batch, program)
	if !ok {
		return nil
	}
	defer values.release()
	vectors := make([]store.Vector, len(program.roots))
	for i, root := range program.roots {
		vectors[i] = values.vectors[root].Retain()
		if vectors[i].Kind() == store.VectorInvalid {
			for j := range i {
				vectors[j].Release()
			}
			return nil
		}
	}
	projected := store.MakeBatch(vectors)
	if projected == nil {
		for i := range vectors {
			vectors[i].Release()
		}
		return nil
	}
	if !projected.SetSelection(batch.Selection().Retain()) {
		projected.Release()
		return nil
	}
	return projected
}

func executePhysicalFilter(a *mem.Allocator, batch *store.Batch, program physicalExprProgram) bool {
	values, ok := executePhysicalExprProgram(a, batch, program)
	if !ok || len(program.roots) != 1 {
		values.release()
		return false
	}
	defer values.release()
	predicate := &values.vectors[program.roots[0]]
	if predicate.TypeID() != dtype.BOOLT {
		return false
	}
	var mask *store.BitMap
	if predicate.Data().RefCount() == 1 {
		predicate.Data().Inc()
		mask = store.MakeBitMapWithUnknownNiN(batch.Len(), predicate.Data())
	} else {
		mask = store.MakeBitMap(a, batch.Len())
		if mask != nil {
			copy(mask.Bytes(), predicate.Bools())
			mask.RecalcNiN()
		}
	}
	if mask == nil {
		return false
	}
	if predicate.Validity() != nil {
		mask.ANDInPlaceViN(predicate.Validity())
	}
	if batch.Selection() != nil {
		prior := batch.Selection().RetainBitMap(a)
		if prior == nil {
			mask.Release()
			return false
		}
		mask.ANDInPlaceViN(prior)
		prior.Release()
	}
	return batch.SetSelection(store.MakeRowSelectionFromBitMap(mask))
}

func executePhysicalExprProgram(a *mem.Allocator, batch *store.Batch, program physicalExprProgram) (physicalExprValues, bool) {
	values := physicalExprValues{vectors: make([]store.Vector, len(program.nodes)), set: make([]bool, len(program.nodes))}
	for _, index := range program.roots {
		if !program.evaluate(a, batch, &values, index, program.outputRoots) {
			values.release()
			return physicalExprValues{}, false
		}
	}
	return values, true
}

func (p physicalExprProgram) evaluate(a *mem.Allocator, batch *store.Batch, values *physicalExprValues, index int, general bool) bool {
	if values.set[index] {
		return true
	}
	node := p.nodes[index]
	if node.kind != exprTernary || node.operation != exprOpCase {
		for i := range int(node.childCount) {
			child := node.children[i]
			if node.square && i == 1 {
				continue
			}
			if node.clipI64F64 && i == 0 {
				child = p.nodes[child].children[0]
			}
			if node.kind == exprTernary && i != 0 && (node.operation == exprOpBetween || node.operation == exprOpNotBetween || node.operation == exprOpClip) && p.nodes[child].kind == exprLiteral {
				continue
			}
			if p.nodes[child].kind == exprLiteral && !p.materialize[child] {
				continue
			}
			if !p.evaluate(a, batch, values, child, false) {
				return false
			}
		}
	} else if !p.evaluate(a, batch, values, node.children[0], false) {
		return false
	}
	var vector store.Vector
	switch {
	case node.kind == exprColumn:
		vector = batch.VectorAt(node.column).Retain()
	case node.kind == exprLiteral:
		vector = materializeScalar(a, batch.Len(), node.literal, general)
	case node.kind == exprTernary && node.operation == exprOpCase:
		vector = p.evaluateCase(a, batch, values, node, general)
	case (node.kind == exprUnary && (node.operation == exprOpUpper || node.operation == exprOpLower)) || (node.kind == exprBinary && node.operation == exprOpConcat) || (node.kind == exprTernary && (node.operation == exprOpReplace || node.operation == exprOpSlice)):
		vector = evaluateStringMapping(a, node, p.nodes, values.vectors, general)
	case node.kind == exprBinary && node.operation == exprOpCoalesce && node.dType.ID() == dtype.STRT:
		vector = evaluateStringCoalesce(a, &values.vectors[node.children[0]], &values.vectors[node.children[1]], general, node.nullable)
	default:
		if general {
			vector = store.MakeVector(a, batch.Len(), node.dType, node.nullable)
		} else {
			vector = store.MakeVectorTemp(a, batch.Len(), node.dType, node.nullable)
		}
		if vector.Kind() != store.VectorInvalid && !evaluatePhysicalExprNode(node, p.nodes, values.vectors, &vector, a) {
			vector.Release()
			return false
		}
	}
	if vector.Kind() == store.VectorInvalid {
		return false
	}
	values.vectors[index] = vector
	values.set[index] = true
	return true
}

func (p physicalExprProgram) evaluateCase(a *mem.Allocator, batch *store.Batch, values *physicalExprValues, node physicalExprNode, general bool) store.Vector {
	condition := &values.vectors[node.children[0]]
	rows := a.AllocSegTemp(batch.Len() * 8)
	if rows == nil {
		return store.Vector{}
	}
	defer rows.Dec()
	indices := rows.AsI64T()[:batch.Len()]
	var result store.Vector
	var strings [][]byte
	var stringValidity []bool
	var stringBranches [2]store.Vector
	defer func() {
		for i := range stringBranches {
			stringBranches[i].Release()
		}
	}()
	if node.dType.ID() == dtype.STRT {
		strings = make([][]byte, batch.Len())
		if node.nullable {
			stringValidity = make([]bool, batch.Len())
		}
	} else if general {
		result = store.MakeVector(a, batch.Len(), node.dType, node.nullable)
	} else {
		result = store.MakeVectorTemp(a, batch.Len(), node.dType, node.nullable)
	}
	if node.dType.ID() != dtype.STRT && result.Kind() == store.VectorInvalid {
		return result
	}
	if result.Validity() != nil {
		result.Validity().ClearAll()
	}
	for branch := 1; branch <= 2; branch++ {
		count := 0
		for row := range batch.Len() {
			trueRow := (condition.Validity() == nil || condition.Validity().IsSet(row)) && condition.Bools()[row>>3]&(1<<(row&7)) != 0
			if trueRow == (branch == 1) {
				indices[count] = int64(row)
				count++
			}
		}
		if count == 0 {
			continue
		}
		subset := gatherCaseBatch(a, batch, indices[:count])
		if subset == nil {
			result.Release()
			return store.Vector{}
		}
		branchValues := physicalExprValues{vectors: make([]store.Vector, len(p.nodes)), set: make([]bool, len(p.nodes))}
		ok := p.evaluate(a, subset, &branchValues, node.children[branch], false)
		if ok {
			src := &branchValues.vectors[node.children[branch]]
			for i, rowIndex := range indices[:count] {
				row := int(rowIndex)
				if src.Validity() != nil && !src.Validity().IsSet(i) {
					continue
				}
				if result.Validity() != nil {
					result.Validity().Set(row)
				}
				switch node.dType.ID() {
				case dtype.INT32T, dtype.DATET:
					result.I32s()[row] = src.I32s()[i]
				case dtype.INT64T, dtype.TIMESTAMPTZT:
					result.I64s()[row] = src.I64s()[i]
				case dtype.FLOAT32T:
					result.F32s()[row] = src.F32s()[i]
				case dtype.FLOAT64T:
					result.F64s()[row] = src.F64s()[i]
				case dtype.BOOLT:
					if src.Bools()[i>>3]&(1<<(i&7)) != 0 {
						result.Bools()[row>>3] |= 1 << (row & 7)
					}
				case dtype.STRT:
					strings[row] = borrowedStringBytes(src.StringAt(i).View())
					if stringValidity != nil {
						stringValidity[row] = true
					}
				default:
					ok = false
				}
			}
		}
		if node.dType.ID() == dtype.STRT && ok {
			stringBranches[branch-1] = branchValues.vectors[node.children[branch]].Retain()
		}
		branchValues.release()
		subset.Release()
		if !ok {
			result.Release()
			return store.Vector{}
		}
	}
	if node.dType.ID() == dtype.STRT {
		if general {
			return store.MakeStringVector(a, strings, stringValidity)
		}
		return store.MakeStringVectorTemp(a, strings, stringValidity)
	}
	return result
}

func gatherCaseBatch(a *mem.Allocator, batch *store.Batch, indices []int64) *store.Batch {
	vectors := make([]store.Vector, batch.NVectors())
	for column := range vectors {
		src := batch.VectorAt(column)
		if src.TypeID() == dtype.STRT {
			strings := make([][]byte, len(indices))
			var valid []bool
			if src.Validity() != nil {
				valid = make([]bool, len(indices))
			}
			for i, index := range indices {
				if valid != nil && !src.Validity().IsSet(int(index)) {
					continue
				}
				strings[i] = borrowedStringBytes(src.StringAt(int(index)).View())
				if valid != nil {
					valid[i] = true
				}
			}
			vectors[column] = store.MakeStringVectorTemp(a, strings, valid)
		} else {
			vectors[column] = store.MakeVectorTemp(a, len(indices), src.Type(), src.Validity() != nil)
			if vectors[column].Kind() == store.VectorInvalid {
				break
			}
			for i, index := range indices {
				row := int(index)
				if vectors[column].Validity() != nil && !src.Validity().IsSet(row) {
					vectors[column].Validity().Clear(i)
				}
				switch src.TypeID() {
				case dtype.INT32T, dtype.DATET:
					vectors[column].I32s()[i] = src.I32s()[row]
				case dtype.INT64T, dtype.TIMESTAMPTZT:
					vectors[column].I64s()[i] = src.I64s()[row]
				case dtype.FLOAT32T:
					vectors[column].F32s()[i] = src.F32s()[row]
				case dtype.FLOAT64T:
					vectors[column].F64s()[i] = src.F64s()[row]
				case dtype.BOOLT:
					if src.Bools()[row>>3]&(1<<(row&7)) != 0 {
						vectors[column].Bools()[i>>3] |= 1 << (i & 7)
					}
				}
			}
		}
		if vectors[column].Kind() == store.VectorInvalid {
			break
		}
	}
	result := store.MakeBatch(vectors)
	if result == nil {
		for i := range vectors {
			vectors[i].Release()
		}
	}
	return result
}

func (v *physicalExprValues) release() {
	for i := range v.vectors {
		if v.set[i] {
			v.vectors[i].Release()
		}
	}
	v.vectors = nil
	v.set = nil
}

func materializeScalar(a *mem.Allocator, length int, scalar Scalar, general bool) store.Vector {
	if scalar.Type().ID() == dtype.STRT {
		values := make([][]byte, length)
		var valid []bool
		if scalar.IsNull() {
			valid = make([]bool, length)
		} else {
			value := borrowedStringBytes(scalar.stringValue())
			for i := range values {
				values[i] = value
			}
		}
		if general {
			return store.MakeStringVector(a, values, valid)
		}
		return store.MakeStringVectorTemp(a, values, valid)
	}
	var vector store.Vector
	if general {
		vector = store.MakeVector(a, length, scalar.Type(), scalar.IsNull())
	} else {
		vector = store.MakeVectorTemp(a, length, scalar.Type(), scalar.IsNull())
	}
	if vector.Kind() == store.VectorInvalid {
		return vector
	}
	if scalar.IsNull() {
		vector.Validity().ClearAll()
		return vector
	}
	switch scalar.Type().ID() {
	case dtype.INT32T, dtype.DATET:
		for i := range vector.I32s() {
			vector.I32s()[i] = scalar.i32()
		}
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		for i := range vector.I64s() {
			vector.I64s()[i] = scalar.i64()
		}
	case dtype.FLOAT32T:
		for i := range vector.F32s() {
			vector.F32s()[i] = scalar.f32()
		}
	case dtype.FLOAT64T:
		for i := range vector.F64s() {
			vector.F64s()[i] = scalar.f64()
		}
	case dtype.BOOLT:
		if scalar.boolean() {
			setBitmapAll(vector.Bools(), length)
		}
	}
	return vector
}

func evaluatePhysicalExprNode(node physicalExprNode, nodes []physicalExprNode, values []store.Vector, dst *store.Vector, a *mem.Allocator) bool {
	children := [3]*store.Vector{}
	for i := range int(node.childCount) {
		children[i] = &values[node.children[i]]
	}
	switch node.kind {
	case exprCast:
		return evaluateCast(children[0], dst)
	case exprUnary:
		return evaluateUnary(node.operation, children[0], dst, a)
	case exprBinary:
		if node.square {
			if !evaluateSquare(children[0], dst) {
				return false
			}
			copyValidity(dst.Validity(), children[0].Validity())
			return true
		}
		var leftScalar, rightScalar *Scalar
		if nodes[node.children[0]].kind == exprLiteral {
			leftScalar = &nodes[node.children[0]].literal
		}
		if nodes[node.children[1]].kind == exprLiteral {
			rightScalar = &nodes[node.children[1]].literal
		}
		return evaluateBinary(node.operation, children[0], children[1], leftScalar, rightScalar, dst, a)
	case exprTernary:
		var bounds [2]*Scalar
		if node.operation == exprOpBetween || node.operation == exprOpNotBetween || node.operation == exprOpClip {
			for i := 1; i <= 2; i++ {
				if nodes[node.children[i]].kind == exprLiteral {
					bounds[i-1] = &nodes[node.children[i]].literal
				}
			}
		}
		if node.clipI64F64 {
			source := &values[nodes[node.children[0]].children[0]]
			numop.ClipI64WithF64Bounds(source.I64s(), dst.F64s(), bounds[0].f64(), bounds[1].f64())
			copyValidity(dst.Validity(), source.Validity())
			if bounds[0].IsNull() || bounds[1].IsNull() {
				dst.Validity().ClearAll()
			}
			return true
		}
		return evaluateTernary(node.operation, children[0], children[1], children[2], dst, bounds)
	default:
		return false
	}
}

func evaluateSquare(src, dst *store.Vector) bool {
	switch src.TypeID() {
	case dtype.INT32T:
		numop.SqI32(src.I32s(), dst.I32s())
	case dtype.INT64T:
		if runtime.GOARCH == "arm64" {
			for i, value := range src.I64s() {
				dst.I64s()[i] = value * value
			}
		} else {
			numop.SqI64(src.I64s(), dst.I64s())
		}
	case dtype.FLOAT32T:
		numop.SqF32(src.F32s(), dst.F32s())
	case dtype.FLOAT64T:
		numop.SqF64(src.F64s(), dst.F64s())
	default:
		return false
	}
	return true
}

func evaluateCast(src, dst *store.Vector) bool {
	checked := false
	switch src.TypeID() {
	case dtype.INT32T:
		switch dst.TypeID() {
		case dtype.INT64T:
			numop.CastI32ToI64(src.I32s(), dst.I64s())
		case dtype.FLOAT32T:
			numop.CastI32ToF32(src.I32s(), dst.F32s())
		case dtype.FLOAT64T:
			numop.CastI32ToF64(src.I32s(), dst.F64s())
		default:
			return false
		}
	case dtype.INT64T:
		switch dst.TypeID() {
		case dtype.INT32T:
			numop.CastI64ToI32(src.I64s(), dst.I32s())
		case dtype.FLOAT32T:
			numop.CastI64ToF32(src.I64s(), dst.F32s())
		case dtype.FLOAT64T:
			numop.CastI64ToF64(src.I64s(), dst.F64s())
		default:
			return false
		}
	case dtype.FLOAT32T:
		switch dst.TypeID() {
		case dtype.INT32T:
			checked = true
			numop.CastF32ToI32Checked(src.F32s(), dst.I32s(), dst.Validity().Bytes())
		case dtype.INT64T:
			checked = true
			numop.CastF32ToI64Checked(src.F32s(), dst.I64s(), dst.Validity().Bytes())
		case dtype.FLOAT64T:
			numop.CastF32ToF64(src.F32s(), dst.F64s())
		default:
			return false
		}
	case dtype.FLOAT64T:
		switch dst.TypeID() {
		case dtype.INT32T:
			checked = true
			numop.CastF64ToI32Checked(src.F64s(), dst.I32s(), dst.Validity().Bytes())
		case dtype.INT64T:
			checked = true
			numop.CastF64ToI64Checked(src.F64s(), dst.I64s(), dst.Validity().Bytes())
		case dtype.FLOAT32T:
			numop.CastF64ToF32(src.F64s(), dst.F32s())
		default:
			return false
		}
	default:
		return false
	}
	if dst.Validity() != nil {
		if checked {
			dst.Validity().RecalcNiN()
			if src.Validity() != nil {
				dst.Validity().ANDInPlaceViN(src.Validity())
			}
		} else {
			copyValidity(dst.Validity(), src.Validity())
		}
	}
	return true
}

func evaluateUnary(operation exprOp, src, dst *store.Vector, a *mem.Allocator) bool {
	switch operation {
	case exprOpExtractYear, exprOpExtractMonth, exprOpExtractDay, exprOpTruncateYear, exprOpTruncateMonth:
		copyValidity(dst.Validity(), src.Validity())
		if !evaluateDateMapping(operation, src, dst) {
			return false
		}
	case exprOpAbs:
		switch src.TypeID() {
		case dtype.INT32T:
			numop.AbsI32(src.I32s(), dst.I32s())
		case dtype.INT64T:
			numop.AbsI64(src.I64s(), dst.I64s())
		case dtype.FLOAT32T:
			numop.AbsF32(src.F32s(), dst.F32s())
		case dtype.FLOAT64T:
			numop.AbsF64(src.F64s(), dst.F64s())
		default:
			return false
		}
		copyValidity(dst.Validity(), src.Validity())
	case exprOpNeg:
		switch src.TypeID() {
		case dtype.INT32T:
			numop.NegI32(src.I32s(), dst.I32s())
		case dtype.INT64T:
			numop.NegI64(src.I64s(), dst.I64s())
		case dtype.FLOAT32T:
			numop.NegF32(src.F32s(), dst.F32s())
		case dtype.FLOAT64T:
			numop.NegF64(src.F64s(), dst.F64s())
		default:
			return false
		}
		copyValidity(dst.Validity(), src.Validity())
	case exprOpSqrt:
		switch src.TypeID() {
		case dtype.INT32T:
			numop.SqrtI32(src.I32s(), dst.F32s())
		case dtype.INT64T:
			numop.SqrtI64(src.I64s(), dst.F64s())
		case dtype.FLOAT32T:
			numop.SqrtF32(src.F32s(), dst.F32s())
		case dtype.FLOAT64T:
			numop.SqrtF64(src.F64s(), dst.F64s())
		default:
			return false
		}
		copyValidity(dst.Validity(), src.Validity())
	case exprOpNot:
		validity, release := physicalValidityBytes(a, src)
		dstValidity, releaseDst := physicalOutputValidityBytes(a, dst)
		if src.Len() != 0 && (validity == nil || dstValidity == nil) {
			if release != nil {
				release.Release()
			}
			if releaseDst != nil {
				releaseDst.Release()
			}
			return false
		}
		boolop.Not(src.Bools(), validity, dst.Bools(), dstValidity, src.Len())
		if release != nil {
			release.Release()
		}
		if releaseDst != nil {
			releaseDst.Release()
		} else {
			dst.Validity().RecalcNiN()
		}
	case exprOpIsNull, exprOpIsNotNull:
		clear(dst.Bools())
		for i := range src.Len() {
			isNull := src.Validity() != nil && !src.Validity().IsSet(i)
			if (operation == exprOpIsNull && isNull) || (operation == exprOpIsNotNull && !isNull) {
				dst.Bools()[i>>3] |= 1 << (i & 7)
			}
		}
	default:
		return false
	}
	return true
}

func evaluateBinary(operation exprOp, left, right *store.Vector, leftScalar, rightScalar *Scalar, dst *store.Vector, a *mem.Allocator) bool {
	if operation == exprOpContains || operation == exprOpLike {
		clear(dst.Bools())
		for row := range dst.Len() {
			var l, r string
			if leftScalar != nil {
				l = leftScalar.stringValue()
			} else {
				l = left.StringAt(row).View()
			}
			if rightScalar != nil {
				r = rightScalar.stringValue()
			} else {
				r = right.StringAt(row).View()
			}
			match := false
			if operation == exprOpContains {
				match = strings.Contains(l, r)
			} else {
				match = likeMatches(l, r)
			}
			if match {
				dst.Bools()[row>>3] |= 1 << (row & 7)
			}
		}
		intersectBinaryValidity(dst.Validity(), left, right, leftScalar, rightScalar)
		return true
	}
	if isArithmetic(operation) {
		if !evaluateArithmetic(operation, left, right, leftScalar, rightScalar, dst) {
			return false
		}
		intersectBinaryValidity(dst.Validity(), left, right, leftScalar, rightScalar)
		return true
	}
	if isComparison(operation) {
		if !evaluateComparison(operation, left, right, leftScalar, rightScalar, dst) {
			return false
		}
		intersectBinaryValidity(dst.Validity(), left, right, leftScalar, rightScalar)
		return true
	}
	if operation == exprOpAnd || operation == exprOpOr {
		leftValidity, releaseLeft := physicalValidityBytes(a, left)
		rightValidity, releaseRight := physicalValidityBytes(a, right)
		dstValidity, releaseDst := physicalOutputValidityBytes(a, dst)
		if dst.Len() != 0 && (leftValidity == nil || rightValidity == nil || dstValidity == nil) {
			if releaseLeft != nil {
				releaseLeft.Release()
			}
			if releaseRight != nil {
				releaseRight.Release()
			}
			if releaseDst != nil {
				releaseDst.Release()
			}
			return false
		}
		if operation == exprOpAnd {
			boolop.And(left.Bools(), leftValidity, right.Bools(), rightValidity, dst.Bools(), dstValidity, dst.Len())
		} else {
			boolop.Or(left.Bools(), leftValidity, right.Bools(), rightValidity, dst.Bools(), dstValidity, dst.Len())
		}
		if releaseLeft != nil {
			releaseLeft.Release()
		}
		if releaseRight != nil {
			releaseRight.Release()
		}
		if releaseDst != nil {
			releaseDst.Release()
		} else {
			dst.Validity().RecalcNiN()
		}
		return true
	}
	if operation == exprOpCoalesce {
		return evaluateCoalesce(left, right, dst)
	}
	return false
}

func evaluateArithmetic(operation exprOp, left, right *store.Vector, leftScalar, rightScalar *Scalar, dst *store.Vector) bool {
	if leftScalar == nil && rightScalar != nil && evaluateArithmeticLiteral(operation, left, dst, *rightScalar, false) {
		return true
	}
	if leftScalar != nil && rightScalar == nil && evaluateArithmeticLiteral(operation, right, dst, *leftScalar, true) {
		return true
	}
	if left.TypeID() == dtype.DATET && right.TypeID() == dtype.DATET && operation == exprOpSub {
		numop.SubI32Vec(left.I32s(), right.I32s(), dst.I32s())
		return true
	}
	switch left.TypeID() {
	case dtype.INT32T:
		switch operation {
		case exprOpAdd:
			numop.AddI32Vec(left.I32s(), right.I32s(), dst.I32s())
		case exprOpSub:
			numop.SubI32Vec(left.I32s(), right.I32s(), dst.I32s())
		case exprOpMul:
			numop.MulI32Vec(left.I32s(), right.I32s(), dst.I32s())
		case exprOpDiv:
			numop.DivI32Vec(left.I32s(), right.I32s(), dst.F32s())
		}
	case dtype.INT64T:
		switch operation {
		case exprOpAdd:
			numop.AddI64Vec(left.I64s(), right.I64s(), dst.I64s())
		case exprOpSub:
			numop.SubI64Vec(left.I64s(), right.I64s(), dst.I64s())
		case exprOpMul:
			if runtime.GOARCH == "arm64" {
				for i, value := range left.I64s() {
					dst.I64s()[i] = value * right.I64s()[i]
				}
			} else {
				numop.MulI64Vec(left.I64s(), right.I64s(), dst.I64s())
			}
		case exprOpDiv:
			numop.DivI64Vec(left.I64s(), right.I64s(), dst.F64s())
		}
	case dtype.FLOAT32T:
		switch operation {
		case exprOpAdd:
			numop.AddF32Vec(left.F32s(), right.F32s(), dst.F32s())
		case exprOpSub:
			numop.SubF32Vec(left.F32s(), right.F32s(), dst.F32s())
		case exprOpMul:
			numop.MulF32Vec(left.F32s(), right.F32s(), dst.F32s())
		case exprOpDiv:
			numop.DivF32Vec(left.F32s(), right.F32s(), dst.F32s())
		}
	case dtype.FLOAT64T:
		switch operation {
		case exprOpAdd:
			numop.AddF64Vec(left.F64s(), right.F64s(), dst.F64s())
		case exprOpSub:
			numop.SubF64Vec(left.F64s(), right.F64s(), dst.F64s())
		case exprOpMul:
			numop.MulF64Vec(left.F64s(), right.F64s(), dst.F64s())
		case exprOpDiv:
			numop.DivF64Vec(left.F64s(), right.F64s(), dst.F64s())
		}
	default:
		return false
	}
	return true
}

func evaluateArithmeticLiteral(operation exprOp, src, dst *store.Vector, literal Scalar, literalLeft bool) bool {
	switch src.TypeID() {
	case dtype.INT32T, dtype.DATET:
		value := literal.i32()
		switch operation {
		case exprOpAdd:
			numop.AddI32Lit(src.I32s(), dst.I32s(), value)
		case exprOpSub:
			if literalLeft {
				numop.SubI32LitLeft(src.I32s(), dst.I32s(), value)
			} else {
				numop.SubI32Lit(src.I32s(), dst.I32s(), value)
			}
		case exprOpMul:
			numop.MulI32Lit(src.I32s(), dst.I32s(), value)
		case exprOpDiv:
			if literalLeft {
				numop.DivI32LitLeft(src.I32s(), dst.F32s(), float32(value))
			} else {
				numop.DivI32Lit(src.I32s(), dst.F32s(), float32(value))
			}
		default:
			return false
		}
	case dtype.INT64T:
		value := literal.i64()
		switch operation {
		case exprOpAdd:
			numop.AddI64Lit(src.I64s(), dst.I64s(), value)
		case exprOpSub:
			if literalLeft {
				numop.SubI64LitLeft(src.I64s(), dst.I64s(), value)
			} else {
				numop.SubI64Lit(src.I64s(), dst.I64s(), value)
			}
		case exprOpMul:
			if runtime.GOARCH == "arm64" {
				for i, input := range src.I64s() {
					dst.I64s()[i] = input * value
				}
			} else {
				numop.MulI64Lit(src.I64s(), dst.I64s(), value)
			}
		case exprOpDiv:
			if literalLeft {
				numop.DivI64LitLeft(src.I64s(), dst.F64s(), float64(value))
			} else {
				numop.DivI64Lit(src.I64s(), dst.F64s(), float64(value))
			}
		default:
			return false
		}
	case dtype.FLOAT32T:
		value := literal.f32()
		switch operation {
		case exprOpAdd:
			numop.AddF32Lit(src.F32s(), dst.F32s(), value)
		case exprOpSub:
			if literalLeft {
				numop.SubF32LitLeft(src.F32s(), dst.F32s(), value)
			} else {
				numop.SubF32Lit(src.F32s(), dst.F32s(), value)
			}
		case exprOpMul:
			numop.MulF32Lit(src.F32s(), dst.F32s(), value)
		case exprOpDiv:
			if literalLeft {
				numop.DivF32LitLeft(src.F32s(), dst.F32s(), value)
			} else {
				numop.DivF32Lit(src.F32s(), dst.F32s(), value)
			}
		default:
			return false
		}
	case dtype.FLOAT64T:
		value := literal.f64()
		switch operation {
		case exprOpAdd:
			numop.AddF64Lit(src.F64s(), dst.F64s(), value)
		case exprOpSub:
			if literalLeft {
				numop.SubF64LitLeft(src.F64s(), dst.F64s(), value)
			} else {
				numop.SubF64Lit(src.F64s(), dst.F64s(), value)
			}
		case exprOpMul:
			numop.MulF64Lit(src.F64s(), dst.F64s(), value)
		case exprOpDiv:
			if literalLeft {
				numop.DivF64LitLeft(src.F64s(), dst.F64s(), value)
			} else {
				numop.DivF64Lit(src.F64s(), dst.F64s(), value)
			}
		default:
			return false
		}
	default:
		return false
	}
	return true
}

func evaluateComparison(operation exprOp, left, right *store.Vector, leftScalar, rightScalar *Scalar, dst *store.Vector) bool {
	if leftScalar == nil && rightScalar != nil && evaluateComparisonLiteral(operation, left, dst.Bools(), *rightScalar) {
		return true
	}
	if leftScalar != nil && rightScalar == nil && evaluateComparisonLiteral(reverseComparison(operation), right, dst.Bools(), *leftScalar) {
		return true
	}
	return evaluateComparisonVectors(operation, left, right, dst)
}

func evaluateComparisonLiteral(operation exprOp, src *store.Vector, dst []byte, literal Scalar) bool {
	switch src.TypeID() {
	case dtype.INT32T, dtype.DATET:
		evaluateI32ComparisonLiteral(src.I32s(), dst, operation, literal.i32())
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		evaluateI64ComparisonLiteral(src.I64s(), dst, operation, literal.i64())
	case dtype.FLOAT32T:
		evaluateF32ComparisonLiteral(src.F32s(), dst, operation, literal.f32())
	case dtype.FLOAT64T:
		evaluateF64ComparisonLiteral(src.F64s(), dst, operation, literal.f64())
	case dtype.STRT:
		clear(dst)
		for i := range src.Len() {
			if comparisonMatches(compareStrings(src.StringAt(i).View(), literal.stringValue()), operation) {
				dst[i>>3] |= 1 << (i & 7)
			}
		}
	case dtype.BOOLT:
		for i := range dst {
			if literal.boolean() {
				dst[i] = src.Bools()[i]
			} else {
				dst[i] = ^src.Bools()[i]
			}
			if operation == exprOpNE {
				dst[i] = ^dst[i]
			}
		}
		maskBitmapTail(dst, src.Len())
	default:
		return false
	}
	return true
}

func evaluateComparisonVectors(operation exprOp, left, right, dst *store.Vector) bool {
	switch left.TypeID() {
	case dtype.INT32T, dtype.DATET:
		evaluateI32ComparisonVectors(left.I32s(), right.I32s(), dst.Bools(), operation)
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		evaluateI64ComparisonVectors(left.I64s(), right.I64s(), dst.Bools(), operation)
	case dtype.FLOAT32T:
		evaluateF32ComparisonVectors(left.F32s(), right.F32s(), dst.Bools(), operation)
	case dtype.FLOAT64T:
		evaluateF64ComparisonVectors(left.F64s(), right.F64s(), dst.Bools(), operation)
	case dtype.BOOLT:
		if operation != exprOpEQ && operation != exprOpNE {
			return false
		}
		for i := range dst.Bools() {
			if operation == exprOpEQ {
				dst.Bools()[i] = ^(left.Bools()[i] ^ right.Bools()[i])
			} else {
				dst.Bools()[i] = left.Bools()[i] ^ right.Bools()[i]
			}
		}
		maskBitmapTail(dst.Bools(), dst.Len())
	case dtype.STRT:
		clear(dst.Bools())
		for i := range left.Len() {
			comparison := compareStrings(left.StringAt(i).View(), right.StringAt(i).View())
			if comparisonMatches(comparison, operation) {
				dst.Bools()[i>>3] |= 1 << (i & 7)
			}
		}
	default:
		return false
	}
	return true
}

func evaluateI32ComparisonVectors(left, right []int32, dst []byte, operation exprOp) {
	switch operation {
	case exprOpEQ:
		cmpop.EqI32Vec(left, right, dst)
	case exprOpNE:
		cmpop.NeqI32Vec(left, right, dst)
	case exprOpLT:
		cmpop.LtI32Vec(left, right, dst)
	case exprOpLE:
		cmpop.LeI32Vec(left, right, dst)
	case exprOpGT:
		cmpop.GtI32Vec(left, right, dst)
	case exprOpGE:
		cmpop.GeI32Vec(left, right, dst)
	}
}

func evaluateI64ComparisonVectors(left, right []int64, dst []byte, operation exprOp) {
	switch operation {
	case exprOpEQ:
		cmpop.EqI64Vec(left, right, dst)
	case exprOpNE:
		cmpop.NeqI64Vec(left, right, dst)
	case exprOpLT:
		cmpop.LtI64Vec(left, right, dst)
	case exprOpLE:
		cmpop.LeI64Vec(left, right, dst)
	case exprOpGT:
		cmpop.GtI64Vec(left, right, dst)
	case exprOpGE:
		cmpop.GeI64Vec(left, right, dst)
	}
}

func evaluateF32ComparisonVectors(left, right []float32, dst []byte, operation exprOp) {
	switch operation {
	case exprOpEQ:
		cmpop.EqF32Vec(left, right, dst)
	case exprOpNE:
		cmpop.NeqF32Vec(left, right, dst)
	case exprOpLT:
		cmpop.LtF32Vec(left, right, dst)
	case exprOpLE:
		cmpop.LeF32Vec(left, right, dst)
	case exprOpGT:
		cmpop.GtF32Vec(left, right, dst)
	case exprOpGE:
		cmpop.GeF32Vec(left, right, dst)
	}
}

func evaluateF64ComparisonVectors(left, right []float64, dst []byte, operation exprOp) {
	switch operation {
	case exprOpEQ:
		cmpop.EqF64Vec(left, right, dst)
	case exprOpNE:
		cmpop.NeqF64Vec(left, right, dst)
	case exprOpLT:
		cmpop.LtF64Vec(left, right, dst)
	case exprOpLE:
		cmpop.LeF64Vec(left, right, dst)
	case exprOpGT:
		cmpop.GtF64Vec(left, right, dst)
	case exprOpGE:
		cmpop.GeF64Vec(left, right, dst)
	}
}

func evaluateI32ComparisonLiteral(src []int32, dst []byte, operation exprOp, literal int32) {
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

func evaluateI64ComparisonLiteral(src []int64, dst []byte, operation exprOp, literal int64) {
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

func evaluateF32ComparisonLiteral(src []float32, dst []byte, operation exprOp, literal float32) {
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

func evaluateF64ComparisonLiteral(src []float64, dst []byte, operation exprOp, literal float64) {
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

func evaluateCoalesce(left, right, dst *store.Vector) bool {
	if dst.Validity() != nil {
		dst.Validity().ClearAll()
	}
	for i := range dst.Len() {
		useLeft := left.Validity() == nil || left.Validity().IsSet(i)
		if !useLeft && right.Validity() != nil && !right.Validity().IsSet(i) {
			continue
		}
		switch dst.TypeID() {
		case dtype.INT32T, dtype.DATET:
			if useLeft {
				dst.I32s()[i] = left.I32s()[i]
			} else {
				dst.I32s()[i] = right.I32s()[i]
			}
		case dtype.INT64T, dtype.TIMESTAMPTZT:
			if useLeft {
				dst.I64s()[i] = left.I64s()[i]
			} else {
				dst.I64s()[i] = right.I64s()[i]
			}
		case dtype.FLOAT32T:
			if useLeft {
				dst.F32s()[i] = left.F32s()[i]
			} else {
				dst.F32s()[i] = right.F32s()[i]
			}
		case dtype.FLOAT64T:
			if useLeft {
				dst.F64s()[i] = left.F64s()[i]
			} else {
				dst.F64s()[i] = right.F64s()[i]
			}
		case dtype.BOOLT:
			value := left.Bools()[i>>3] >> (i & 7) & 1
			if !useLeft {
				value = right.Bools()[i>>3] >> (i & 7) & 1
			}
			if value != 0 {
				dst.Bools()[i>>3] |= 1 << (i & 7)
			}
		default:
			return false
		}
		if dst.Validity() != nil {
			dst.Validity().Set(i)
		}
	}
	return true
}

func evaluateStringCoalesce(a *mem.Allocator, left, right *store.Vector, general, nullable bool) store.Vector {
	values := make([][]byte, left.Len())
	var valid []bool
	if nullable {
		valid = make([]bool, left.Len())
	}
	for i := range left.Len() {
		selected := left
		if left.Validity() != nil && !left.Validity().IsSet(i) {
			selected = right
		}
		if selected.Validity() != nil && !selected.Validity().IsSet(i) {
			continue
		}
		values[i] = borrowedStringBytes(selected.StringAt(i).View())
		if valid != nil {
			valid[i] = true
		}
	}
	if general {
		return store.MakeStringVector(a, values, valid)
	}
	return store.MakeStringVectorTemp(a, values, valid)
}

func evaluateTernary(operation exprOp, value, lower, upper, dst *store.Vector, bounds [2]*Scalar) bool {
	if operation == exprOpClip {
		if !evaluateClip(value, lower, upper, dst, bounds) {
			return false
		}
	} else if operation == exprOpBetween || operation == exprOpNotBetween {
		if !evaluateBetween(operation, value, lower, upper, dst, bounds) {
			return false
		}
	} else {
		return false
	}
	intersectValidity(dst.Validity(), value.Validity(), lower.Validity(), upper.Validity())
	if (bounds[0] != nil && bounds[0].IsNull()) || (bounds[1] != nil && bounds[1].IsNull()) {
		dst.Validity().ClearAll()
	}
	return true
}

func evaluateBetween(operation exprOp, value, lower, upper, dst *store.Vector, bounds [2]*Scalar) bool {
	if bounds[0] != nil && bounds[1] != nil && value.Len() != 0 {
		switch value.TypeID() {
		case dtype.INT32T, dtype.DATET:
			if operation == exprOpBetween {
				cmpop.BetI32(value.I32s(), dst.Bools(), bounds[0].i32(), bounds[1].i32())
			} else {
				cmpop.NBetI32(value.I32s(), dst.Bools(), bounds[0].i32(), bounds[1].i32())
			}
			return true
		case dtype.INT64T, dtype.TIMESTAMPTZT:
			if operation == exprOpBetween {
				cmpop.BetI64(value.I64s(), dst.Bools(), bounds[0].i64(), bounds[1].i64())
			} else {
				cmpop.NBetI64(value.I64s(), dst.Bools(), bounds[0].i64(), bounds[1].i64())
			}
			return true
		case dtype.FLOAT32T:
			if operation == exprOpBetween {
				cmpop.BetF32(value.F32s(), dst.Bools(), bounds[0].f32(), bounds[1].f32())
			} else {
				cmpop.NBetF32(value.F32s(), dst.Bools(), bounds[0].f32(), bounds[1].f32())
			}
			return true
		case dtype.FLOAT64T:
			if operation == exprOpBetween {
				cmpop.BetF64(value.F64s(), dst.Bools(), bounds[0].f64(), bounds[1].f64())
			} else {
				cmpop.NBetF64(value.F64s(), dst.Bools(), bounds[0].f64(), bounds[1].f64())
			}
			return true
		}
	}
	clear(dst.Bools())
	for i := range value.Len() {
		var match bool
		switch value.TypeID() {
		case dtype.INT32T, dtype.DATET:
			x := value.I32s()[i]
			var lo, hi int32
			if bounds[0] != nil {
				lo = bounds[0].i32()
			} else {
				lo = lower.I32s()[i]
			}
			if bounds[1] != nil {
				hi = bounds[1].i32()
			} else {
				hi = upper.I32s()[i]
			}
			match = x >= lo && x <= hi
			if operation == exprOpNotBetween {
				match = x < lo || x > hi
			}
		case dtype.INT64T, dtype.TIMESTAMPTZT:
			x := value.I64s()[i]
			var lo, hi int64
			if bounds[0] != nil {
				lo = bounds[0].i64()
			} else {
				lo = lower.I64s()[i]
			}
			if bounds[1] != nil {
				hi = bounds[1].i64()
			} else {
				hi = upper.I64s()[i]
			}
			match = x >= lo && x <= hi
			if operation == exprOpNotBetween {
				match = x < lo || x > hi
			}
		case dtype.FLOAT32T:
			x := value.F32s()[i]
			var lo, hi float32
			if bounds[0] != nil {
				lo = bounds[0].f32()
			} else {
				lo = lower.F32s()[i]
			}
			if bounds[1] != nil {
				hi = bounds[1].f32()
			} else {
				hi = upper.F32s()[i]
			}
			match = x >= lo && x <= hi
			if operation == exprOpNotBetween {
				match = x < lo || x > hi
			}
		case dtype.FLOAT64T:
			x := value.F64s()[i]
			var lo, hi float64
			if bounds[0] != nil {
				lo = bounds[0].f64()
			} else {
				lo = lower.F64s()[i]
			}
			if bounds[1] != nil {
				hi = bounds[1].f64()
			} else {
				hi = upper.F64s()[i]
			}
			match = x >= lo && x <= hi
			if operation == exprOpNotBetween {
				match = x < lo || x > hi
			}
		case dtype.STRT:
			x := value.StringAt(i).View()
			var lo, hi string
			if bounds[0] != nil {
				lo = bounds[0].stringValue()
			} else {
				lo = lower.StringAt(i).View()
			}
			if bounds[1] != nil {
				hi = bounds[1].stringValue()
			} else {
				hi = upper.StringAt(i).View()
			}
			match = x >= lo && x <= hi
			if operation == exprOpNotBetween {
				match = x < lo || x > hi
			}
		default:
			return false
		}
		if match {
			dst.Bools()[i>>3] |= 1 << (i & 7)
		}
	}
	return true
}

func evaluateClip(value, lower, upper, dst *store.Vector, bounds [2]*Scalar) bool {
	constant := bounds[0] != nil && bounds[1] != nil && value.Len() != 0
	if constant && value.Type().IsFloating() {
		if value.TypeID() == dtype.FLOAT32T {
			constant = !math.IsNaN(float64(bounds[0].f32())) && !math.IsNaN(float64(bounds[1].f32()))
		} else {
			constant = !math.IsNaN(bounds[0].f64()) && !math.IsNaN(bounds[1].f64())
		}
	}
	if constant {
		switch value.TypeID() {
		case dtype.INT32T:
			numop.ClipI32WithI32Bounds(value.I32s(), dst.I32s(), bounds[0].i32(), bounds[1].i32())
			return true
		case dtype.INT64T:
			numop.ClipI64WithI64Bounds(value.I64s(), dst.I64s(), bounds[0].i64(), bounds[1].i64())
			return true
		case dtype.FLOAT32T:
			numop.ClipF32WithF32Bounds(value.F32s(), dst.F32s(), bounds[0].f32(), bounds[1].f32())
			return true
		case dtype.FLOAT64T:
			numop.ClipF64WithF64Bounds(value.F64s(), dst.F64s(), bounds[0].f64(), bounds[1].f64())
			return true
		}
	}
	for i := range value.Len() {
		switch value.TypeID() {
		case dtype.INT32T:
			x := value.I32s()[i]
			var lo, hi int32
			if bounds[0] != nil {
				lo = bounds[0].i32()
			} else {
				lo = lower.I32s()[i]
			}
			if bounds[1] != nil {
				hi = bounds[1].i32()
			} else {
				hi = upper.I32s()[i]
			}
			if x < lo {
				x = lo
			}
			if x > hi {
				x = hi
			}
			dst.I32s()[i] = x
		case dtype.INT64T:
			x := value.I64s()[i]
			var lo, hi int64
			if bounds[0] != nil {
				lo = bounds[0].i64()
			} else {
				lo = lower.I64s()[i]
			}
			if bounds[1] != nil {
				hi = bounds[1].i64()
			} else {
				hi = upper.I64s()[i]
			}
			if x < lo {
				x = lo
			}
			if x > hi {
				x = hi
			}
			dst.I64s()[i] = x
		case dtype.FLOAT32T:
			x := value.F32s()[i]
			var lo, hi float32
			if bounds[0] != nil {
				lo = bounds[0].f32()
			} else {
				lo = lower.F32s()[i]
			}
			if bounds[1] != nil {
				hi = bounds[1].f32()
			} else {
				hi = upper.F32s()[i]
			}
			if x < lo {
				x = lo
			}
			if x > hi {
				x = hi
			}
			dst.F32s()[i] = x
		case dtype.FLOAT64T:
			x := value.F64s()[i]
			var lo, hi float64
			if bounds[0] != nil {
				lo = bounds[0].f64()
			} else {
				lo = lower.F64s()[i]
			}
			if bounds[1] != nil {
				hi = bounds[1].f64()
			} else {
				hi = upper.F64s()[i]
			}
			if x < lo {
				x = lo
			}
			if x > hi {
				x = hi
			}
			dst.F64s()[i] = x
		default:
			return false
		}
	}
	return true
}

func copyValidity(dst, src *store.BitMap) {
	if dst == nil {
		return
	}
	if src == nil {
		dst.SetAll()
		return
	}
	copy(dst.Bytes(), src.Bytes())
	dst.RecalcNiN()
}

func intersectValidity(dst *store.BitMap, inputs ...*store.BitMap) {
	if dst == nil {
		return
	}
	dst.SetAll()
	for _, input := range inputs {
		if input != nil {
			dst.ANDInPlaceViN(input)
		}
	}
}

func intersectBinaryValidity(dst *store.BitMap, left, right *store.Vector, leftScalar, rightScalar *Scalar) {
	if dst == nil {
		return
	}
	if (leftScalar != nil && leftScalar.IsNull()) || (rightScalar != nil && rightScalar.IsNull()) {
		dst.ClearAll()
		return
	}
	dst.SetAll()
	if leftScalar == nil && left.Validity() != nil {
		dst.ANDInPlaceViN(left.Validity())
	}
	if rightScalar == nil && right.Validity() != nil {
		dst.ANDInPlaceViN(right.Validity())
	}
}

func physicalValidityBytes(a *mem.Allocator, vector *store.Vector) ([]byte, *store.BitMap) {
	if vector.Validity() != nil {
		return vector.Validity().Bytes(), nil
	}
	validity := store.MakeBitMapTemp(a, vector.Len())
	if validity == nil {
		return nil, nil
	}
	validity.SetAll()
	return validity.Bytes(), validity
}

func physicalOutputValidityBytes(a *mem.Allocator, vector *store.Vector) ([]byte, *store.BitMap) {
	if vector.Validity() != nil {
		return vector.Validity().Bytes(), nil
	}
	validity := store.MakeBitMapTemp(a, vector.Len())
	if validity == nil {
		return nil, nil
	}
	return validity.Bytes(), validity
}

func compareStrings(left, right string) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func comparisonMatches(comparison int, operation exprOp) bool {
	switch operation {
	case exprOpEQ:
		return comparison == 0
	case exprOpNE:
		return comparison != 0
	case exprOpLT:
		return comparison < 0
	case exprOpLE:
		return comparison <= 0
	case exprOpGT:
		return comparison > 0
	case exprOpGE:
		return comparison >= 0
	default:
		return false
	}
}

func setBitmapAll(bitmap []byte, length int) {
	for i := range bitmap {
		bitmap[i] = 0xff
	}
	maskBitmapTail(bitmap, length)
}

func maskBitmapTail(bitmap []byte, length int) {
	if length == 0 || length&7 == 0 {
		return
	}
	bitmap[len(bitmap)-1] &= byte(1<<(length&7) - 1)
}

func borrowedStringBytes(value string) []byte {
	if value == "" {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(value), len(value))
}
