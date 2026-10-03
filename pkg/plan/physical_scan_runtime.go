package plan

import (
	"context"
	"math"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
)

func runtimeIntegerColumn(program physicalExprProgram, root int) int {
	node := program.nodes[root]
	if node.kind == exprCast && node.dType.ID() == dtype.INT64T {
		child := program.nodes[node.children[0]]
		if child.kind == exprColumn && child.dType.ID() == dtype.INT32T {
			node = child
		}
	}
	if node.kind != exprColumn {
		return -1
	}
	switch node.dType.ID() {
	case dtype.INT32T, dtype.INT64T, dtype.DATET, dtype.TIMESTAMPTZT:
		return node.column
	}
	return -1
}

func runtimeScanColumn(source *scanSource, steps []physicalStep, at, column int) int {
	if source == nil || source.parquetPath == "" {
		return -1
	}
	for i := at - 1; i >= 0; i-- {
		switch steps[i].operation {
		case physicalFilter, physicalPushedFilter:
		case physicalProject:
			program := steps[i].program
			if column < 0 || column >= len(program.roots) {
				return -1
			}
			column = runtimeIntegerColumn(program, program.roots[column])
			if column < 0 {
				return -1
			}
		default:
			return -1
		}
	}
	if len(source.projection) != 0 {
		if column < 0 || column >= len(source.projection) {
			return -1
		}
		column = source.projection[column]
	}
	if column < 0 || column >= source.schema.Len() {
		return -1
	}
	return column
}

func makeTopNRuntimeFilter(source *scanSource, steps []physicalStep) (*parquet.RuntimePruningFilter, int) {
	for i, step := range steps {
		if step.operation != physicalSort || makeCompactTopNState(step) == nil || step.topN <= 0 {
			continue
		}
		program := step.order[0].program
		keyColumn := runtimeIntegerColumn(program, program.roots[0])
		if keyColumn < 0 {
			return nil, -1
		}
		column := runtimeScanColumn(source, steps, i, keyColumn)
		if column >= 0 {
			return parquet.MakeRuntimePruningFilter(column, step.order[0].nullsFirst), i
		}
		return nil, -1
	}
	return nil, -1
}

func publishTopNRuntimeFilter(filter *parquet.RuntimePruningFilter, top *compactTopNState, step physicalStep) {
	if filter == nil || top == nil || top.limit == 0 || top.length != top.limit || top.slice()[0].null != 0 {
		return
	}
	key := top.slice()[0].key
	if step.order[0].descending {
		key = ^key
	}
	var value int64
	switch step.order[0].program.nodes[step.order[0].program.roots[0]].dType.ID() {
	case dtype.INT32T, dtype.DATET:
		value = int64(int32(uint32(key) ^ 0x80000000))
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		value = int64(key ^ 0x8000000000000000)
	default:
		return
	}
	// Inclusive bounds preserve primary-key ties for later ordering keys.
	if step.order[0].descending {
		filter.UpdateLowerBound(value)
	} else {
		filter.UpdateUpperBound(value)
	}
}

func makeJoinRuntimeFilter(ctx context.Context, source *scanSource, steps []physicalStep, at int, state *joinState) (*parquet.RuntimePruningFilter, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if at < 0 || at >= len(steps) || state == nil {
		return nil, nil
	}
	spec := steps[at].join
	if spec == nil || spec.kind != JoinInner && spec.kind != JoinSemi || len(spec.leftKeys) == 0 {
		return nil, nil
	}
	program := spec.leftKeys[0]
	node := program.nodes[program.roots[0]]
	keyColumn := runtimeIntegerColumn(program, program.roots[0])
	if keyColumn < 0 {
		return nil, nil
	}
	column := runtimeScanColumn(source, steps, at, keyColumn)
	if column < 0 {
		return nil, nil
	}
	lower, upper := int64(math.MaxInt64), int64(math.MinInt64)
	for row := range state.keys.rows.length {
		if row&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		key := state.keys.rows.at(row, 0)
		value := key.i64()
		if node.dType.ID() == dtype.INT32T || node.dType.ID() == dtype.DATET {
			value = int64(key.i32())
		}
		lower, upper = min(lower, value), max(upper, value)
	}
	filter := parquet.MakeRuntimePruningFilter(column, false)
	filter.UpdateLowerBound(lower)
	filter.UpdateUpperBound(upper)
	return filter, nil
}
