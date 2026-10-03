package plan

import "github.com/rhawrami/peGosus/pkg/io/parquet"

func (p *PhysicalPlan) parallelJoinSequences(tasks int, reader *parquet.ParquetReader, joins []int, builds []*joinState) ([]uint64, []uint64, bool) {
	taskSequences := make([]uint64, tasks)
	finalSequences := make([]uint64, len(p.steps))
	var sequence uint64
	reserve := func(rows uint64, after int) bool {
		for _, at := range joins {
			if at <= after || p.steps[at].join.kind == JoinSemi || p.steps[at].join.kind == JoinAnti {
				continue
			}
			fanout := uint64(max(1, builds[at].length))
			if rows > ^uint64(0)/fanout {
				return false
			}
			rows *= fanout
		}
		if rows > ^uint64(0)-sequence {
			return false
		}
		sequence += rows
		return true
	}
	// Source tasks reserve disjoint ranges even when joins expand each row.
	for task := range tasks {
		taskSequences[task] = sequence
		var rows uint64
		if p.source.table != nil {
			rows = uint64(p.source.table.BatchAt(task).Len())
		} else {
			rows = uint64(reader.RowGroupRows(task))
		}
		if !reserve(rows, -1) {
			return nil, nil, false
		}
	}
	// Full-join unmatched tails follow source tasks and earlier join tails.
	for _, at := range joins {
		finalSequences[at] = sequence
		if p.steps[at].join.kind == JoinFull && !reserve(uint64(builds[at].length), at) {
			return nil, nil, false
		}
	}
	return taskSequences, finalSequences, true
}
