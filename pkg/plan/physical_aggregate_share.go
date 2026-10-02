package plan

import "slices"

func shareGlobalAggregates(steps []physicalStep) {
	for at := range steps {
		step := &steps[at]
		if step.operation != physicalAggregate || len(step.groupKeys) != 0 {
			continue
		}
		sources := make([]int, len(step.aggregates))
		shared := false
		for i, aggregate := range step.aggregates {
			sources[i] = i
			for j := 0; j < i; j++ {
				other := step.aggregates[j]
				if sources[j] != j || aggregate.kind != other.kind || aggregate.distinct != other.distinct {
					continue
				}
				left, right := aggregate.program, other.program
				if left.outputRoots == right.outputRoots && slices.Equal(left.nodes, right.nodes) && slices.Equal(left.roots, right.roots) && slices.Equal(left.materialize, right.materialize) {
					sources[i], shared = j, true
					break
				}
			}
		}
		if shared {
			step.aggregateSources = sources
		}
	}
}
