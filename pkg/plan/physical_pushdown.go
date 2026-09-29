package plan

func pruneScanProjection(source *scanSource, steps []physicalStep) {
	if source == nil || source.schema.Len() < 2 {
		return
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
			mark(step.program)
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
	var selected []int
	for i, keep := range needed {
		if keep {
			selected = append(selected, i)
		}
	}
	if len(selected) == 0 {
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
