package csv

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (r *CSVReader) nextASCII(ctx context.Context) (*store.Batch, *CSVError) {
	var data []byte
	var storage *mem.Segment
	var rows []int64
	var rowStorage *mem.Segment
	defer func() {
		if storage != nil {
			storage.Dec()
		}
		if rowStorage != nil {
			rowStorage.Dec()
		}
	}()
	for len(rows) < r.options.BatchSize {
		if err := ctx.Err(); err != nil {
			return nil, &CSVError{code: CSVCancelled, record: r.record + len(rows) + 1, column: -1, cause: err}
		}
		record, segment, err := r.readRawRecord()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			code := CSVReadFailure
			if errors.Is(err, errCSVRecordResource) {
				code = CSVResourceExhausted
			}
			return nil, &CSVError{code: code, record: r.record + len(rows) + 1, column: -1, cause: err}
		}
		if len(record) > cap(data)-len(data) {
			size := max(4096, cap(data)*2, len(data)+len(record))
			next := r.allocator.AllocSegTemp(size)
			if next == nil {
				if segment != nil {
					segment.Dec()
				}
				return nil, &CSVError{code: CSVResourceExhausted, record: r.record + len(rows) + 1, column: -1, cause: errCSVRecordResource}
			}
			grown := next.AsBytes()[:len(data)]
			copy(grown, data)
			if storage != nil {
				storage.Dec()
			}
			data, storage = grown, next
		}
		data = append(data, record...)
		if segment != nil {
			segment.Dec()
		}
		if len(rows) == cap(rows) {
			size := max(512, cap(rows)*16)
			next := r.allocator.AllocSegTemp(size)
			if next == nil {
				return nil, &CSVError{code: CSVResourceExhausted, record: r.record + len(rows) + 1, column: -1, cause: errCSVRecordResource}
			}
			grown := next.AsI64T()[:len(rows)]
			copy(grown, rows)
			if rowStorage != nil {
				rowStorage.Dec()
			}
			rows, rowStorage = grown, next
		}
		rows = append(rows, int64(len(data)))
	}
	if len(rows) == 0 {
		return nil, nil
	}
	vectors := make([]store.Vector, len(r.selected))
	success := false
	defer func() {
		if !success {
			for i := range vectors {
				vectors[i].Release()
			}
		}
	}()
	stringsIn := make([][][]byte, len(r.selected))
	stringValid := make([][]bool, len(r.selected))
	for output, column := range r.selected {
		if r.types[column].ID() == dtype.STRT {
			stringsIn[output] = make([][]byte, len(rows))
			if r.nullable[column] {
				stringValid[output] = make([]bool, len(rows))
			}
			continue
		}
		vectors[output] = store.MakeVector(r.allocator, len(rows), r.types[column], r.nullable[column])
		if vectors[output].Kind() == store.VectorInvalid {
			return nil, &CSVError{code: CSVResourceExhausted, record: r.record + 1, column: column, cause: errors.New("cannot allocate column")}
		}
	}
	fields := make([][]byte, 0, len(r.types))
	nullToken := []byte(r.options.NullToken)
	start := 0
	for row, end := range rows {
		var err error
		fields, err = decodeCSVFields(data[start:int(end)], byte(r.options.Comma), len(r.types), fields)
		if err != nil {
			return nil, &CSVError{code: CSVInvalidRecord, record: r.record + row + 1, column: -1, cause: err}
		}
		start = int(end)
		for output, column := range r.selected {
			raw := fields[column]
			isNull := bytes.Equal(raw, nullToken) || r.options.EmptyIsNull && len(raw) == 0
			if isNull {
				if !r.nullable[column] {
					return nil, &CSVError{code: CSVNullViolation, record: r.record + row + 1, column: column, cause: errors.New("NULL in non-nullable column")}
				}
				if stringsIn[output] == nil {
					vectors[output].Validity().Clear(row)
				}
				continue
			}
			if stringsIn[output] != nil {
				stringsIn[output][row] = raw
				if stringValid[output] != nil {
					stringValid[output][row] = true
				}
			} else if err := setCSVValue(&vectors[output], row, string(raw), r.types[column]); err != nil {
				return nil, &CSVError{code: CSVInvalidValue, record: r.record + row + 1, column: column, cause: ownCSVValueError(err)}
			}
		}
	}
	for output, column := range r.selected {
		if stringsIn[output] == nil {
			continue
		}
		vectors[output] = store.MakeStringVector(r.allocator, stringsIn[output], stringValid[output])
		if vectors[output].Kind() == store.VectorInvalid {
			return nil, &CSVError{code: CSVResourceExhausted, record: r.record + 1, column: column, cause: errors.New("cannot allocate column")}
		}
	}
	runtime.KeepAlive(data)
	batch := store.MakeBatch(vectors)
	if batch == nil {
		return nil, &CSVError{code: CSVResourceExhausted, record: r.record + 1, column: -1, cause: errors.New("cannot create batch")}
	}
	success = true
	r.record += len(rows)
	return batch, nil
}
