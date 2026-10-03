package plan

import (
	"context"
	"hash/maphash"
	"sync"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (p *PhysicalPlan) executeGroupedShards(ctx context.Context, a *mem.Allocator, scope *mem.AllocationScope, step physicalStep, sources []*groupState, budget, rows int64, sink func(*store.Batch) bool) (ExecutionResult, bool) {
	if rows <= 0 {
		return ExecutionResult{}, false
	}
	shards := 1
	for shards*2 <= len(sources) && shards < 8 {
		shards *= 2
	}
	if shards < 2 {
		return ExecutionResult{}, false
	}
	var groups int64
	var charged int64
	for _, source := range sources {
		if source == nil || source.compact != nil && source.compact.keyType.ID() != dtype.STRT {
			return ExecutionResult{}, false
		}
		groups = saturatingAdd(groups, int64(source.groupCount()))
		charged = saturatingAdd(charged, source.charged(len(step.aggregates)))
	}
	if groups < 4096 || groups < rows-rows/4 || charged >= budget/2 {
		return ExecutionResult{}, false
	}
	perShard := (budget - charged) / int64(shards)
	if perShard < 1<<20 {
		return ExecutionResult{}, false
	}
	if groups > int64(int(^uint(0)>>1)/8) || uint64(len(sources)) > uint64(^uint32(0)) {
		return ExecutionResult{}, false
	}
	for _, source := range sources {
		if uint64(source.groupCount()) > uint64(^uint32(0)) {
			return ExecutionResult{}, false
		}
	}
	reservation := groups * 8
	if reservation > budget-scope.Live() {
		return ExecutionResult{}, false
	}
	indices := a.AllocSegTemp(int(reservation))
	if indices == nil {
		return ExecutionResult{code: ExecutionResourceExhausted}, true
	}
	defer func() {
		if indices != nil {
			indices.Dec()
		}
	}()
	assigned := indices.AsU64T()[:groups]
	seed := maphash.MakeSeed()
	// Count, prefix, then scatter group references so each shard visits only its own keys.
	offsets := make([]int, shards+1)
	for _, source := range sources {
		for row := range source.groupCount() {
			if err := ctx.Err(); err != nil {
				return ExecutionResult{code: ExecutionCancelled, cause: err}, true
			}
			shard := int(maphash.Bytes(seed, source.encodedKey(row)) & uint64(shards-1))
			offsets[shard+1]++
		}
	}
	for i := range shards {
		offsets[i+1] += offsets[i]
	}
	position := append([]int(nil), offsets[:shards]...)
	for sourceIndex, source := range sources {
		for row := range source.groupCount() {
			if row&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return ExecutionResult{code: ExecutionCancelled, cause: err}, true
				}
			}
			shard := int(maphash.Bytes(seed, source.encodedKey(row)) & uint64(shards-1))
			assigned[position[shard]] = uint64(uint32(sourceIndex))<<32 | uint64(uint32(row))
			position[shard]++
		}
	}

	merged := make([]*groupState, shards)
	defer func() {
		for _, state := range merged {
			if state != nil {
				state.release()
			}
		}
	}()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var resultMu sync.Mutex
	result := ExecutionResult{code: ExecutionCompleted}
	var wg sync.WaitGroup
	for shard := range shards {
		merged[shard] = makeGroupState(step, scope)
		wg.Add(1)
		go func(shard int) {
			defer wg.Done()
			target := merged[shard]
			keyValues := make([]Scalar, len(step.groupKeys))
			for _, item := range assigned[offsets[shard]:offsets[shard+1]] {
				if runCtx.Err() != nil {
					return
				}
				source := sources[int(item>>32)]
				if !target.mergeOwnedRow(a, source, step, perShard, int(uint32(item)), keyValues) {
					resultMu.Lock()
					if result.code == ExecutionCompleted {
						result = ExecutionResult{code: ExecutionResourceExhausted}
						cancel()
					}
					resultMu.Unlock()
					return
				}
			}
		}(shard)
	}
	wg.Wait()
	indices.Dec()
	indices = nil
	if result.code != ExecutionCompleted {
		return result, true
	}
	if err := ctx.Err(); err != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: err}, true
	}
	for i, source := range sources {
		source.release()
		sources[i] = nil
	}
	var used int64
	for _, state := range merged {
		used = saturatingAdd(used, state.charged(len(step.aggregates)))
	}
	if used > budget {
		return ExecutionResult{code: ExecutionResourceExhausted}, true
	}
	for i, state := range merged {
		if err := ctx.Err(); err != nil {
			return ExecutionResult{code: ExecutionCancelled, cause: err}, true
		}
		if state.groupCount() == 0 {
			state.release()
			merged[i] = nil
			continue
		}
		if state.charged(len(step.aggregates)) > budget-used {
			return ExecutionResult{code: ExecutionResourceExhausted}, true
		}
		batch := state.finish(a, step)
		if batch == nil {
			if scope.Exhausted() {
				return ExecutionResult{code: ExecutionResourceExhausted}, true
			}
			return ExecutionResult{code: ExecutionFailed}, true
		}
		if err := ctx.Err(); err != nil {
			batch.Release()
			return ExecutionResult{code: ExecutionCancelled, cause: err}, true
		}
		keepGoing := sink(batch)
		batch.Release()
		if !keepGoing {
			return ExecutionResult{code: ExecutionStopped}, true
		}
		used -= state.charged(len(step.aggregates))
		state.release()
		merged[i] = nil
	}
	return ExecutionResult{code: ExecutionCompleted}, true
}
