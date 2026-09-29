package peg

import (
	"context"
	"errors"
	"os"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/csv"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/plan"
	"github.com/rhawrami/peGosus/pkg/store"
)

const defaultMemoryBudget int64 = 256 << 20
const defaultSlabBytes = 1 << 20

// EngineOptions sets defaults for every query executed by an engine. A zero
// budget selects 256 MiB; zero workers selects the executor's automatic policy.
type EngineOptions struct {
	MemoryBudget int64
	Workers      int
}

// ParquetOptions configures Parquet decoding and resource limits.
type ParquetOptions struct {
	BatchSize           int
	MaxFooterBytes      int
	MaxPageBytes        int
	MaxDecodedPageBytes int
}

// CSVOptions configures a CSV scan with a caller-supplied typed schema.
type CSVOptions = csv.CSVOptions

func (o ParquetOptions) parserOptions() parquet.ParquetOptions {
	return parquet.ParquetOptions{
		BatchSize: o.BatchSize, MaxFooterBytes: o.MaxFooterBytes,
		MaxPageBytes: o.MaxPageBytes, MaxDecodedPageBytes: o.MaxDecodedPageBytes,
	}
}

// MakeEngine returns an allocator-owning query engine. Invalid negative
// budgets or worker counts yield nil.
func MakeEngine(options ...EngineOptions) *Engine {
	if len(options) > 1 {
		return nil
	}
	var config EngineOptions
	if len(options) == 1 {
		config = options[0]
	}
	if config.MemoryBudget < 0 || config.Workers < 0 {
		return nil
	}
	if config.MemoryBudget == 0 {
		config.MemoryBudget = defaultMemoryBudget
	}
	return &Engine{
		allocator:    mem.MakeAllocatorWithProfiles([]int{defaultSlabBytes}, []int{defaultSlabBytes}),
		memoryBudget: config.MemoryBudget, workers: config.Workers,
	}
}

// Engine owns reusable allocation slabs; individual executions have separate
// query budgets. Concurrent prepared queries may share an Engine.
type Engine struct {
	allocator    *mem.Allocator
	memoryBudget int64
	workers      int
}

// ScanTable scans an in-memory table. The caller keeps table alive until the
// query is prepared; a prepared plan retains the table independently.
func (e *Engine) ScanTable(table *store.Table, names []string) Query {
	query := Query{engine: e}
	if e == nil || e.allocator == nil || table == nil || !table.Valid() || len(names) != table.NColumns() || len(names) == 0 {
		query.err = &Error{code: ErrorInvalidQuery, cause: errors.New("invalid table or column names")}
		return query
	}
	types := make([]dtype.Type, len(names))
	nullable := make([]bool, len(names))
	for i := range names {
		types[i], nullable[i] = table.TypeAt(i), table.NullableAt(i)
	}
	query.logical = plan.MakeScan(table, plan.MakeSchemaWithNullability(names, types, nullable))
	query.columns = append([]string(nil), names...)
	return query
}

// ScanCSV scans a CSV file using explicit names, types, and nullability.
// CSV schema inference is not performed.
func (e *Engine) ScanCSV(path string, fields []Field, options ...CSVOptions) Query {
	query := Query{engine: e}
	if e == nil || e.allocator == nil || path == "" || len(fields) == 0 || len(options) > 1 {
		query.err = &Error{code: ErrorInvalidQuery, cause: errors.New("invalid engine, CSV path, or schema")}
		return query
	}
	var config CSVOptions
	if len(options) == 1 {
		config = options[0]
	}
	names := make([]string, len(fields))
	types := make([]dtype.Type, len(fields))
	nullable := make([]bool, len(fields))
	for i, field := range fields {
		names[i], types[i], nullable[i] = field.Name, dtype.MakeType(field.Kind), field.Nullable
	}
	schema := plan.MakeSchemaWithNullability(names, types, nullable)
	if !schema.Valid() {
		query.err = &Error{code: ErrorInvalidQuery, cause: errors.New("invalid CSV field type")}
		return query
	}
	query.logical = plan.MakeCSVScan(path, schema, config)
	query.columns = names
	return query
}

