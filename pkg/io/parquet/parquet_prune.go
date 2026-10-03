package parquet

import "github.com/rhawrami/peGosus/pkg/dtype"

// PruningOp identifies a signed-integer comparison against a literal.
type PruningOp uint8

const (
	PruneEqual PruningOp = iota + 1
	PruneNotEqual
	PruneLess
	PruneLessEqual
	PruneGreater
	PruneGreaterEqual
)

// PruningPredicate describes one conjunctive scan predicate. Column is the
// zero-based file column index; Literal is a signed INT32 or INT64 value.
// The executor still evaluates the full predicate on every emitted batch.
type PruningPredicate struct {
	Column  int
	Op      PruningOp
	Literal int64
}

// SetPruningPredicates installs conservative row-group pruning before the
// first Next call. Invalid or unsupported predicates are ignored. It returns
// false if reading has already begun or the reader has been closed.
func (r *ParquetReader) SetPruningPredicates(predicates []PruningPredicate) bool {
	if r == nil || r.closed || r.group != 0 || r.row != 0 || r.cursors != nil {
		return false
	}
	r.pruning = r.pruning[:0]
	for _, predicate := range predicates {
		if predicate.Column < 0 || predicate.Column >= len(r.columns) || predicate.Op < PruneEqual || predicate.Op > PruneGreaterEqual {
			continue
		}
		switch r.columns[predicate.Column].typ.ID() {
		case dtype.INT32T, dtype.INT64T, dtype.DATET, dtype.TIMESTAMPTZT:
			r.pruning = append(r.pruning, predicate)
		}
	}
	return true
}

func (r *ParquetReader) groupCannotMatch(group parquetGroup) bool {
	if r.runtimeFilter != nil && r.runtimeFilter.cannotMatch(group.chunks[r.runtimeFilter.column]) {
		r.runtimeFilter.pruned.Add(1)
		return true
	}
	for _, predicate := range r.pruning {
		chunk := group.chunks[predicate.Column]
		if chunk.allNull {
			return true
		}
		if !chunk.hasMinMax {
			continue
		}
		value := predicate.Literal
		switch predicate.Op {
		case PruneEqual:
			if value < chunk.min || value > chunk.max {
				return true
			}
		case PruneNotEqual:
			if chunk.min == value && chunk.max == value {
				return true
			}
		case PruneLess:
			if chunk.min >= value {
				return true
			}
		case PruneLessEqual:
			if chunk.min > value {
				return true
			}
		case PruneGreater:
			if chunk.max <= value {
				return true
			}
		case PruneGreaterEqual:
			if chunk.max < value {
				return true
			}
		}
	}
	return false
}
