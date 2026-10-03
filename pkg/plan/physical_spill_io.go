package plan

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

var errSpillBudget = errors.New("spill working set exceeds query memory budget")
var errSpillFormat = errors.New("invalid temporary spill record")

type spillStream struct {
	file                        *os.File
	buffer                      *mem.Segment
	start, end                  int
	ctx                         context.Context
	recordFrame                 *mem.Segment
	recordOffsets               *mem.Segment
	recordLength, recordColumns int
	recordReady                 bool
}

func makeSpillStream(ctx context.Context, a *mem.Allocator, file *os.File, budget int64) *spillStream {
	buffer := a.AllocSegTemp(int(min(int64(32<<10), max(int64(256), budget/64))))
	if buffer == nil {
		return nil
	}
	return &spillStream{file: file, buffer: buffer, ctx: ctx}
}

func (s *spillStream) release() {
	if s.recordFrame != nil {
		s.recordFrame.Dec()
		s.recordFrame = nil
	}
	if s.recordOffsets != nil {
		s.recordOffsets.Dec()
		s.recordOffsets = nil
	}
	s.recordReady = false
	s.recordLength, s.recordColumns = 0, 0
	if s.buffer != nil {
		s.buffer.Dec()
		s.buffer = nil
	}
	if s.file != nil {
		s.file.Close()
		s.file = nil
	}
}

func (s *spillStream) flush() error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.end == 0 {
		return nil
	}
	n, err := s.file.Write(s.buffer.AsBytes()[:s.end])
	if err == nil && n != s.end {
		err = io.ErrShortWrite
	}
	if err == nil {
		s.end = 0
	}
	return err
}

func (s *spillStream) write(data []byte) error {
	for len(data) != 0 {
		if s.end == s.buffer.Len() {
			if err := s.flush(); err != nil {
				return err
			}
		}
		n := copy(s.buffer.AsBytes()[s.end:], data)
		s.end += n
		data = data[n:]
	}
	return nil
}

func (s *spillStream) read(data []byte) error {
	on := 0
	for on < len(data) {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		if s.start == s.end {
			n, err := s.file.Read(s.buffer.AsBytes())
			s.start, s.end = 0, n
			if n == 0 {
				if err == io.EOF && on > 0 {
					return io.ErrUnexpectedEOF
				}
				return err
			}
		}
		n := copy(data[on:], s.buffer.AsBytes()[s.start:s.end])
		s.start += n
		on += n
	}
	return nil
}

func (s *spillStream) writeRow(batch *store.Batch, row int) error {
	length := uint64(batch.NVectors()) * 9
	for column := range batch.NVectors() {
		v := batch.VectorAt(column)
		if v.TypeID() == dtype.STRT && (v.Validity() == nil || v.Validity().IsSet(row)) {
			length += uint64(v.StringAt(row).Len())
		}
	}
	var header [9]byte
	binary.LittleEndian.PutUint64(header[:8], length)
	if err := s.write(header[:8]); err != nil {
		return err
	}
	for column := range batch.NVectors() {
		v := scalarAt(batch.VectorAt(column), row)
		clear(header[:])
		if v.IsNull() {
			header[0] = 1
		}
		bits := v.bits
		if v.Type().ID() == dtype.STRT {
			bits = uint64(len(v.text))
		}
		binary.LittleEndian.PutUint64(header[1:], bits)
		if err := s.write(header[:]); err != nil {
			return err
		}
		if !v.IsNull() && v.Type().ID() == dtype.STRT {
			if err := s.write(borrowedStringBytes(v.text)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *spillStream) readRow(a *mem.Allocator, schema Schema, budget int64) (*store.Batch, error) {
	var header [8]byte
	if err := s.read(header[:]); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint64(header[:])
	if length < uint64(schema.Len())*9 || length > uint64(^uint(0)>>1) {
		return nil, errSpillFormat
	}
	if length > uint64(max(int64(0), budget/4)) {
		return nil, errSpillBudget
	}
	frame := a.AllocSegTemp(int(length))
	if frame == nil {
		return nil, errSpillBudget
	}
	defer frame.Dec()
	if err := s.read(frame.AsBytes()); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return decodeSpillFrame(a, schema, frame, int(length))
}

func spillExecutionError(ctx context.Context, err error) ExecutionResult {
	if cause := ctx.Err(); cause != nil {
		return ExecutionResult{code: ExecutionCancelled, cause: cause}
	}
	if errors.Is(err, errSpillBudget) {
		return ExecutionResult{code: ExecutionResourceExhausted, cause: err}
	}
	return ExecutionResult{code: ExecutionFailed, cause: err}
}
