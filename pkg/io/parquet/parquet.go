package parquet

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

const (
	parquetFooterLimit  = 16 << 20
	parquetPageLimit    = 64 << 20
	parquetDecodedLimit = 256 << 20
	parquetHeaderLimit  = 64 << 10
	parquetColumnLimit  = 1024
	parquetBatchSize    = 4096
)

// ParquetErrorCode classifies failures when reading a Parquet source.
type ParquetErrorCode uint8

const (
	ParquetInvalid ParquetErrorCode = iota + 1
	ParquetUnsupported
	ParquetResourceExhausted
	ParquetReadFailure
	ParquetCancelled
)

// ParquetError describes a reader failure and its location, if known.
type ParquetError struct {
	code                ParquetErrorCode
	group, column, page int
	cause               error
}

// MakeParquetError returns a source-level Parquet error without a page location.
func MakeParquetError(code ParquetErrorCode, cause error) *ParquetError {
	return &ParquetError{code: code, group: -1, column: -1, page: -1, cause: cause}
}

// Code returns the failure category.
func (e *ParquetError) Code() ParquetErrorCode {
	if e == nil {
		return 0
	}
	return e.code
}

// Unwrap returns the underlying failure.
func (e *ParquetError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Error returns a description with the row-group, column, and page location.
func (e *ParquetError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("Parquet group %d column %d page %d: %v", e.group, e.column, e.page, e.cause)
}

// RowGroup returns the zero-based row-group location, or -1 if unknown.
func (e *ParquetError) RowGroup() int {
	if e == nil {
		return -1
	}
	return e.group
}

// Column returns the zero-based column location, or -1 if unknown.
func (e *ParquetError) Column() int {
	if e == nil {
		return -1
	}
	return e.column
}

// Page returns the zero-based page location, or -1 if unknown.
func (e *ParquetError) Page() int {
	if e == nil {
		return -1
	}
	return e.page
}

// ParquetOptions configures the maximum input sizes and output batch length.
// Zero values select bounded defaults.
type ParquetOptions struct {
	BatchSize           int
	MaxFooterBytes      int
	MaxPageBytes        int
	MaxDecodedPageBytes int
}

type parquetColumn struct {
	name     string
	typ      dtype.Type
	optional bool
}
type parquetChunk struct {
	start, end int64
	codec      int64
	values     int64
	min, max   int64
	hasMinMax  bool
	allNull    bool
}
type parquetGroup struct {
	rows   int64
	chunks []parquetChunk
}

// MakeParquetReader opens all flat columns from a borrowed random-access input.
func MakeParquetReader(input io.ReaderAt, size int64, a *mem.Allocator, options ParquetOptions) (*ParquetReader, *ParquetError) {
	return MakeParquetReaderProjected(input, size, a, nil, options)
}

// MakeParquetReaderProjected opens a borrowed random-access input. The caller
// owns the input; each Next result owns its materialized vectors independently.
// A nil selected slice reads every column.
func MakeParquetReaderProjected(input io.ReaderAt, size int64, a *mem.Allocator, selected []int, options ParquetOptions) (*ParquetReader, *ParquetError) {
	invalid := func(code ParquetErrorCode, err error) (*ParquetReader, *ParquetError) {
		return nil, &ParquetError{code: code, group: -1, column: -1, page: -1, cause: err}
	}
	if input == nil || a == nil || size < 12 {
		return invalid(ParquetInvalid, errors.New("invalid input or file size"))
	}
	if options.BatchSize <= 0 {
		options.BatchSize = parquetBatchSize
	}
	if options.MaxFooterBytes <= 0 {
		options.MaxFooterBytes = parquetFooterLimit
	}
	if options.MaxPageBytes <= 0 {
		options.MaxPageBytes = parquetPageLimit
	}
	if options.MaxDecodedPageBytes <= 0 {
		options.MaxDecodedPageBytes = parquetDecodedLimit
	}
	var trailer [8]byte
	var prefix [4]byte
	if n, err := input.ReadAt(prefix[:], 0); err != nil || n != len(prefix) {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return invalid(ParquetReadFailure, err)
	}
	if n, err := input.ReadAt(trailer[:], size-8); err != nil || n != len(trailer) {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return invalid(ParquetReadFailure, err)
	}
	if string(prefix[:]) != "PAR1" || string(trailer[4:]) != "PAR1" {
		return invalid(ParquetInvalid, errors.New("missing Parquet magic"))
	}
	footerSize := int64(binary.LittleEndian.Uint32(trailer[:4]))
	if footerSize <= 0 || footerSize > size-12 {
		return invalid(ParquetInvalid, errors.New("invalid footer length"))
	}
	if footerSize > int64(options.MaxFooterBytes) || footerSize > math.MaxInt {
		return invalid(ParquetResourceExhausted, errors.New("footer exceeds configured size limit"))
	}
	segment := a.AllocSegTemp(int(footerSize))
	if segment == nil {
		return invalid(ParquetResourceExhausted, errors.New("footer allocation failed"))
	}
	defer segment.Dec()
	buf := segment.AsBytes()[:footerSize]
	if n, err := input.ReadAt(buf, size-8-footerSize); err != nil || n != len(buf) {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return invalid(ParquetReadFailure, err)
	}
	d := compactDecoder{buf: buf}
	meta, err := d.decode(12)
	if err != nil || d.pos != len(buf) {
		return invalid(ParquetInvalid, errCompact)
	}
	if meta.field(1).kind == 0 || meta.field(3).kind == 0 {
		return invalid(ParquetInvalid, errors.New("missing Parquet metadata fields"))
	}
	if meta.number(1) != 1 && meta.number(1) != 2 {
		return invalid(ParquetUnsupported, errors.New("unsupported file version"))
	}
	if meta.field(8).kind != 0 || meta.field(9).kind != 0 {
		return invalid(ParquetUnsupported, errors.New("encrypted footer"))
	}
	schema, groups := meta.field(2).items, meta.field(4).items
	if len(schema) < 2 || len(schema) > parquetColumnLimit+1 || schema[0].field(1).kind != 0 || meta.number(3) < 0 || meta.field(2).kind != 9 || meta.field(4).kind != 9 {
		return invalid(ParquetInvalid, errors.New("invalid flat schema or row groups"))
	}
	if schema[0].number(5) != int64(len(schema)-1) {
		return invalid(ParquetUnsupported, errors.New("nested schema"))
	}
	columns := make([]parquetColumn, len(schema)-1)
	seen := make(map[string]bool, len(columns))
	for i, node := range schema[1:] {
		name := node.stringField(4)
		if name == "" || !utf8.ValidString(name) || seen[name] || node.field(3).kind == 0 {
			return invalid(ParquetInvalid, errors.New("invalid schema field"))
		}
		seen[name] = true
		if node.field(1).kind == 0 || node.field(5).kind != 0 || node.number(3) > 1 || node.number(3) < 0 {
			return invalid(ParquetUnsupported, errors.New("nested or repeated field"))
		}
		typ, ok := parquetType(node)
		if !ok {
			return invalid(ParquetUnsupported, fmt.Errorf("unsupported type or annotation on %s", name))
		}
		columns[i] = parquetColumn{name: name, typ: typ, optional: node.number(3) == 1}
	}
	if selected == nil {
		selected = make([]int, len(columns))
		for i := range selected {
			selected[i] = i
		}
	}
	used := make([]bool, len(columns))
	for _, col := range selected {
		if col < 0 || col >= len(columns) || used[col] {
			return invalid(ParquetInvalid, errors.New("invalid projection"))
		}
		used[col] = true
	}
	if len(selected) == 0 {
		return invalid(ParquetInvalid, errors.New("empty projection"))
	}
	rowGroups := make([]parquetGroup, len(groups))
	var total int64
	footerStart := size - 8 - footerSize
	for i, group := range groups {
		rows, chunks := group.number(3), group.field(1).items
		if group.field(3).kind == 0 || rows < 0 || rows > math.MaxInt || len(chunks) != len(columns) || group.field(1).kind != 9 || total > math.MaxInt64-rows {
			return invalid(ParquetInvalid, errors.New("invalid row group"))
		}
		total += rows
		rowGroups[i] = parquetGroup{rows: rows, chunks: make([]parquetChunk, len(columns))}
		for j, chunk := range chunks {
			if chunk.field(1).kind != 0 || chunk.field(8).kind != 0 || chunk.field(9).kind != 0 {
				return invalid(ParquetUnsupported, errors.New("external or encrypted column"))
			}
			info := chunk.field(3)
			if info.kind != 12 || info.field(1).kind == 0 || info.field(2).kind != 9 || info.field(3).kind != 9 || info.field(4).kind == 0 || info.field(5).kind == 0 || info.field(7).kind == 0 || info.field(9).kind == 0 || info.number(1) != int64(schema[j+1].number(1)) || info.number(5) != rows || len(info.field(3).items) != 1 || info.field(3).items[0].text != columns[j].name {
				return invalid(ParquetInvalid, errors.New("column metadata or row count mismatch"))
			}
			start := info.number(9)
			if info.field(11).kind != 0 && info.number(11) < start {
				start = info.number(11)
			}
			length := info.number(7)
			if start < 4 || length < 0 || start > footerStart || length > footerStart-start {
				return invalid(ParquetInvalid, errors.New("column chunk outside file"))
			}
			column := parquetChunk{start: start, end: start + length, codec: info.number(4), values: rows}
			stats := info.field(12)
			if stats.kind == 12 {
				column.allNull = rows != 0 && stats.field(3).kind != 0 && stats.number(3) == rows
				width := 0
				switch columns[j].typ.ID() {
				case dtype.INT32T, dtype.DATET:
					width = 4
				case dtype.INT64T, dtype.TIMESTAMPTZT:
					width = 8
				}
				if width == 4 || width == 8 {
					minValue, maxValue := stats.field(6), stats.field(5)
					if minValue.kind == 0 && maxValue.kind == 0 {
						minValue, maxValue = stats.field(2), stats.field(1)
					}
					if minValue.kind == 8 && maxValue.kind == 8 && len(minValue.text) == width && len(maxValue.text) == width {
						if width == 4 {
							column.min = int64(int32(binary.LittleEndian.Uint32([]byte(minValue.text))))
							column.max = int64(int32(binary.LittleEndian.Uint32([]byte(maxValue.text))))
						} else {
							column.min = int64(binary.LittleEndian.Uint64([]byte(minValue.text)))
							column.max = int64(binary.LittleEndian.Uint64([]byte(maxValue.text)))
						}
						column.hasMinMax = column.min <= column.max
					}
				}
			}
			rowGroups[i].chunks[j] = column
		}
	}
	if total != meta.number(3) {
		return invalid(ParquetInvalid, errors.New("file row count mismatch"))
	}
	return &ParquetReader{input: input, allocator: a, options: options, columns: columns, groups: rowGroups, selected: append([]int(nil), selected...)}, nil
}

func parquetType(node compactValue) (dtype.Type, bool) {
	physical := node.number(1)
	legacy := int64(-1)
	if node.field(6).kind != 0 {
		legacy = node.number(6)
	}
	modern := node.field(10)
	annotation := int64(-1)
	if modern.kind != 0 {
		if modern.kind != 12 || len(modern.fields) != 1 {
			return dtype.Type{}, false
		}
		for id, value := range modern.fields {
			annotation = int64(id)
			if value.kind != 12 {
				return dtype.Type{}, false
			}
			switch id {
			case 8:
				unit := value.field(2)
				if value.field(1).kind == 0 || value.number(1) != 1 || unit.kind != 12 || len(unit.fields) != 1 || unit.field(2).kind != 12 {
					return dtype.Type{}, false
				}
			case 10:
				if value.number(1) != physicalWidth(physical) || value.field(2).kind == 0 || value.number(2) != 1 {
					return dtype.Type{}, false
				}
			case 1, 6:
				if len(value.fields) != 0 {
					return dtype.Type{}, false
				}
			default:
				return dtype.Type{}, false
			}
		}
	}
	if annotation == -1 {
		switch legacy {
		case 0:
			annotation = 1
		case 6:
			annotation = 6
		case 10:
			annotation = 8
		case 17, 18:
			annotation = 10
		case -1:
		default:
			return dtype.Type{}, false
		}
	} else if legacy != -1 && !(annotation == 1 && legacy == 0 || annotation == 6 && legacy == 6 || annotation == 8 && legacy == 10 || annotation == 10 && (physical == 1 && legacy == 17 || physical == 2 && legacy == 18)) {
		return dtype.Type{}, false
	}
	switch {
	case physical == 0 && annotation == -1:
		return dtype.BoolT(), true
	case physical == 1 && annotation == -1 || physical == 1 && annotation == 10 && (legacy == -1 || legacy == 17):
		return dtype.Int32T(), true
	case physical == 2 && annotation == -1 || physical == 2 && annotation == 10 && (legacy == -1 || legacy == 18):
		return dtype.Int64T(), true
	case physical == 4 && annotation == -1:
		return dtype.Float32T(), true
	case physical == 5 && annotation == -1:
		return dtype.Float64T(), true
	case physical == 6 && annotation == 1:
		return dtype.StringT(), true
	case physical == 1 && annotation == 6:
		return dtype.DateT(), true
	case physical == 2 && annotation == 8:
		return dtype.TimestampTZT(), true
	}
	return dtype.Type{}, false
}

func physicalWidth(typ int64) int64 {
	if typ == 1 {
		return 32
	}
	if typ == 2 {
		return 64
	}
	return 0
}

// ParquetReader scans selected flat columns into independently owned batches.
type ParquetReader struct {
	input     io.ReaderAt
	allocator *mem.Allocator
	options   ParquetOptions
	columns   []parquetColumn
	groups    []parquetGroup
	selected  []int
	group     int
	row       int64
	cursors   []parquetCursor
	closed    bool
	pruning   []PruningPredicate
}

// ColumnCount returns the number of columns in the file schema.
func (r *ParquetReader) ColumnCount() int { return len(r.columns) }

// Column returns the name, type, and nullability of a source column.
func (r *ParquetReader) Column(i int) (string, dtype.Type, bool) {
	c := r.columns[i]
	return c.name, c.typ, c.optional
}

// Close releases transient page storage and ends the scan; it does not close
// the input.
func (r *ParquetReader) Close() {
	if r == nil {
		return
	}
	for i := range r.cursors {
		r.cursors[i].close()
	}
	r.cursors = nil
	r.closed = true
}

// Next returns the next owned batch, or nil at end of file.
func (r *ParquetReader) Next(ctx context.Context) (*store.Batch, *ParquetError) {
	if r == nil || ctx == nil {
		return nil, &ParquetError{code: ParquetInvalid, cause: errors.New("nil reader or context")}
	}
	if r.closed {
		return nil, MakeParquetError(ParquetInvalid, errors.New("closed Parquet reader"))
	}
	if err := ctx.Err(); err != nil {
		return nil, &ParquetError{code: ParquetCancelled, cause: err}
	}
	for r.group < len(r.groups) && (r.row == r.groups[r.group].rows || r.row == 0 && r.groupCannotMatch(r.groups[r.group])) {
		if err := ctx.Err(); err != nil {
			return nil, MakeParquetError(ParquetCancelled, err)
		}
		for i := range r.cursors {
			c := &r.cursors[i]
			if c.rows != c.chunk.values || c.offset != c.chunk.end || c.pagePos != c.pageRows || c.nonNull != 0 || c.encoding == 0 && (c.column.typ.ID() != dtype.BOOLT && c.valuePos != len(c.values) || c.column.typ.ID() == dtype.BOOLT && (c.valuePos+7)/8 != len(c.values)) {
				return nil, c.failure(ParquetInvalid, errors.New("column chunk did not end at row-group boundary"))
			}
		}
		for i := range r.cursors {
			r.cursors[i].close()
		}
		r.cursors = nil
		r.group++
		r.row = 0
	}
	if r.group == len(r.groups) {
		return nil, nil
	}
	if r.cursors == nil {
		r.cursors = make([]parquetCursor, len(r.selected))
		for i, col := range r.selected {
			r.cursors[i] = parquetCursor{chunk: r.groups[r.group].chunks[col], offset: r.groups[r.group].chunks[col].start, column: r.columns[col], input: r.input, a: r.allocator, options: r.options, group: r.group, columnIndex: col}
		}
	}
	n := int(min(int64(r.options.BatchSize), r.groups[r.group].rows-r.row))
	vectors := make([]store.Vector, len(r.selected))
	defer func() {
		if vectors != nil {
			for i := range vectors {
				vectors[i].Release()
			}
		}
	}()
	for i := range r.selected {
		c := &r.cursors[i]
		if c.chunk.codec != 0 && c.chunk.codec != 1 {
			return nil, c.failure(ParquetUnsupported, errors.New("unsupported codec"))
		}
		if c.column.typ.ID() == dtype.STRT {
			var failure *ParquetError
			vectors[i], failure = makeParquetStringVector(ctx, c, n)
			if failure != nil {
				return nil, failure
			}
		} else {
			vectors[i] = store.MakeVector(r.allocator, n, c.column.typ, c.column.optional)
			if vectors[i].Kind() == store.VectorInvalid {
				return nil, c.failure(ParquetResourceExhausted, errors.New("vector allocation failed"))
			}
			for j := range n {
				v, yes, err := c.next(ctx)
				if err != nil {
					return nil, err
				}
				if !yes {
					vectors[i].Validity().Clear(j)
					continue
				}
				switch c.column.typ.ID() {
				case dtype.BOOLT:
					if v[0] != 0 {
						vectors[i].Bools()[j>>3] |= 1 << (j & 7)
					}
				case dtype.INT32T, dtype.DATET:
					vectors[i].I32s()[j] = int32(binary.LittleEndian.Uint32(v))
				case dtype.INT64T, dtype.TIMESTAMPTZT:
					vectors[i].I64s()[j] = int64(binary.LittleEndian.Uint64(v))
				case dtype.FLOAT32T:
					vectors[i].F32s()[j] = math.Float32frombits(binary.LittleEndian.Uint32(v))
				case dtype.FLOAT64T:
					vectors[i].F64s()[j] = math.Float64frombits(binary.LittleEndian.Uint64(v))
				}
			}
		}
		if vectors[i].Kind() == store.VectorInvalid {
			return nil, c.failure(ParquetResourceExhausted, errors.New("string allocation failed"))
		}
	}
	batch := store.MakeBatch(vectors)
	if batch == nil {
		return nil, &ParquetError{code: ParquetResourceExhausted, cause: errors.New("batch creation failed")}
	}
	vectors = nil
	r.row += int64(n)
	return batch, nil
}

func makeParquetStringVector(ctx context.Context, c *parquetCursor, n int) (store.Vector, *ParquetError) {
	values := make([][]byte, n)
	lengths := make([]int, n)
	var scratch *mem.Segment
	var copied []byte
	var valid []bool
	if c.column.optional {
		valid = make([]bool, n)
	}
	defer func() {
		if scratch != nil {
			scratch.Dec()
		}
	}()
	for j := range values {
		v, yes, err := c.next(ctx)
		if err != nil {
			return store.Vector{}, err
		}
		if !yes {
			continue
		}
		lengths[j] = len(v)
		if len(v) > cap(copied)-len(copied) {
			if len(v) > math.MaxInt-len(copied) {
				return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("string scratch overflow"))
			}
			size := max(4096, cap(copied)*2, len(copied)+len(v))
			if size < len(copied) || size < len(v) {
				return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("string scratch overflow"))
			}
			next := c.a.AllocSegTemp(size)
			if next == nil {
				return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("string scratch allocation failed"))
			}
			grown := next.AsBytes()[:len(copied)]
			copy(grown, copied)
			if scratch != nil {
				scratch.Dec()
			}
			scratch, copied = next, grown
		}
		copied = append(copied, v...)
		if valid != nil {
			valid[j] = true
		}
	}
	on := 0
	for j := range values {
		if valid != nil && !valid[j] {
			continue
		}
		values[j] = copied[on : on+lengths[j]]
		on += lengths[j]
	}
	v := store.MakeStringVector(c.a, values, valid)
	if v.Kind() == store.VectorInvalid {
		return v, c.failure(ParquetResourceExhausted, errors.New("string allocation failed"))
	}
	return v, nil
}
