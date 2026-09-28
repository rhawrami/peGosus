package plan

import (
	"context"
	"errors"
	"os"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/io/csv"
	"github.com/rhawrami/peGosus/pkg/io/parquet"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

// MakeCSVScan returns a reusable logical scan over a CSV file. Every execution
// opens the file independently; the schema supplies names, types and nullability.
func MakeCSVScan(path string, schema Schema, options csv.CSVOptions) LogicalPlan {
	if path == "" || !schema.Valid() || schema.Len() == 0 {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalScan, csvPath: path, csvOptions: options, schema: schema}}
}

// MakeParquetScan returns a reusable logical scan over a Parquet file.
// The declared schema is checked against the file on every execution.
func MakeParquetScan(path string, schema Schema, options parquet.ParquetOptions) LogicalPlan {
	if path == "" || !schema.Valid() || schema.Len() == 0 {
		return LogicalPlan{}
	}
	return LogicalPlan{root: &logicalNode{operation: logicalScan, parquetPath: path, parquetOptions: options, schema: schema}}
}

type scanSource struct {
	table          *store.Table
	csvPath        string
	csvOptions     csv.CSVOptions
	parquetPath    string
	parquetOptions parquet.ParquetOptions
	schema         Schema
	projection     []int
	filters        []physicalExprProgram
}

var errScanFilter = errors.New("scan predicate evaluation failed")

func (s *scanSource) Valid() bool {
	return s != nil && (s.table.Valid() || (s.csvPath != "" || s.parquetPath != "") && s.schema.Valid())
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
	source  *scanSource
	file    *os.File
	csv     *csv.CSVReader
	parquet *parquet.ParquetReader
	index   int
}

func (c *scanCursor) close() {
	if c.parquet != nil {
		c.parquet.Close()
		c.parquet = nil
	}
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
	if c.source.parquetPath != "" {
		if c.parquet == nil {
			file, err := os.Open(c.source.parquetPath)
			if err != nil {
				return nil, false, err
			}
			c.file = file
			info, err := file.Stat()
			if err != nil {
				return nil, false, err
			}
			reader, parseErr := parquet.MakeParquetReaderProjected(file, info.Size(), a, c.source.projection, c.source.parquetOptions)
			if parseErr != nil {
				return nil, false, parseErr
			}
			if reader.ColumnCount() != c.source.schema.Len() {
				reader.Close()
				return nil, false, parquet.MakeParquetError(parquet.ParquetInvalid, errors.New("Parquet schema width differs from bound scan schema"))
			}
			for i := range c.source.schema.Len() {
				name, typ, nullable := reader.Column(i)
				field := c.source.schema.FieldAt(i)
				if name != field.Name() || !typ.Equal(field.Type()) || nullable && !field.Nullable() {
					reader.Close()
					return nil, false, parquet.MakeParquetError(parquet.ParquetInvalid, errors.New("Parquet schema differs from bound scan schema"))
				}
			}
			c.parquet = reader
		}
		batch, parseErr := c.parquet.Next(ctx)
		if parseErr != nil {
			return nil, false, parseErr
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
		reader, parseError := csv.MakeCSVReaderProjected(file, a, types, nullable, c.source.projection, c.source.csvOptions)
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
