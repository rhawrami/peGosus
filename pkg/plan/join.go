package plan

// JoinKind identifies a supported equality join.
type JoinKind uint8

const (
	JoinInner JoinKind = iota + 1
	JoinLeft
	JoinFull
	JoinSemi
	JoinAnti
)

// Join returns an equijoin over pairs of left and right key expressions. Both
// inputs must have distinct source aliases for qualified column references.
func (p LogicalPlan) Join(right LogicalPlan, kind JoinKind, leftKeys, rightKeys []Expr, residual ...Expr) LogicalPlan {
	if !p.Valid() || !right.Valid() || kind < JoinInner || kind > JoinAnti || len(leftKeys) == 0 || len(leftKeys) != len(rightKeys) || len(residual) > 1 {
		return LogicalPlan{}
	}
	for i := range leftKeys {
		if !leftKeys[i].Valid() || !rightKeys[i].Valid() {
			return LogicalPlan{}
		}
	}
	var predicate Expr
	if len(residual) == 1 {
		predicate = residual[0]
		if !predicate.Valid() {
			return LogicalPlan{}
		}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalJoin, input: p.root, right: right.root, joinKind: kind, leftKeys: append([]Expr(nil), leftKeys...), rightKeys: append([]Expr(nil), rightKeys...), residual: predicate}}
}
