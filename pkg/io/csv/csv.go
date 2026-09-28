package csv

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"runtime"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

const defaultCSVBatchSize = 4096

var errCSVRecordResource = errors.New("cannot allocate CSV record buffer")

// CSVOptions configures a streaming CSV reader. BatchSize defaults to 4096
// and the default NULL token is \\N; EmptyIsNull also treats empty fields as NULL.
type CSVOptions struct {
	Comma       rune
	Comment     rune
	BatchSize   int
	NullToken   string
	EmptyIsNull bool
	HasHeader   bool
}

// CSVErrorCode identifies a reader failure.
type CSVErrorCode uint8

const (
	CSVInvalidConfiguration CSVErrorCode = iota + 1
	CSVInvalidRecord
	CSVInvalidValue
	CSVNullViolation
	CSVResourceExhausted
	CSVReadFailure
	CSVCancelled
)

// CSVError is a typed CSV error. Record is one-based after any header;
// column is zero-based, or -1 for record-level failures.
type CSVError struct {
	code   CSVErrorCode
	record int
	column int
	cause  error
}

// Code returns the failure category.
func (e *CSVError) Code() CSVErrorCode {
	if e == nil {
		return 0
	}
	return e.code
}

// Record returns the one-based data record index.
func (e *CSVError) Record() int {
	if e == nil {
		return 0
	}
	return e.record
}

// Column returns the zero-based column index, or -1 for record failures.
func (e *CSVError) Column() int {
	if e == nil {
		return -1
	}
	return e.column
}

