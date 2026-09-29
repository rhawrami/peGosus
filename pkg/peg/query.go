package peg

import (
	"context"
	"errors"
	"strconv"

	"github.com/rhawrami/peGosus/pkg/plan"
)

// JoinKind selects an equality join's output rows.
type JoinKind = plan.JoinKind

const (
	JoinInner JoinKind = plan.JoinInner
	JoinLeft  JoinKind = plan.JoinLeft
	JoinFull  JoinKind = plan.JoinFull
	JoinSemi  JoinKind = plan.JoinSemi
	JoinAnti  JoinKind = plan.JoinAnti
)

// Query is an immutable lazy relation. Construction failures are carried
// through the chain and returned by Prepare or Exec.
type Query struct {
	engine  *Engine
	logical plan.LogicalPlan
	columns []string
	alias   string
	err     error
}

// Filter retains only rows whose nullable predicate is TRUE. Multiple
// predicates compose in order.
func (q Query) Filter(predicates ...Expr) Query {
	if q.err != nil {
		return q
	}
	if len(predicates) == 0 {
		q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("Filter requires a predicate")}
		return q
	}
	for _, predicate := range predicates {
		if predicate.aggregate != aggregateNone || !predicate.Valid() {
			q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("invalid filter expression")}
			return q
		}
		q.logical = q.logical.Filter(predicate.inner)
	}
	return q
}

// WithCols keeps existing columns while adding or replacing named expressions.
// An unaliased expression derived from C("name") replaces that column.
func (q Query) WithCols(expressions ...Expr) Query {
	if q.err != nil {
		return q
	}
	if len(expressions) == 0 {
		q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("WithCols requires an expression")}
		return q
	}
	names := append([]string(nil), q.columns...)
	projected := make([]plan.Expr, len(names))
	for i, name := range names {
		projected[i] = plan.MakeColumn(name)
	}
	updated := make(map[string]bool, len(expressions))
	for _, expression := range expressions {
		name := expression.name()
		if !expression.Valid() || expression.aggregate != aggregateNone || name == "" || updated[name] {
			q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("WithCols requires distinct named column expressions")}
			return q
		}
		updated[name] = true
		at := -1
		for i, existing := range names {
			if existing == name {
				at = i
				break
			}
		}
		if at < 0 {
			at = len(names)
			names = append(names, name)
			projected = append(projected, plan.Expr{})
		}
		projected[at] = expression.inner.Alias(name)
	}
	q.logical = q.logical.Project(projected...)
	q.columns = names
	q.alias = ""
	return q
}

// Select keeps only the named expressions, in argument order.
func (q Query) Select(expressions ...Expr) Query {
	if q.err != nil {
		return q
	}
	if len(expressions) == 0 {
		q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("Select requires expressions")}
		return q
	}
	names := make([]string, len(expressions))
	projected := make([]plan.Expr, len(expressions))
	seen := make(map[string]bool, len(expressions))
	for i, expression := range expressions {
		name := expression.name()
		if !expression.Valid() || expression.aggregate != aggregateNone || name == "" || seen[name] {
			q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("Select requires distinct named column expressions")}
			return q
		}
		seen[name] = true
		names[i], projected[i] = name, expression.inner.Alias(name)
	}
	q.logical = q.logical.Project(projected...)
	q.columns = names
	q.alias = ""
	return q
}

// GroupBy begins a grouped aggregate over one or more key expressions.
func (q Query) GroupBy(keys ...Expr) GroupedQuery {
	return GroupedQuery{query: q, keys: append([]Expr(nil), keys...)}
}

// Agg computes one global row of aggregate expressions.
func (q Query) Agg(expressions ...Expr) Query {
	return GroupedQuery{query: q}.aggregate(expressions, true)
}

// GroupedQuery is a pending GROUP BY, completed by Agg.
type GroupedQuery struct {
	query Query
	keys  []Expr
}

// Agg completes a grouped aggregation with aggregate expressions.
func (g GroupedQuery) Agg(expressions ...Expr) Query {
	return g.aggregate(expressions, false)
}

func (g GroupedQuery) aggregate(expressions []Expr, global bool) Query {
	q := g.query
	if q.err != nil {
		return q
	}
	if !global && len(g.keys) == 0 || len(expressions) == 0 {
		q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("GroupBy and Agg each require expressions")}
		return q
	}
	keys := make([]plan.Expr, len(g.keys))
	names := make([]string, 0, len(g.keys)+len(expressions))
	seen := make(map[string]bool, len(g.keys)+len(expressions))
	for i, key := range g.keys {
		name := key.name()
		if !key.Valid() || key.aggregate != aggregateNone || name == "" || seen[name] {
			q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("GroupBy requires distinct named column expressions")}
			return q
		}
		seen[name] = true
		names = append(names, name)
		keys[i] = key.inner.Alias(name)
	}
	aggregates := make([]plan.Aggregate, len(expressions))
	for i, expression := range expressions {
		if !expression.Valid() || expression.aggregate == aggregateNone {
			q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("Agg requires aggregate expressions")}
			return q
		}
		name := expression.alias
		if name == "" {
			name = "aggregate_" + strconv.Itoa(i+1)
		}
		if seen[name] {
			q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("aggregate output names must be distinct")}
			return q
		}
		seen[name] = true
		names = append(names, name)
		aggregates[i] = expression.asPlanAggregate().Alias(name)
	}
	if global {
		q.logical = q.logical.Aggregate(aggregates...)
	} else {
		q.logical = q.logical.GroupBy(keys, aggregates...)
	}
	q.columns = names
	q.alias = ""
	return q
}

