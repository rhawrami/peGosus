package plan

import "github.com/rhawrami/peGosus/pkg/dtype"

func selectGroupDictionaries(source *scanSource, steps []physicalStep) {
	if source == nil || source.parquetPath == "" {
		return
	}
	for _, step := range steps {
		if step.operation == physicalFilter {
			continue
		}
		if step.operation != physicalAggregate || makeCompactGroupState(step) == nil || step.schema.FieldAt(0).Type().ID() != dtype.STRT {
			return
		}
		program := step.groupKeys[0]
		root := program.nodes[program.roots[0]]
		if root.kind == exprColumn {
			column := root.column
			if source.projection != nil {
				column = source.projection[column]
			}
			source.parquetDictionaries = []int{column}
		}
		return
	}
}

func foldAggregateProjection(steps []physicalStep) []physicalStep {
	for at := len(steps) - 1; at > 0; at-- {
		if steps[at].operation != physicalAggregate || steps[at-1].operation != physicalProject {
			continue
		}
		projection := steps[at-1].program
		columns := make([]int, len(projection.roots))
		plain := true
		for i, root := range projection.roots {
			if projection.nodes[root].kind != exprColumn {
				plain = false
				break
			}
			columns[i] = projection.nodes[root].column
		}
		if !plain {
			continue
		}
		remap := func(program *physicalExprProgram) {
			for i := range program.nodes {
				if program.nodes[i].kind == exprColumn {
					program.nodes[i].column = columns[program.nodes[i].column]
				}
			}
		}
		for i := range steps[at].groupKeys {
			remap(&steps[at].groupKeys[i])
		}
		for i := range steps[at].aggregates {
			remap(&steps[at].aggregates[i].program)
		}
		copy(steps[at-1:], steps[at:])
		steps = steps[:len(steps)-1]
	}
	return steps
}

func pruneScanProjection(source *scanSource, steps []physicalStep) {
	if source == nil {
		return
	}
	if source.parquetPath != "" {
		for i, step := range steps {
			if step.operation != physicalFilter {
				break
			}
			predicates, residual, handled := splitParquetScanPredicates(step.program)
			source.parquetFilterHandled = append(source.parquetFilterHandled, handled)
			source.parquetPredicates = append(source.parquetPredicates, predicates...)
			if !handled {
				steps[i].program = residual
			}
		}
	}
	needed := make([]bool, source.schema.Len())
	boundary := -1
	mark := func(program physicalExprProgram) {
		for _, node := range program.nodes {
			if node.kind == exprColumn && node.column >= 0 && node.column < len(needed) {
				needed[node.column] = true
			}
		}
	}
	for i, step := range steps {
		switch step.operation {
		case physicalLimit:
			continue
		case physicalFilter, physicalProject:
			if i >= len(source.parquetFilterHandled) || !source.parquetFilterHandled[i] {
				mark(step.program)
			}
			if step.operation == physicalProject {
				boundary = i
			}
		case physicalAggregate:
			for _, program := range step.groupKeys {
				mark(program)
			}
			for _, aggregate := range step.aggregates {
				mark(aggregate.program)
			}
			boundary = i
		default:
			return
		}
		if boundary >= 0 {
			break
		}
	}
	if boundary < 0 {
		return
	}
	selected := make([]int, 0, len(needed))
	for i, keep := range needed {
		if keep {
			selected = append(selected, i)
		}
	}
	if len(selected) == 0 && source.parquetPath == "" {
		selected = append(selected, 0)
	}
	if len(selected) == len(needed) {
		return
	}
	remap := make([]int, len(needed))
	for i, old := range selected {
		remap[old] = i
	}
	remapProgram := func(program *physicalExprProgram) {
		for j := range program.nodes {
			if program.nodes[j].kind == exprColumn {
				program.nodes[j].column = remap[program.nodes[j].column]
			}
		}
	}
	for i := 0; i <= boundary; i++ {
		if i < len(source.parquetFilterHandled) && source.parquetFilterHandled[i] {
			continue
		}
		if steps[i].operation == physicalAggregate {
			for j := range steps[i].groupKeys {
				remapProgram(&steps[i].groupKeys[j])
			}
			for j := range steps[i].aggregates {
				remapProgram(&steps[i].aggregates[j].program)
			}
		} else {
			remapProgram(&steps[i].program)
		}
	}
	source.projection = selected
}

func pushScanFilters(source *scanSource, steps []physicalStep) {
	if source == nil {
		return
	}
	for i := range steps {
		if steps[i].operation != physicalFilter {
			break
		}
		source.filters = append(source.filters, steps[i].program)
		steps[i].operation = physicalPushedFilter
	}
}
