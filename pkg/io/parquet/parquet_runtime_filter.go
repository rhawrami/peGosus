package parquet

import (
	"math"
	"sync/atomic"

	"github.com/rhawrami/peGosus/pkg/dtype"
)

// MakeRuntimePruningFilter returns shared conservative bounds for one signed
// integer file column. Bounds initially include every value. allowNull keeps
// groups that might contain nulls, independently of their non-null bounds.
func MakeRuntimePruningFilter(column int, allowNull bool) *RuntimePruningFilter {
	f := &RuntimePruningFilter{column: column, allowNull: allowNull}
	f.lower.Store(math.MinInt64)
	f.upper.Store(math.MaxInt64)
	return f
}

// RuntimePruningFilter shares monotonically tightening integer bounds between
// producers and owner-confined readers. Publish bounds only when every rejected
// value is proven unable to contribute to the query result.
type RuntimePruningFilter struct {
	column    int
	allowNull bool
	lower     atomic.Int64
	upper     atomic.Int64
	active    atomic.Bool
	pruned    atomic.Int64
}

// UpdateLowerBound retains values greater than or equal to value. Concurrent
// updates retain the greatest bound; bounds cannot be relaxed.
func (f *RuntimePruningFilter) UpdateLowerBound(value int64) {
	if f == nil {
		return
	}
	for prior := f.lower.Load(); value > prior; prior = f.lower.Load() {
		if f.lower.CompareAndSwap(prior, value) {
			f.active.Store(true)
			return
		}
	}
	f.active.Store(true)
}

// UpdateUpperBound retains values less than or equal to value. Concurrent
// updates retain the least bound; bounds cannot be relaxed.
func (f *RuntimePruningFilter) UpdateUpperBound(value int64) {
	if f == nil {
		return
	}
	for prior := f.upper.Load(); value < prior; prior = f.upper.Load() {
		if f.upper.CompareAndSwap(prior, value) {
			f.active.Store(true)
			return
		}
	}
	f.active.Store(true)
}

// PrunedRowGroups returns the number of unread row groups rejected by all
// readers sharing this filter.
func (f *RuntimePruningFilter) PrunedRowGroups() int64 {
	if f == nil {
		return 0
	}
	return f.pruned.Load()
}

// SetRuntimePruningFilter attaches a shared filter before this reader starts.
// The filter is inherited by row-group readers, which recheck it before opening
// column chunks. Reading already-open groups continues unchanged.
func (r *ParquetReader) SetRuntimePruningFilter(f *RuntimePruningFilter) bool {
	if r == nil || r.closed || r.group != 0 || r.row != 0 || r.cursors != nil {
		return false
	}
	if f != nil {
		if f.column < 0 || f.column >= len(r.columns) {
			return false
		}
		switch r.columns[f.column].typ.ID() {
		case dtype.INT32T, dtype.INT64T, dtype.DATET, dtype.TIMESTAMPTZT:
		default:
			return false
		}
	}
	r.runtimeFilter = f
	return true
}

func (f *RuntimePruningFilter) cannotMatch(chunk parquetChunk) bool {
	if !f.active.Load() {
		return false
	}
	if f.allowNull && !chunk.noNull {
		return false
	}
	if !f.allowNull && chunk.allNull {
		return true
	}
	lower, upper := f.lower.Load(), f.upper.Load()
	return lower > upper || chunk.hasMinMax && (chunk.max < lower || chunk.min > upper)
}