// Distinct retains one row per distinct tuple of the current columns.
func (q Query) Distinct() Query {
	if q.err == nil {
		q.logical = q.logical.Distinct()
	}
	return q
}

// As gives this relation's columns a source alias for joins and qualified names.
func (q Query) As(alias string) Query {
	if q.err != nil {
		return q
	}
	if alias == "" {
		q.err = &Error{code: ErrorInvalidQuery, cause: errors.New("source alias must not be empty")}
		return q
	}
	q.logical = q.logical.As(alias)
	q.alias = alias
	return q
}

// OrderKey identifies a sort expression and its null placement.
type OrderKey struct {
	inner plan.OrderKey
	valid bool
}

// Asc returns an ascending ordering key with nulls last.
func (e Expr) Asc() OrderKey {
	return OrderKey{inner: plan.MakeOrderKey(e.inner), valid: e.Valid() && e.aggregate == aggregateNone}
}

// Desc returns a descending ordering key with nulls last.
func (e Expr) Desc() OrderKey { key := e.Asc(); key.inner = key.inner.Desc(); return key }

// NullsFirst places null keys before all non-null values.
func (k OrderKey) NullsFirst() OrderKey { k.inner = k.inner.NullsFirst(); return k }

// NullsLast places null keys after all non-null values.
func (k OrderKey) NullsLast() OrderKey { k.inner = k.inner.NullsLast(); return k }

// OrderBy sorts by one or more expressions in priority order.
func (q Query) OrderBy(keys ...OrderKey) Query {
	if q.err != nil {
		return q
	}
	if len(keys) == 0 {
		q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("OrderBy requires keys")}
		return q
	}
	order := make([]plan.OrderKey, len(keys))
	for i, key := range keys {
		if !key.valid {
			q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("invalid order key")}
			return q
		}
		order[i] = key.inner
	}
	q.logical = q.logical.OrderBy(order...)
	return q
}

// Join joins two aliased relations on corresponding equality keys, optionally
// applying a residual predicate after key matching.
func (q Query) Join(right Query, kind JoinKind, leftKeys, rightKeys []Expr, residual ...Expr) Query {
	if q.err != nil {
		return q
	}
	if right.err != nil {
		q.err = right.err
		return q
	}
	if q.engine == nil || q.engine != right.engine || q.alias == "" || right.alias == "" || len(leftKeys) == 0 || len(leftKeys) != len(rightKeys) || len(residual) > 1 || kind < JoinInner || kind > JoinAnti {
		q.err = &Error{code: ErrorInvalidQuery, cause: errors.New("Join requires one engine, distinct source aliases, and corresponding equality keys")}
		return q
	}
	left, other := make([]plan.Expr, len(leftKeys)), make([]plan.Expr, len(rightKeys))
	for i := range left {
		if !leftKeys[i].Valid() || leftKeys[i].aggregate != aggregateNone || !rightKeys[i].Valid() || rightKeys[i].aggregate != aggregateNone {
			q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("invalid join key")}
			return q
		}
		left[i], other[i] = leftKeys[i].inner, rightKeys[i].inner
	}
	var predicate []plan.Expr
	if len(residual) == 1 {
		if !residual[0].Valid() || residual[0].aggregate != aggregateNone {
			q.err = &Error{code: ErrorInvalidExpression, cause: errors.New("invalid join residual")}
			return q
		}
		predicate = []plan.Expr{residual[0].inner}
	}
	q.logical = q.logical.Join(right.logical, kind, left, other, predicate...)
	q.columns = append([]string(nil), q.columns...)
	for i, name := range q.columns {
		q.columns[i] = q.alias + "." + name
	}
	if kind != JoinSemi && kind != JoinAnti {
		for _, name := range right.columns {
			q.columns = append(q.columns, right.alias+"."+name)
		}
	}
	q.alias = ""
	return q
}

// Limit restricts the active result rows, with an optional offset.
func (q Query) Limit(count int64, offset ...int64) Query {
	if q.err == nil {
		q.logical = q.logical.Limit(count, offset...)
	}
	return q
}

// Exec prepares and executes the query with a background context.
func (q Query) Exec() (*Result, error) { return q.ExecContext(context.Background()) }

// ExecContext prepares and executes the query with cancellation.
func (q Query) ExecContext(ctx context.Context) (*Result, error) {
	if q.engine == nil {
		return nil, &Error{code: ErrorInvalidQuery, cause: errors.New("query has no engine")}
	}
	prepared, err := q.engine.Prepare(q)
	if err != nil {
		return nil, err
	}
	defer prepared.Release()
	return prepared.ExecContext(ctx)
}

// RunContext executes into a borrowed-batch sink without materializing a result.
func (q Query) RunContext(ctx context.Context, sink func(Batch) bool) (RunStatus, error) {
	if q.engine == nil {
		return RunCompleted, &Error{code: ErrorInvalidQuery, cause: errors.New("query has no engine")}
	}
	prepared, err := q.engine.Prepare(q)
	if err != nil {
		return RunCompleted, err
	}
	defer prepared.Release()
	return prepared.RunContext(ctx, sink)
}