// Unwrap returns the underlying I/O, decoding, or cancellation failure.
func (e *CSVError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Error returns a description with the record and column location.
func (e *CSVError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("CSV record %d column %d: %v", e.record, e.column, e.cause)
}

// MakeCSVReader returns a reader over a borrowed input. It does not close the
// input. Its Next method produces independently retainable batches.
func MakeCSVReader(input io.Reader, a *mem.Allocator, types []dtype.Type, nullable []bool, options CSVOptions) (*CSVReader, *CSVError) {
	return MakeCSVReaderProjected(input, a, types, nullable, nil, options)
}

// MakeCSVReaderProjected converts selected columns while still checking the
// complete record width. A nil selection converts all columns.
func MakeCSVReaderProjected(input io.Reader, a *mem.Allocator, types []dtype.Type, nullable []bool, selected []int, options CSVOptions) (*CSVReader, *CSVError) {
	if input == nil || a == nil || len(types) == 0 || len(types) != len(nullable) {
		return nil, &CSVError{code: CSVInvalidConfiguration, column: -1, cause: errors.New("invalid source or schema")}
	}
	for col, t := range types {
		if !t.Valid() || t.ID() == dtype.NULLT {
			return nil, &CSVError{code: CSVInvalidConfiguration, column: col, cause: errors.New("invalid physical type")}
		}
	}
	if selected == nil {
		selected = make([]int, len(types))
		for i := range selected {
			selected[i] = i
		}
	}
	if len(selected) == 0 {
		return nil, &CSVError{code: CSVInvalidConfiguration, column: -1, cause: errors.New("no columns selected")}
	}
	seen := make([]bool, len(types))
	for _, column := range selected {
		if column < 0 || column >= len(types) || seen[column] {
			return nil, &CSVError{code: CSVInvalidConfiguration, column: column, cause: errors.New("invalid selected column")}
		}
		seen[column] = true
	}
	if options.Comma == 0 {
		options.Comma = ','
	}
	if options.Comma == '\n' || options.Comma == '\r' || options.Comma == '"' || options.Comment != 0 && (options.Comment == options.Comma || options.Comment == '\n' || options.Comment == '\r' || options.Comment == '"') {
		return nil, &CSVError{code: CSVInvalidConfiguration, column: -1, cause: errors.New("invalid delimiter or comment rune")}
	}
	if options.BatchSize <= 0 {
		options.BatchSize = defaultCSVBatchSize
	}
	if options.NullToken == "" {
		options.NullToken = "\\N"
	}
	r := &CSVReader{source: bufio.NewReaderSize(input, 64<<10), allocator: a, types: append([]dtype.Type(nil), types...), nullable: append([]bool(nil), nullable...), selected: append([]int(nil), selected...), options: options}
	if options.Comma > 127 || options.Comment > 127 {
		r.fallback = r.makeFallback(r.source)
	}
	return r, nil
}

// CSVReader incrementally converts CSV records into typed batches. It is
// owner-confined; concurrent Next calls require external synchronization.
type CSVReader struct {
	source     *bufio.Reader
	fallback   *csv.Reader
	allocator  *mem.Allocator
	types      []dtype.Type
	nullable   []bool
	selected   []int
	options    CSVOptions
	record     int
	headerRead bool
}

func (r *CSVReader) makeFallback(input io.Reader) *csv.Reader {
	c := csv.NewReader(input)
	c.Comma, c.Comment = r.options.Comma, r.options.Comment
	c.FieldsPerRecord = len(r.types)
	c.ReuseRecord = true
	return c
}

func (r *CSVReader) readRecord() ([]string, error) {
	if r.fallback != nil {
		return r.fallback.Read()
	}
	line, segment, err := r.readRawRecord()
	if segment != nil {
		defer segment.Dec()
	}
	if err != nil {
		return nil, err
	}
	return r.decodeCSVRecord(line)
}

func (r *CSVReader) readRawRecord() (raw []byte, segment *mem.Segment, err error) {
	var buffer []byte
	defer func() {
		if err != nil && segment != nil {
			segment.Dec()
			segment = nil
		}
	}()
	var inQuote, comment bool
	for {
		line, readErr := r.source.ReadSlice('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, bufio.ErrBufferFull) {
			return nil, segment, readErr
		}
		if len(line) == 0 && errors.Is(readErr, io.EOF) {
			if len(buffer) == 0 {
				return nil, segment, io.EOF
			}
			return buffer, segment, nil
		}
		if comment || len(buffer) == 0 && r.options.Comment != 0 && len(line) > 0 && line[0] == byte(r.options.Comment) {
			comment = line[len(line)-1] != '\n'
			continue
		}
		if len(buffer) == 0 && (len(line) == 1 && (line[0] == '\n' || line[0] == '\r' && errors.Is(readErr, io.EOF)) || len(line) == 2 && line[0] == '\r' && line[1] == '\n') {
			continue
		}
		var ended bool
		_, _, inQuote, ended = scanCSVChunk(line, byte(r.options.Comma), nil, len(buffer), inQuote)
		if len(buffer) != 0 || !ended && !errors.Is(readErr, io.EOF) {
			if len(line) > cap(buffer)-len(buffer) {
				size := max(4096, cap(buffer)*2, len(buffer)+len(line))
				next := r.allocator.AllocSegTemp(size)
				if next == nil {
					return nil, segment, errCSVRecordResource
				}
				grown := next.AsBytes()[:len(buffer)]
				copy(grown, buffer)
				if segment != nil {
					segment.Dec()
				}
				buffer, segment = grown, next
			}
			buffer = append(buffer, line...)
			line = buffer
		}
		if !ended && !errors.Is(readErr, io.EOF) {
			continue
		}
		return line, segment, nil
	}
}

func (r *CSVReader) decodeCSVRecord(line []byte) ([]string, error) {
	fields, err := decodeCSVFields(line, byte(r.options.Comma), len(r.types), make([][]byte, 0, len(r.types)))
	if err != nil {
		return nil, err
	}
	record := make([]string, len(fields))
	for i, field := range fields {
		record[i] = string(field)
	}
	return record, nil
}

// Next returns the next owned batch, or (nil, nil) after end of input.
func (r *CSVReader) Next(ctx context.Context) (*store.Batch, *CSVError) {
	if r == nil || ctx == nil {
		return nil, &CSVError{code: CSVInvalidConfiguration, column: -1, cause: errors.New("nil reader or context")}
	}
	if err := ctx.Err(); err != nil {
		return nil, &CSVError{code: CSVCancelled, record: r.record + 1, column: -1, cause: err}
	}
	if !r.headerRead {
		r.headerRead = true
		if r.options.HasHeader {
			header, err := r.readRecord()
			if err != nil && !errors.Is(err, io.EOF) {
				code := CSVInvalidRecord
				if errors.Is(err, errCSVRecordResource) {
					code = CSVResourceExhausted
				}
				return nil, &CSVError{code: code, column: -1, cause: err}
			}
			if err == nil && len(header) != len(r.types) {
				return nil, &CSVError{code: CSVInvalidRecord, column: -1, cause: csv.ErrFieldCount}
			}
		}
	}
	if r.fallback == nil {
		return r.nextASCII(ctx)
	}
	rows := make([][]string, 0, r.options.BatchSize)
	for len(rows) < r.options.BatchSize {
		if err := ctx.Err(); err != nil {
			return nil, &CSVError{code: CSVCancelled, record: r.record + 1, column: -1, cause: err}
		}
		record, err := r.readRecord()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			code := CSVReadFailure
			var parseError *csv.ParseError
			if errors.Is(err, errCSVRecordResource) {
				code = CSVResourceExhausted
			} else if errors.As(err, &parseError) || errors.Is(err, csv.ErrFieldCount) {
				code = CSVInvalidRecord
			}
			return nil, &CSVError{code: code, record: r.record + len(rows) + 1, column: -1, cause: err}
		}
		if r.fallback != nil {
			record = append([]string(nil), record...)
		}
		rows = append(rows, record)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	vectors := make([]store.Vector, len(r.selected))
	release := func() {
		for i := range vectors {
			vectors[i].Release()
		}
	}
	for outputColumn, column := range r.selected {
		t := r.types[column]
		if t.ID() == dtype.STRT {
			stringsIn := make([][]byte, len(rows))
			var valid []bool
			if r.nullable[column] {
				valid = make([]bool, len(rows))
			}
			for row, record := range rows {
				raw := record[column]
				isNull := raw == r.options.NullToken || r.options.EmptyIsNull && raw == ""
				if isNull && !r.nullable[column] {
					release()
					return nil, &CSVError{code: CSVNullViolation, record: r.record + row + 1, column: column, cause: errors.New("NULL in non-nullable column")}
				}
				if isNull {
					continue
				}
				stringsIn[row] = unsafe.Slice(unsafe.StringData(raw), len(raw))
				if valid != nil {
					valid[row] = true
				}
			}
			vectors[outputColumn] = store.MakeStringVector(r.allocator, stringsIn, valid)
		} else {
			vectors[outputColumn] = store.MakeVector(r.allocator, len(rows), t, r.nullable[column])
			if vectors[outputColumn].Kind() == store.VectorInvalid {
				release()
				return nil, &CSVError{code: CSVResourceExhausted, record: r.record + 1, column: column, cause: errors.New("cannot allocate column")}
			}
			for row, record := range rows {
				raw := record[column]
				isNull := raw == r.options.NullToken || r.options.EmptyIsNull && raw == ""
				if isNull {
					if !r.nullable[column] {
						release()
						return nil, &CSVError{code: CSVNullViolation, record: r.record + row + 1, column: column, cause: errors.New("NULL in non-nullable column")}
					}
					vectors[outputColumn].Validity().Clear(row)
					continue
				}
				if err := setCSVValue(&vectors[outputColumn], row, raw, t); err != nil {
					release()
					return nil, &CSVError{code: CSVInvalidValue, record: r.record + row + 1, column: column, cause: err}
				}
			}
		}
		if vectors[outputColumn].Kind() == store.VectorInvalid {
			release()
			return nil, &CSVError{code: CSVResourceExhausted, record: r.record + 1, column: column, cause: errors.New("cannot allocate column")}
		}
	}
	runtime.KeepAlive(rows)
	batch := store.MakeBatch(vectors)
	if batch == nil {
		release()
		return nil, &CSVError{code: CSVResourceExhausted, record: r.record + 1, column: -1, cause: errors.New("cannot create batch")}
	}
	r.record += len(rows)
	return batch, nil
}
