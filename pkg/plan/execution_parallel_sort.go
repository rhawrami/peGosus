package plan

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/rhawrami/peGosus/pkg/mem"
)

const (
	parallelSortMinRows       = 32 << 10
	parallelSortRowsPerWorker = 16 << 10
)

func compactSortWorkers(requested int) int {
	if requested == 1 {
		return 1
	}
	workers := runtime.GOMAXPROCS(0)
	if requested > 1 {
		return min(workers, requested)
	}
	return min(workers, 8)
}

type parallelSortMerge struct {
	left, middle, right int
	start, end          int
	ordered             bool
}

func (s *compactSortState) sortParallel(a *mem.Allocator, step physicalStep, source, destination []uint64) ([]uint64, []uint64, bool) {
	workers := min(s.workers, max(2, s.length/parallelSortRowsPerWorker))
	boundaries := make([]int, workers+1)
	for worker := range workers + 1 {
		boundaries[worker] = worker*(s.length/workers) + min(worker, s.length%workers)
	}
	var failed atomic.Bool
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			start, end := boundaries[worker], boundaries[worker+1]
			local := *s
			local.offset, local.length, local.workers = start, end-start, 1
			src, dst := source[start:end], destination[start:end]
			for i := range src {
				src[i] = uint64(i)
			}
			sorted, _, ok := local.sortItems(a, step, src, dst)
			if !ok {
				failed.Store(true)
				return
			}
			if &sorted[0] != &src[0] {
				copy(src, sorted)
			}
			for i := range src {
				src[i] += uint64(start)
			}
		}(worker)
	}
	wg.Wait()
	if failed.Load() || s.cancelled() {
		return nil, nil, false
	}
	var prefixes []uint64
	if s.keys[0].stringKey {
		cache := s.stringPrefixes(a, &s.keys[0], step.order[0])
		if cache != nil {
			defer cache.Dec()
			prefixes = cache.AsU64T()
		}
		if s.cancelled() {
			return nil, nil, false
		}
	}
	less := func(left, right uint64) bool {
		return s.lessItem(step, prefixes, left, right)
	}
	for len(boundaries) > 2 {
		pairs := (len(boundaries) - 1) / 2
		partitions := max(1, workers/max(1, pairs))
		jobs := make([]parallelSortMerge, 0, workers+1)
		nextBoundaries := make([]int, 1, (len(boundaries)+1)/2)
		for run := 0; run+2 < len(boundaries); run += 2 {
			left, middle, right := boundaries[run], boundaries[run+1], boundaries[run+2]
			ordered := !less(source[middle], source[middle-1])
			for partition := range partitions {
				start := partition*((right-left)/partitions) + min(partition, (right-left)%partitions)
				end := (partition+1)*((right-left)/partitions) + min(partition+1, (right-left)%partitions)
				jobs = append(jobs, parallelSortMerge{left, middle, right, start, end, ordered})
			}
			nextBoundaries = append(nextBoundaries, right)
		}
		if (len(boundaries)-1)&1 != 0 {
			left, right := boundaries[len(boundaries)-2], boundaries[len(boundaries)-1]
			copy(destination[left:right], source[left:right])
			nextBoundaries = append(nextBoundaries, right)
		}
		var next atomic.Int64
		for range min(workers, len(jobs)) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					at := int(next.Add(1) - 1)
					if at >= len(jobs) || s.cancelled() {
						return
					}
					job := jobs[at]
					if job.ordered {
						copy(destination[job.left+job.start:job.left+job.end], source[job.left+job.start:job.left+job.end])
						continue
					}
					left, right := source[job.left:job.middle], source[job.middle:job.right]
					l, r := parallelSortMergePath(left, right, job.start, less)
					for on := job.start; on < job.end; on++ {
						if on&65535 == 0 && s.cancelled() {
							return
						}
						if l < len(left) && (r == len(right) || !less(right[r], left[l])) {
							destination[job.left+on] = left[l]
							l++
						} else {
							destination[job.left+on] = right[r]
							r++
						}
					}
				}
			}()
		}
		wg.Wait()
		if s.cancelled() {
			return nil, nil, false
		}
		source, destination = destination, source
		boundaries = nextBoundaries
	}
	return source, destination, true
}

func parallelSortMergePath(left, right []uint64, diagonal int, less func(uint64, uint64) bool) (int, int) {
	low, high := max(0, diagonal-len(right)), min(diagonal, len(left))
	for low <= high {
		l := low + (high-low)/2
		r := diagonal - l
		if l > 0 && r < len(right) && less(right[r], left[l-1]) {
			high = l - 1
		} else if r > 0 && l < len(left) && !less(right[r-1], left[l]) {
			low = l + 1
		} else {
			return l, r
		}
	}
	return low, diagonal - low
}

func (s *compactSortState) lessItem(step physicalStep, prefixes []uint64, left, right uint64) bool {
	if prefixes != nil {
		if prefixes[left*2] != prefixes[right*2] {
			return prefixes[left*2] < prefixes[right*2]
		}
		if prefixes[left*2+1] != prefixes[right*2+1] {
			return prefixes[left*2+1] < prefixes[right*2+1]
		}
	}
	for k, order := range step.order {
		key := &s.keys[k]
		if !key.stringKey {
			values := key.values.AsU64T()
			ln, rn := values[left*2+1] != 0, values[right*2+1] != 0
			if ln != rn {
				return ln == order.nullsFirst
			}
			if !ln && values[left*2] != values[right*2] {
				return values[left*2] < values[right*2]
			}
			continue
		}
		rows := s.rows.AsU64T()
		l, r := rows[left], rows[right]
		lv, rv := &key.vectors[l>>32], &key.vectors[r>>32]
		li, ri := int(uint32(l)), int(uint32(r))
		ln, rn := lv.Validity() != nil && !lv.Validity().IsSet(li), rv.Validity() != nil && !rv.Validity().IsSet(ri)
		if ln != rn {
			return ln == order.nullsFirst
		}
		if ln {
			continue
		}
		lt, rt := lv.StringAt(li).View(), rv.StringAt(ri).View()
		if lt != rt {
			if order.descending {
				return lt > rt
			}
			return lt < rt
		}
	}
	// Compact IDs preserve source order across independently sorted runs.
	return left < right
}
