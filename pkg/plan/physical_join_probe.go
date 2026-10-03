package plan

import (
	"context"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (s *joinState) probe(ctx context.Context, a *mem.Allocator, batch *store.Batch, step physicalStep, budget int64, emit func(*store.Batch) ExecutionResult) ExecutionResult {
	spec := step.join
	keyBatch, values := makeJoinKeyBatch(a, batch, spec.leftKeys)
	if keyBatch == nil {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	defer keyBatch.Release()
	defer func() {
		for i := range values {
			values[i].release()
		}
	}()
	selection := batch.Selection().RetainBitMap(a)
	if batch.Selection() != nil && selection == nil {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	if selection != nil {
		defer selection.Release()
	}
	matched := store.MakeBitMapTemp(a, batch.Len())
	if matched == nil {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	defer matched.Release()
	matched.ClearAll()
	remaining := budget - s.charged - s.keys.charged - int64(len(matched.Bytes()))
	capacity := int(min(int64(joinOutputBatchRows), remaining/(16+int64(spec.leftColumns+spec.rightColumns)*64*3)))
	if capacity < 1 {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	pairSegment := a.AllocSegTemp(capacity * 16)
	if pairSegment == nil {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	defer pairSegment.Dec()
	pairs := pairSegment.AsU64T()
	remaining -= int64(pairSegment.Len())
	lookup := distinctState{index: s.keys.index, keys: s.keys.keys, scope: s.scope}
	defer func() {
		if lookup.scratch != nil {
			lookup.scratch.Dec()
		}
	}()
	candidateSchema := step.schema
	if spec.residual != nil && (spec.kind == JoinSemi || spec.kind == JoinAnti) {
		fields := append([]Field(nil), step.schema.fields...)
		fields = append(fields, spec.right.schema.fields...)
		candidateSchema = Schema{valid: true, fields: fields}
	}
	length := 0
	flush := func(candidate bool) ExecutionResult {
		if length == 0 {
			return ExecutionResult{code: ExecutionCompleted}
		}
		if err := ctx.Err(); err != nil {
			return ExecutionResult{code: ExecutionCancelled, cause: err}
		}
		var output *store.Batch
		if !candidate || spec.residual != nil || spec.kind != JoinSemi && spec.kind != JoinAnti {
			schema := step.schema
			if candidate {
				schema = candidateSchema
			}
			output = s.gather(ctx, a, batch, schema, spec.leftColumns, pairs[:length*2], remaining-lookup.charged)
			if output == nil {
				if err := ctx.Err(); err != nil {
					return ExecutionResult{code: ExecutionCancelled, cause: err}
				}
				return ExecutionResult{code: ExecutionResourceExhausted}
			}
			if candidate && spec.residual != nil && !executePhysicalFilter(a, output, *spec.residual) {
				output.Release()
				return ExecutionResult{code: ExecutionResourceExhausted}
			}
		}
		if candidate {
			var mask *store.BitMap
			if output != nil && output.Selection() != nil {
				mask, _ = output.Selection().AsBitMap()
			}
			for i := range length {
				if mask != nil && !mask.IsSet(i) {
					continue
				}
				matched.Set(int(pairs[i*2]))
				if s.matched != nil {
					s.matched.AsBytes()[pairs[i*2+1]] = 1
				}
			}
		}
		length = 0
		if output == nil {
			return ExecutionResult{code: ExecutionCompleted}
		}
		if candidate && (spec.kind == JoinSemi || spec.kind == JoinAnti) || output.ActiveLen() == 0 {
			output.Release()
			return ExecutionResult{code: ExecutionCompleted}
		}
		return emit(output)
	}
	for row := range batch.Len() {
		if row&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return ExecutionResult{code: ExecutionCancelled, cause: err}
			}
		}
		if selection != nil && !selection.IsSet(row) || !joinableKeys(keyBatch, row) {
			continue
		}
		var index int
		var found bool
		if lookup.index.stringKeys {
			index, found = lookup.index.lookupString(keyBatch.VectorAt(0).StringAt(row).View(), false, &lookup.keys)
		} else {
			index, found = lookup.lookup(a, keyBatch, row)
		}
		if !found {
			if s.scope != nil && s.scope.Exhausted() {
				return ExecutionResult{code: ExecutionResourceExhausted}
			}
			continue
		}
		for candidate := s.head.AsU64T()[index]; candidate != 0; {
			right := candidate - 1
			candidate = s.next.AsU64T()[right]
			pairs[length*2], pairs[length*2+1] = uint64(row), right
			length++
			if length == capacity {
				if result := flush(true); result.code != ExecutionCompleted {
					return result
				}
			}
			if (spec.kind == JoinSemi || spec.kind == JoinAnti) && (spec.residual == nil || matched.IsSet(row)) {
				break
			}
		}
	}
	if result := flush(true); result.code != ExecutionCompleted {
		return result
	}
	if spec.kind != JoinInner {
		for row := range batch.Len() {
			if selection != nil && !selection.IsSet(row) {
				continue
			}
			pass := matched.IsSet(row)
			if spec.kind == JoinSemi && !pass || spec.kind != JoinSemi && pass {
				continue
			}
			pairs[length*2], pairs[length*2+1] = uint64(row), joinNullRow
			length++
			if length == capacity {
				if result := flush(false); result.code != ExecutionCompleted {
					return result
				}
			}
		}
	}
	return flush(false)
}

func (s *joinState) finish(ctx context.Context, a *mem.Allocator, step physicalStep, budget int64, emit func(*store.Batch) ExecutionResult) ExecutionResult {
	if step.join.kind != JoinFull || s.length == 0 {
		return ExecutionResult{code: ExecutionCompleted}
	}
	remaining := budget - s.charged - s.keys.charged
	capacity := int(min(int64(joinOutputBatchRows), remaining/(16+int64(step.schema.Len())*64)))
	if capacity < 1 {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	segment := a.AllocSegTemp(capacity * 16)
	if segment == nil {
		return ExecutionResult{code: ExecutionResourceExhausted}
	}
	defer segment.Dec()
	remaining -= int64(segment.Len())
	pairs, length := segment.AsU64T(), 0
	flush := func() ExecutionResult {
		if length == 0 {
			return ExecutionResult{code: ExecutionCompleted}
		}
		output := s.gather(ctx, a, nil, step.schema, step.join.leftColumns, pairs[:length*2], remaining)
		if output == nil {
			if err := ctx.Err(); err != nil {
				return ExecutionResult{code: ExecutionCancelled, cause: err}
			}
			return ExecutionResult{code: ExecutionResourceExhausted}
		}
		length = 0
		return emit(output)
	}
	for row := range s.length {
		if row&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return ExecutionResult{code: ExecutionCancelled, cause: err}
			}
		}
		if s.matched.AsBytes()[row] != 0 {
			continue
		}
		pairs[length*2], pairs[length*2+1] = joinNullRow, uint64(row)
		length++
		if length == capacity {
			if result := flush(); result.code != ExecutionCompleted {
				return result
			}
		}
	}
	return flush()
}
