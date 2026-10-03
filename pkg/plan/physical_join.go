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
	batches     []*store.Batch
	rows        *mem.Segment
	length      int
	scope       *mem.AllocationScope
	head        *mem.Segment
	next        *mem.Segment
	matched     *mem.Segment
	charged     int64
	initialized bool
}

func (s *joinState) release() {
	s.keys.release()
	for _, batch := range s.batches {
		batch.Release()
	}
	for _, segment := range []*mem.Segment{s.head, s.next, s.matched, s.rows} {
		if segment != nil {
			segment.Dec()
		}
	}
	*s = joinState{}
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
	if len(spec.rightKeys) == 1 && spec.rightKeys[0].nodes[spec.rightKeys[0].roots[0]].dType.ID() == dtype.STRT {
		s.keys.index.stringKeys = true
	}
	result := spec.right.executeWithScope(ctx, a, ExecutionOptions{MemoryBudget: budget, Workers: 1}, func(batch *store.Batch) bool {
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
		selection := batch.Selection().RetainBitMap(a)
		if batch.Selection() != nil && selection == nil {
			return false
		}
		if selection != nil {
			defer selection.Release()
		}
		if batch.ActiveLen() == 0 {
			return true
		}
		if uint64(len(s.batches)) >= math.MaxUint32 || uint64(batch.Len()) > math.MaxUint32 || batch.ActiveLen() > int(^uint(0)>>1)-s.length {
			return false
		}
		var retainedBytes int64
		for column := range batch.NVectors() {
			retainedBytes = saturatingAdd(retainedBytes, sortVectorBytes(batch.VectorAt(column)))
		}
		if batch.Selection() != nil {
			retainedBytes = saturatingAdd(retainedBytes, int64(batch.Len())*4)
		}
		if retainedBytes > budget-s.charged-s.keys.charged {
			return false
		}
		batchIndex := uint64(len(s.batches))
		s.batches = append(s.batches, batch.Retain())
		s.charged = saturatingAdd(s.charged, retainedBytes)
		for row := range batch.Len() {
			if row&1023 == 0 && ctx.Err() != nil {
				return false
			}
			if selection != nil && !selection.IsSet(row) {
				continue
			}
			index := -1
			if joinableKeys(keyBatch, row) {
				var ok bool
				if s.keys.index.stringKeys {
					index, ok = s.keys.addString(a, keyBatch, row, budget-s.charged)
				} else {
					index, ok = s.keys.add(a, keyBatch, row, budget-s.charged)
				}
				if !ok {
					return false
				}
				if !s.growArray(a, &s.head, index+1, 8, budget) {
					return false
				}
			}
			if !s.growArray(a, &s.next, s.length+1, 8, budget) || !s.growArray(a, &s.rows, s.length+1, 8, budget) {
				return false
			}
			if spec.kind == JoinFull && !s.growArray(a, &s.matched, s.length+1, 1, budget) {
				return false
			}
			rightIndex := s.length
			s.rows.AsU64T()[rightIndex] = batchIndex<<32 | uint64(row)
			s.length++
			if index >= 0 {
				heads := s.head.AsU64T()
				s.next.AsU64T()[rightIndex] = heads[index]
				heads[index] = uint64(rightIndex) + 1
			}
		}
		return true
	}, s.scope)
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}
	}
	if result.Code() == ExecutionStopped {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	return result
}
