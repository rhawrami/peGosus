package parquet

import "github.com/rhawrami/peGosus/pkg/mem"

// RowGroupCount returns the number of row groups described by the file footer.
func (r *ParquetReader) RowGroupCount() int {
	if r == nil {
		return 0
	}
	return len(r.groups)
}

// RowGroupRows returns a row group's logical row count, or zero for an invalid index.
func (r *ParquetReader) RowGroupRows(i int) int64 {
	if r == nil || i < 0 || i >= len(r.groups) {
		return 0
	}
	return r.groups[i].rows
}

// MakeRowGroupReader returns an owner-confined cursor for one row group. The
// input and immutable footer metadata are borrowed from r; the supplied
// allocator owns this cursor's pages and output batches. The input must stay
// open until all row-group readers finish.
func (r *ParquetReader) MakeRowGroupReader(i int, a *mem.Allocator) *ParquetReader {
	if r == nil || r.closed || a == nil || i < 0 || i >= len(r.groups) || r.group != 0 || r.row != 0 || r.cursors != nil {
		return nil
	}
	return &ParquetReader{
		input: r.input, allocator: a, options: r.options,
		columns: r.columns, groups: r.groups[i : i+1], selected: r.selected,
		groupBase: r.groupBase + i, pruning: r.pruning, predicates: r.predicates,
		dictionaryColumns: r.dictionaryColumns,
		runtimeFilter:     r.runtimeFilter,
	}
}
