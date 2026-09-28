package plan

func pruneScanProjection(source *scanSource, steps []physicalStep) {
	if source == nil || source.schema.Len() < 2 {
		return
	}
	needed := make([]bool, source.schema.Len())
	projectAt := -1
	for i, step := range steps {
		switch step.operation {
		case physicalLimit:
			continue
		case physicalFilter, physicalProject:
			for _, node := range step.program.nodes {
				if node.kind == exprColumn && node.column >= 0 && node.column < len(needed) {
					needed[node.column] = true
				}
			}
			if step.operation == physicalProject {
				projectAt = i
			}
		default:
			return
		}
		if projectAt >= 0 {
			break
		}
	}
	if projectAt < 0 {
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
	for i := 0; i <= projectAt; i++ {
		for j := range steps[i].program.nodes {
			if steps[i].program.nodes[j].kind == exprColumn {
				old := steps[i].program.nodes[j].column
				steps[i].program.nodes[j].column = remap[old]
			}
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
