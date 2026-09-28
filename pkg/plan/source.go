package plan

import (
	"context"
	"errors"
	"os"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/parse"
	"github.com/rhawrami/peGosus/pkg/store"
)

// MakeCSVScan returns a reusable logical scan over a CSV file. Every execution
// opens the file independently; the schema supplies names, types and nullability.
func MakeCSVScan(path string, schema Schema, options parse.CSVOptions) LogicalPlan {
	if path == "" || !schema.Valid() || schema.Len() == 0 {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalScan, csvPath: path, csvOptions: options, schema: schema}}
}

type scanSource struct {
	table      *store.Table
	csvPath    string
	csvOptions parse.CSVOptions
	schema     Schema
	projection []int
	filters    []physicalExprProgram
}

var errScanFilter = errors.New("scan predicate evaluation failed")

func (s *scanSource) Valid() bool {
	return s != nil && (s.table.Valid() || s.csvPath != "" && s.schema.Valid())
}

func (s *scanSource) Retain() *scanSource {
	if !s.Valid() {
		return nil
	}
	retained := *s
	if s.table != nil {
		retained.table = s.table.Retain()
	}
	return &retained
}

func (s *scanSource) Release() {
	if s == nil {
		return
	}
	if s.table != nil {
		s.table.Release()
	}
	*s = scanSource{}
}

func (s *scanSource) NBatches() int {
	if s == nil || s.table == nil {
		return 0
	}
	return s.table.NBatches()
}

func (s *scanSource) BatchAt(i int) *store.Batch { return s.table.BatchAt(i) }

type scanCursor struct {
	source *scanSource
	file   *os.File
	csv    *parse.CSVReader
	index  int
}

func (c *scanCursor) close() {
	if c.file != nil {
		c.file.Close()
		c.file = nil
	}
}

func (c *scanCursor) next(ctx context.Context, a *mem.Allocator) (*store.Batch, bool, error) {
	if c.source.table != nil {
		if c.index == c.source.table.NBatches() {
			return nil, true, nil
		}
		var batch *store.Batch
		if len(c.source.projection) != 0 {
			batch = c.source.table.BatchAt(c.index).Project(c.source.projection)
		} else {
			batch = c.source.table.BatchAt(c.index).Retain()
		}
		c.index++
		if !c.applyFilters(a, batch) {
			batch.Release()
			return nil, false, errScanFilter
		}
		return batch, false, nil
	}
	if c.csv == nil {
		file, err := os.Open(c.source.csvPath)
		if err != nil {
			return nil, false, err
		}
		c.file = file
		types := make([]dtype.Type, c.source.schema.Len())
		nullable := make([]bool, len(types))
		for i := range types {
			field := c.source.schema.FieldAt(i)
			types[i], nullable[i] = field.Type(), field.Nullable()
		}
		reader, parseError := parse.MakeCSVReaderProjected(file, a, types, nullable, c.source.projection, c.source.csvOptions)
		if parseError != nil {
			return nil, false, parseError
		}
		c.csv = reader
	}
	batch, parseError := c.csv.Next(ctx)
	if parseError != nil {
		return nil, false, parseError
	}
	if batch == nil {
		return nil, true, nil
	}
	if !c.applyFilters(a, batch) {
		batch.Release()
		return nil, false, errScanFilter
	}
	return batch, false, nil
}

func (c *scanCursor) applyFilters(a *mem.Allocator, batch *store.Batch) bool {
	for _, program := range c.source.filters {
		if !executePhysicalFilter(a, batch, program) {
			return false
		}
	}
	return true
}
