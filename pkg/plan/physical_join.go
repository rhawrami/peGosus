package plan

import (
	"context"
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type physicalJoinSpec struct {
	right        *PhysicalPlan
	kind         JoinKind
	leftColumns  int
	rightColumns int
	leftKeys     []physicalExprProgram
	rightKeys    []physicalExprProgram
	residual     *physicalExprProgram
}

type joinState struct {
	keys        distinctState
	rightRows   packedRows
	scope       *mem.AllocationScope
	head        *mem.Segment
	next        *mem.Segment
	matched     *mem.Segment
	charged     int64
	initialized bool
}

func (s *joinState) release() {
	s.keys.release()
	s.rightRows.release()
	for _, segment := range []*mem.Segment{s.head, s.next, s.matched} {
		if segment != nil {
			segment.Dec()
		}
	}
}

func (s *joinState) growArray(a *mem.Allocator, target **mem.Segment, length, width int, budget int64) bool {
	capacity, oldBytes := 0, 0
	if *target != nil {
		oldBytes = (*target).Len()
		capacity = oldBytes / width
	}
	if length <= capacity {
		return true
	}
	capacity = max(16, capacity)
	for capacity < length {
		if capacity > int(^uint(0)>>1)/2 {
			return false
		}
		capacity *= 2
	}
	if capacity > int(^uint(0)>>1)/width || int64(capacity)*int64(width) > budget-s.keys.charged-s.charged {
		return false
	}
	segment, ok := allocOperatorSegment(a, s.scope, capacity*width)
	if !ok {
		return false
	}
	clear(segment.AsBytes())
	if *target != nil {
		copy(segment.AsBytes(), (*target).AsBytes())
		(*target).Dec()
	}
	*target = segment
	s.charged += int64(segment.Len() - oldBytes)
	return true
}

func joinableKeys(batch *store.Batch, row int) bool {
	for column := range batch.NVectors() {
		v := batch.VectorAt(column)
		if v.Validity() != nil && !v.Validity().IsSet(row) {
			return false
		}
		if v.TypeID() == dtype.FLOAT32T && math.IsNaN(float64(v.F32s()[row])) || v.TypeID() == dtype.FLOAT64T && math.IsNaN(v.F64s()[row]) {
			return false
		}
	}
	return true
}

func makeJoinKeyBatch(a *mem.Allocator, batch *store.Batch, programs []physicalExprProgram) (*store.Batch, []physicalExprValues) {
	values := make([]physicalExprValues, len(programs))
	vectors := make([]store.Vector, len(programs))
	for i, program := range programs {
		var ok bool
		values[i], ok = executePhysicalExprProgram(a, batch, program)
		if !ok {
			for j := range values {
				values[j].release()
				vectors[j].Release()
			}
			return nil, nil
		}
		vectors[i] = values[i].vectors[program.roots[0]].Retain()
	}
	result := store.MakeBatch(vectors)
	if result == nil {
		for j := range values {
			values[j].release()
			vectors[j].Release()
		}
	}
	return result, values
}

func (s *joinState) build(ctx context.Context, a *mem.Allocator, spec *physicalJoinSpec, budget int64) ExecutionResult {
	if s.initialized {
		return ExecutionResult{code: ExecutionCompleted}
	}
	s.initialized = true
	result := spec.right.ExecuteWithOptions(ctx, a, ExecutionOptions{MemoryBudget: budget}, func(batch *store.Batch) bool {
		keyBatch, values := makeJoinKeyBatch(a, batch, spec.rightKeys)
		if keyBatch == nil {
			return false
		}
		defer keyBatch.Release()
		defer func() {
			for i := range values {
				values[i].release()
			}
		}()
		selection := batch.Selection().MakeBitMapTemp(a)
		if batch.Selection() != nil && selection == nil {
			return false
		}
		if selection != nil {
			defer selection.Release()
		}
		for row := range batch.Len() {
			if selection != nil && !selection.IsSet(row) {
				continue
			}
			index := -1
			if joinableKeys(keyBatch, row) {
				var ok bool
				index, ok = s.keys.add(a, keyBatch, row, budget-s.charged)
				if !ok {
					return false
				}
				if !s.growArray(a, &s.head, index+1, 8, budget) {
					return false
				}
			}
			if !s.growArray(a, &s.next, s.rightRows.length+1, 8, budget) || !s.growArray(a, &s.matched, s.rightRows.length+1, 1, budget) {
				return false
			}
			prior := s.rightRows.bytes()
			if !s.rightRows.append(a, batch, row, budget-s.keys.charged-s.charged+prior) {
				return false
			}
			s.charged = saturatingAdd(s.charged, s.rightRows.bytes()-prior)
			rightIndex := s.rightRows.length - 1
			if index >= 0 {
				heads := s.head.AsU64T()
				s.next.AsU64T()[rightIndex] = heads[index]
				heads[index] = uint64(rightIndex) + 1
			}
		}
		return true
	})
	if result.Code() == ExecutionStopped {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	return result
}

func (s *joinState) probe(a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64) (*store.Batch, ExecutionResult) {
	spec := step.join
	keyBatch, values := makeJoinKeyBatch(a, batch, spec.leftKeys)
	if keyBatch == nil {
		return nil, ExecutionResult{code: ExecutionFailed}
	}
	defer keyBatch.Release()
	defer func() {
		for i := range values {
			values[i].release()
		}
	}()
	selection := batch.Selection().MakeBitMapTemp(a)
	if batch.Selection() != nil && selection == nil {
		return nil, ExecutionResult{code: ExecutionResourceExhausted}
	}
	if selection != nil {
		defer selection.Release()
	}
	rows := &packedRows{scope: s.scope}
	defer rows.release()
	full := make([]Scalar, spec.leftColumns+spec.rightColumns)
	outputBudget := (budget - s.charged - s.keys.charged) / 2
	for row := range batch.Len() {
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		for column := range spec.leftColumns {
			full[column] = scalarAt(batch.VectorAt(column), row)
		}
		var candidate uint64
		if joinableKeys(keyBatch, row) {
			if index, found := s.keys.lookup(a, keyBatch, row); found {
				candidate = s.head.AsU64T()[index]
			} else if s.scope != nil && s.scope.Exhausted() {
				return nil, ExecutionResult{code: ExecutionResourceExhausted}
			}
		}
		matched := false
		for candidate != 0 {
			rightIndex := int(candidate - 1)
			candidate = s.next.AsU64T()[rightIndex]
			for column := range spec.rightColumns {
				full[spec.leftColumns+column] = s.rightRows.at(rightIndex, column)
			}
			if spec.residual != nil {
				cost := int64(len(full) * 64)
				for _, value := range full {
					if value.Type().ID() == dtype.STRT && !value.IsNull() {
						cost = saturatingAdd(cost, int64(len(value.text)))
					}
				}
				if cost > (budget-s.charged-s.keys.charged-rows.bytes())/3 {
					return nil, ExecutionResult{code: ExecutionResourceExhausted}
				}
				one := makeJoinScalarBatch(a, full)
				if one == nil {
					return nil, ExecutionResult{code: ExecutionFailed}
				}
				result, ok := executePhysicalExprProgram(a, one, *spec.residual)
				one.Release()
				if !ok {
					return nil, ExecutionResult{code: ExecutionFailed}
				}
				v := &result.vectors[spec.residual.roots[0]]
				pass := (v.Validity() == nil || v.Validity().IsSet(0)) && v.Bools()[0]&1 != 0
				result.release()
				if !pass {
					continue
				}
			}
			matched = true
			s.matched.AsBytes()[rightIndex] = 1
			if spec.kind == JoinAnti {
				continue
			}
			if spec.kind == JoinSemi {
				break
			}
			if !rows.appendScalars(a, full, outputBudget) {
				return nil, ExecutionResult{code: ExecutionResourceExhausted}
			}
		}
		if spec.kind == JoinSemi || spec.kind == JoinAnti {
			if (spec.kind == JoinSemi && matched) || (spec.kind == JoinAnti && !matched) {
				if !rows.appendScalars(a, full[:spec.leftColumns], outputBudget) {
					return nil, ExecutionResult{code: ExecutionResourceExhausted}
				}
			}
		} else if !matched && (spec.kind == JoinLeft || spec.kind == JoinFull) {
			for column := range spec.rightColumns {
				full[spec.leftColumns+column] = MakeNullScalar(s.rightRowsType(spec, column))
			}
			if !rows.appendScalars(a, full, outputBudget) {
				return nil, ExecutionResult{code: ExecutionResourceExhausted}
			}
		}
	}
	if rows.length == 0 {
		return nil, ExecutionResult{code: ExecutionCompleted}
	}
	output := rows.makeBatch(a, step.schema)
	if output == nil {
		return nil, ExecutionResult{code: ExecutionFailed}
	}
	return output, ExecutionResult{code: ExecutionCompleted}
}

func (s *joinState) rightRowsType(spec *physicalJoinSpec, column int) dtype.Type {
	return spec.right.schema.FieldAt(column).Type()
}

func makeJoinScalarBatch(a *mem.Allocator, values []Scalar) *store.Batch {
	vectors := make([]store.Vector, len(values))
	for i, value := range values {
		vectors[i] = materializeScalar(a, 1, value, false)
	}
	result := store.MakeBatch(vectors)
	if result == nil {
		for i := range vectors {
			vectors[i].Release()
		}
	}
	return result
}

func (s *joinState) finish(a *mem.Allocator, step physicalStep, budget int64) (*store.Batch, bool) {
	spec := step.join
	if spec.kind != JoinFull {
		return nil, true
	}
	rows := &packedRows{scope: s.scope}
	defer rows.release()
	full := make([]Scalar, spec.leftColumns+spec.rightColumns)
	for column := range spec.leftColumns {
		full[column] = MakeNullScalar(step.schema.FieldAt(column).Type())
	}
	outputBudget := (budget - s.charged - s.keys.charged) / 2
	for index := range s.rightRows.length {
		if s.matched.AsBytes()[index] != 0 {
			continue
		}
		for column := range spec.rightColumns {
			full[spec.leftColumns+column] = s.rightRows.at(index, column)
		}
		if !rows.appendScalars(a, full, outputBudget) {
			return nil, false
		}
	}
	if rows.length == 0 {
		return nil, true
	}
	return rows.makeBatch(a, step.schema), true
}
