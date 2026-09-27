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
	index       map[int][]int
	rightRows   []distinctRow
	matched     []bool
	charged     int64
	initialized bool
}

func (s *joinState) release() {
	s.keys.release()
	for _, row := range s.rightRows {
		for _, text := range row.text {
			if text != nil {
				text.Dec()
			}
		}
	}
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
		if selection != nil {
			defer selection.Release()
		}
		for row := range batch.Len() {
			if selection != nil && !selection.IsSet(row) {
				continue
			}
			charge := int64(batch.NVectors() * 64)
			for column := range batch.NVectors() {
				v := batch.VectorAt(column)
				if v.TypeID() == dtype.STRT && (v.Validity() == nil || v.Validity().IsSet(row)) {
					charge = saturatingAdd(charge, int64(len(v.Strings()[row].View())))
				}
			}
			if charge > budget-s.charged-s.keys.charged {
				return false
			}
			record := distinctRow{values: make([]Scalar, batch.NVectors()), text: make([]*mem.Segment, batch.NVectors())}
			for column := range record.values {
				record.values[column], record.text[column] = ownScalar(a, scalarAt(batch.VectorAt(column), row))
			}
			if joinableKeys(keyBatch, row) {
				key, ok := s.keys.add(a, keyBatch, row, budget-s.charged)
				if !ok {
					for _, text := range record.text {
						if text != nil {
							text.Dec()
						}
					}
					return false
				}
				if s.index == nil {
					s.index = make(map[int][]int)
				}
				s.index[key] = append(s.index[key], len(s.rightRows))
			}
			s.rightRows = append(s.rightRows, record)
			s.matched = append(s.matched, false)
			s.charged += charge
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
	if selection != nil {
		defer selection.Release()
	}
	rows := make([]distinctRow, 0)
	outputCharge := int64(0)
	appendRow := func(values []Scalar) bool {
		cost := int64(len(values) * 64)
		for _, value := range values {
			if value.Type().ID() == dtype.STRT && !value.IsNull() {
				cost = saturatingAdd(cost, int64(len(value.text)))
			}
		}
		if cost > (budget-s.charged-s.keys.charged)/2-outputCharge {
			return false
		}
		outputCharge += cost
		rows = append(rows, distinctRow{values: values})
		return true
	}
	for row := range batch.Len() {
		if selection != nil && !selection.IsSet(row) {
			continue
		}
		var candidates []int
		if joinableKeys(keyBatch, row) {
			if index, found := s.keys.lookup(a, keyBatch, row); found {
				candidates = s.index[index]
			}
		}
		matched := false
		for _, rightIndex := range candidates {
			full := make([]Scalar, 0, spec.leftColumns+spec.rightColumns)
			for column := range spec.leftColumns {
				full = append(full, scalarAt(batch.VectorAt(column), row))
			}
			full = append(full, s.rightRows[rightIndex].values...)
			if spec.residual != nil {
				cost := int64(len(full) * 64)
				for _, value := range full {
					if value.Type().ID() == dtype.STRT && !value.IsNull() {
						cost = saturatingAdd(cost, int64(len(value.text)))
					}
				}
				if cost > (budget-s.charged-s.keys.charged-outputCharge)/3 {
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
			s.matched[rightIndex] = true
			if spec.kind == JoinAnti {
				continue
			}
			if spec.kind == JoinSemi {
				break
			}
			if !appendRow(full) {
				return nil, ExecutionResult{code: ExecutionResourceExhausted}
			}
		}
		if spec.kind == JoinSemi || spec.kind == JoinAnti {
			if (spec.kind == JoinSemi && matched) || (spec.kind == JoinAnti && !matched) {
				left := make([]Scalar, spec.leftColumns)
				for column := range left {
					left[column] = scalarAt(batch.VectorAt(column), row)
				}
				if !appendRow(left) {
					return nil, ExecutionResult{code: ExecutionResourceExhausted}
				}
			}
		} else if !matched && (spec.kind == JoinLeft || spec.kind == JoinFull) {
			full := make([]Scalar, 0, spec.leftColumns+spec.rightColumns)
			for column := range spec.leftColumns {
				full = append(full, scalarAt(batch.VectorAt(column), row))
			}
			for column := range spec.rightColumns {
				full = append(full, MakeNullScalar(s.rightRowsType(spec, column)))
			}
			if !appendRow(full) {
				return nil, ExecutionResult{code: ExecutionResourceExhausted}
			}
		}
	}
	if len(rows) == 0 {
		return nil, ExecutionResult{code: ExecutionCompleted}
	}
	output := makeDistinctBatch(a, step.schema, rows)
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
	rows := make([]distinctRow, 0)
	var outputCharge int64
	for index, record := range s.rightRows {
		if s.matched[index] {
			continue
		}
		values := make([]Scalar, 0, spec.leftColumns+spec.rightColumns)
		for column := range spec.leftColumns {
			values = append(values, MakeNullScalar(step.schema.FieldAt(column).Type()))
		}
		values = append(values, record.values...)
		cost := int64(len(values) * 64)
		for _, value := range values {
			if value.Type().ID() == dtype.STRT && !value.IsNull() {
				cost = saturatingAdd(cost, int64(len(value.text)))
			}
		}
		if cost > (budget-s.charged-s.keys.charged)/2-outputCharge {
			return nil, false
		}
		outputCharge += cost
		rows = append(rows, distinctRow{values: values})
	}
	if len(rows) == 0 {
		return nil, true
	}
	return makeDistinctBatch(a, step.schema, rows), true
}