// ScanParquet discovers a flat Parquet schema and returns an immutable lazy
// query. Source errors are returned by Exec or Prepare so scans can be chained.
func (e *Engine) ScanParquet(path string, options ...ParquetOptions) Query {
	query := Query{engine: e}
	if e == nil || e.allocator == nil || path == "" || len(options) > 1 {
		query.err = &Error{code: ErrorInvalidQuery, cause: errors.New("invalid engine, path, or Parquet options")}
		return query
	}
	var config ParquetOptions
	if len(options) == 1 {
		config = options[0]
	}
	file, err := os.Open(path)
	if err != nil {
		query.err = wrapError(err)
		return query
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		query.err = wrapError(err)
		return query
	}
	scope := mem.MakeAllocationScope(e.allocator, e.memoryBudget)
	view := mem.MakeAllocatorWithScope(e.allocator, scope)
	reader, failure := parquet.MakeParquetReader(file, info.Size(), view, config.parserOptions())
	if failure != nil {
		query.err = wrapError(failure)
		return query
	}
	defer reader.Close()
	names := make([]string, reader.ColumnCount())
	types := make([]dtype.Type, len(names))
	nullable := make([]bool, len(names))
	for i := range names {
		names[i], types[i], nullable[i] = reader.Column(i)
	}
	query.logical = plan.MakeParquetScan(path, plan.MakeSchemaWithNullability(names, types, nullable), config.parserOptions())
	query.columns = names
	return query
}

// Prepare binds a lazy query into a reusable execution plan.
func (e *Engine) Prepare(query Query) (*Prepared, error) {
	if e == nil || e.allocator == nil || query.engine != e {
		return nil, &Error{code: ErrorInvalidQuery, cause: errors.New("query belongs to another or invalid engine")}
	}
	if query.err != nil {
		return nil, query.err
	}
	physical, err := plan.MakePhysicalPlan(query.logical)
	if err != nil {
		return nil, wrapError(err)
	}
	return &Prepared{engine: e, physical: physical, fields: makeFields(physical.Schema())}, nil
}

// Prepared is a reusable bound plan. Release requires active executions to
// finish; sink-retained result batches are independent of plan lifetime.
type Prepared struct {
	engine   *Engine
	physical *plan.PhysicalPlan
	fields   []Field
}

// Release releases source ownership held by a prepared query.
func (p *Prepared) Release() {
	if p == nil || p.physical == nil {
		return
	}
	p.physical.Release()
	p.physical = nil
	p.fields = nil
}

// RunStatus distinguishes normal completion from a sink-requested stop.
type RunStatus uint8

const (
	RunCompleted RunStatus = iota
	RunStopped
)

// RunContext pushes borrowed batches to sink without materializing a result.
// Returning false from sink stops upstream workers cooperatively.
func (p *Prepared) RunContext(ctx context.Context, sink func(Batch) bool) (RunStatus, error) {
	if p == nil || p.physical == nil || p.engine == nil || ctx == nil || sink == nil {
		return RunCompleted, &Error{code: ErrorInvalidQuery, cause: errors.New("invalid prepared query, context, or sink")}
	}
	outcome := p.physical.ExecuteWithOptions(ctx, p.engine.allocator, plan.ExecutionOptions{MemoryBudget: p.engine.memoryBudget, Workers: p.engine.workers}, func(batch *store.Batch) bool {
		return sink(Batch{inner: batch, fields: p.fields})
	})
	switch outcome.Code() {
	case plan.ExecutionCompleted:
		return RunCompleted, nil
	case plan.ExecutionStopped:
		return RunStopped, nil
	default:
		return RunCompleted, executionError(outcome)
	}
}

// Exec executes with a background context and materializes owned batches.
func (p *Prepared) Exec() (*Result, error) { return p.ExecContext(context.Background()) }

// ExecContext executes with cancellation and materializes owned batches.
// Release the result when finished so its allocator storage can be reused.
func (p *Prepared) ExecContext(ctx context.Context) (*Result, error) {
	if p == nil || p.physical == nil || p.engine == nil || ctx == nil {
		return nil, &Error{code: ErrorInvalidQuery, cause: errors.New("invalid prepared query or context")}
	}
	result := &Result{fields: p.fields}
	outcome := p.physical.ExecuteWithOptions(ctx, p.engine.allocator, plan.ExecutionOptions{MemoryBudget: p.engine.memoryBudget, Workers: p.engine.workers}, func(batch *store.Batch) bool {
		result.batches = append(result.batches, batch.Retain())
		result.rows += batch.ActiveLen()
		return true
	})
	if outcome.Code() != plan.ExecutionCompleted {
		result.Release()
		return nil, executionError(outcome)
	}
	return result, nil
}

func executionError(outcome plan.ExecutionResult) error {
	code := ErrorExecution
	switch outcome.Code() {
	case plan.ExecutionInvalidInvocation:
		code = ErrorInvalidQuery
	case plan.ExecutionCancelled:
		code = ErrorCancelled
	case plan.ExecutionResourceExhausted:
		code = ErrorResourceExhausted
	case plan.ExecutionSourceFailure:
		return wrapError(outcome.Err())
	}
	cause := outcome.Err()
	if cause == nil {
		cause = errors.New("query execution failed")
	}
	return &Error{code: code, cause: cause}
}
