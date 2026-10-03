package plan

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func openSpillRecordTest(t *testing.T, ctx context.Context, a *mem.Allocator, data []byte, budget int64) *spillStream {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "record")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(data); err != nil {
		t.Fatal(err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	stream := makeSpillStream(ctx, a, file, budget)
	if stream == nil {
		file.Close()
		t.Fatal("stream allocation")
	}
	t.Cleanup(stream.release)
	return stream
}

func spillRecordTestWire(body []byte) []byte {
	wire := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint64(wire, uint64(len(body)))
	copy(wire[8:], body)
	return wire
}

func TestSpillRecordOwnedDifferential(t *testing.T) {
	const budget = 1 << 20
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	types := []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.BoolT(), dtype.DateT(), dtype.TimestampTZT(), dtype.StringT()}
	schema := MakeSchema([]string{"i32", "i64", "f32", "f64", "bool", "date", "time", "str"}, types)
	rows := packedRows{}
	defer rows.release()
	random := rand.New(rand.NewPCG(17, 93))
	f32 := []uint32{0, 1 << 31, 0x7f800000, 0xff800000, 0x7fc12345, 0x7f812345}
	f64 := []uint64{0, 1 << 63, 0x7ff0000000000000, 0xfff0000000000000, 0x7ff812345678abcd, 0x7ff012345678abcd}
	expected := make([][]Scalar, 257)
	for row := range expected {
		values := []Scalar{MakeI32Scalar(int32(random.Uint32())), MakeI64Scalar(int64(random.Uint64())), MakeF32Scalar(math.Float32frombits(f32[row%len(f32)])), MakeF64Scalar(math.Float64frombits(f64[row%len(f64)])), MakeBoolScalar(row%2 == 0), MakeDateScalar(int32(random.Uint32())), MakeTimestampTZScalar(int64(random.Uint64())), MakeStringScalar("")}
		if row%5 != 0 {
			values[7] = MakeStringScalar(strings.Repeat(string([]byte{byte(row), 0, 0xff, 'x'}), row%197))
		}
		for column := range values {
			if (row+column)%11 == 0 {
				values[column] = MakeNullScalar(types[column])
			}
		}
		if row >= len(f32) {
			values[2] = MakeF32Scalar(math.Float32frombits(random.Uint32()))
			values[3] = MakeF64Scalar(math.Float64frombits(random.Uint64()))
			if row%11 == 0 {
				values[2] = MakeNullScalar(types[2])
				values[3] = MakeNullScalar(types[3])
			}
		}
		expected[row] = values
		if !rows.appendScalars(a, values, budget) {
			t.Fatal("append")
		}
	}
	batch := rows.makeBatch(a, schema)
	if batch == nil {
		t.Fatal("batch")
	}
	defer batch.Release()
	file, err := os.CreateTemp(t.TempDir(), "owned")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	writer := makeSpillStream(context.Background(), a, file, budget)
	if writer == nil {
		t.Fatal("writer")
	}
	for row := range expected {
		if err := writer.writeRow(batch, row); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.flush(); err != nil {
		t.Fatal(err)
	}
	writer.release()
	wire, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	scope := mem.MakeAllocationScope(a, budget)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	borrowed := openSpillRecordTest(t, context.Background(), scoped, wire, budget)
	owned := openSpillRecordTest(t, context.Background(), scoped, wire, budget)
	forward := openSpillRecordTest(t, context.Background(), a, nil, budget)
	var retainedText string
	var retainedRow = -1
	var retainedBatch *store.Batch
	for row, want := range expected {
		ok, err := borrowed.nextRecord(scoped, schema, budget)
		if !ok || err != nil {
			t.Fatalf("row %d: %t %v", row, ok, err)
		}
		old, err := owned.readRow(scoped, schema, budget)
		if err != nil {
			t.Fatal(err)
		}
		at := 0
		for column, value := range want {
			got := borrowed.recordValue(schema, column)
			reference := scalarAt(old.VectorAt(column), 0)
			if got != value || got != reference {
				t.Fatalf("row %d col %d: got %#v reference %#v want %#v", row, column, got, reference, value)
			}
			if borrowed.recordColumnOffset(column) != at {
				t.Fatalf("column offset %d/%d", borrowed.recordColumnOffset(column), at)
			}
			at += 9
			if !value.IsNull() && value.Type().ID() == dtype.STRT {
				at += len(value.text)
			}
		}
		if borrowed.recordColumnOffset(schema.Len()) != at || len(borrowed.recordBytes()) != at {
			t.Fatal("body boundary")
		}
		if err := forward.writeBytes(borrowed.recordBytes()); err != nil {
			t.Fatal(err)
		}
		if retainedBatch == nil && len(want[7].text) > 12 {
			retainedBatch, retainedText, retainedRow = old.Retain(), want[7].text, row
		}
		old.Release()
		if retainedBatch != nil && scalarAt(retainedBatch.VectorAt(7), 0).text != retainedText {
			t.Fatalf("owned row %d changed after advance", retainedRow)
		}
	}
	if ok, err := borrowed.nextRecord(scoped, schema, budget); ok || err != nil {
		t.Fatalf("EOF %t %v", ok, err)
	}
	if old, err := owned.readRow(scoped, schema, budget); old != nil || err != io.EOF {
		t.Fatalf("owned EOF %v %v", old, err)
	}
	if err := forward.flush(); err != nil {
		t.Fatal(err)
	}
	forwarded, err := os.ReadFile(forward.file.Name())
	if err != nil || string(forwarded) != string(wire) {
		t.Fatalf("wire forwarding: %v", err)
	}
	if retainedBatch != nil {
		retainedBatch.Release()
	}
	borrowed.release()
	owned.release()
	if scope.Live() != 0 || scope.Peak() > budget {
		t.Fatalf("scope live=%d peak=%d", scope.Live(), scope.Peak())
	}
}

func TestSpillRecordReuseAndGrowth(t *testing.T) {
	const budget = 64 << 10
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, budget)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	schema := MakeSchema([]string{"text"}, []dtype.Type{dtype.StringT()})
	var wire []byte
	for _, length := range []int{100, 101, 10, 5000, 5001, 0, 13} {
		body := make([]byte, 9+length)
		binary.LittleEndian.PutUint64(body[1:], uint64(length))
		for i := 9; i < len(body); i++ {
			body[i] = byte(length)
		}
		wire = append(wire, spillRecordTestWire(body)...)
	}
	stream := openSpillRecordTest(t, context.Background(), scoped, wire, budget)
	var frame, offsets *mem.Segment
	for row, length := range []int{100, 101, 10, 5000, 5001, 0, 13} {
		ok, err := stream.nextRecord(scoped, schema, budget)
		if !ok || err != nil {
			t.Fatalf("row %d: %t %v", row, ok, err)
		}
		if row != 0 && row != 3 && stream.recordFrame != frame {
			t.Fatal("frame allocated without growth")
		}
		if row != 0 && stream.recordOffsets != offsets {
			t.Fatal("offsets allocated per row")
		}
		value := stream.recordValue(schema, 0)
		if len(value.text) != length || value.text != strings.Repeat(string([]byte{byte(length)}), length) {
			t.Fatal("borrowed text")
		}
		frame, offsets = stream.recordFrame, stream.recordOffsets
		if frame.Len() > budget/4 {
			t.Fatal("frame limit")
		}
		if scope.Live() != int64(frame.Len()+offsets.Len()+stream.buffer.Len()) {
			t.Fatal("unaccounted cursor payload")
		}
	}
	stream.release()
	if scope.Live() != 0 {
		t.Fatalf("leak %d", scope.Live())
	}
}

func TestSpillRecordMalformed(t *testing.T) {
	schema := MakeSchema([]string{"text"}, []dtype.Type{dtype.StringT()})
	body := make([]byte, 9)
	badNull := append([]byte(nil), body...)
	badNull[0] = 2
	badLength := append([]byte(nil), body...)
	binary.LittleEndian.PutUint64(badLength[1:], 1)
	nullString := append([]byte(nil), badLength...)
	nullString[0] = 1
	nullString = append(nullString, 'x')
	nonnullable := MakeSchemaWithNullability([]string{"text"}, []dtype.Type{dtype.StringT()}, []bool{false})
	nullBody := append([]byte(nil), body...)
	nullBody[0] = 1
	boolSchema := MakeSchema([]string{"bool"}, []dtype.Type{dtype.BoolT()})
	boolBody := append([]byte(nil), body...)
	binary.LittleEndian.PutUint64(boolBody[1:], 2)
	oversized := make([]byte, 8)
	binary.LittleEndian.PutUint64(oversized, 1<<20)
	overflow := make([]byte, 8)
	binary.LittleEndian.PutUint64(overflow, ^uint64(0))
	cases := []struct {
		name   string
		schema Schema
		wire   []byte
		want   error
	}{
		{"empty", schema, nil, nil}, {"short-header", schema, []byte{9, 0, 0}, io.ErrUnexpectedEOF},
		{"short-body", schema, spillRecordTestWire(body)[:12], io.ErrUnexpectedEOF},
		{"missing-body", schema, spillRecordTestWire(body)[:8], io.ErrUnexpectedEOF},
		{"null-tag", schema, spillRecordTestWire(badNull), errSpillFormat},
		{"string-length", schema, spillRecordTestWire(badLength), errSpillFormat},
		{"null-string-length", schema, spillRecordTestWire(nullString), errSpillFormat},
		{"trailing", schema, spillRecordTestWire(append(append([]byte(nil), body...), 'x')), errSpillFormat},
		{"short-length", schema, spillRecordTestWire(body[:8]), errSpillFormat},
		{"nonnullable", nonnullable, spillRecordTestWire(nullBody), errSpillFormat},
		{"bool-value", boolSchema, spillRecordTestWire(boolBody), errSpillFormat},
		{"oversized", schema, oversized, errSpillBudget}, {"overflow", schema, overflow, errSpillFormat},
		{"invalid-schema", Schema{}, spillRecordTestWire(body), errSpillFormat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			scope := mem.MakeAllocationScope(a, 64<<10)
			scoped := mem.MakeAllocatorWithScope(a, scope)
			stream := openSpillRecordTest(t, context.Background(), scoped, tc.wire, 64<<10)
			ok, err := stream.nextRecord(scoped, tc.schema, 64<<10)
			if ok || !errors.Is(err, tc.want) {
				t.Fatalf("got %t %v want %v", ok, err, tc.want)
			}
			if stream.recordBytes() != nil || stream.recordValue(tc.schema, 0).valid() || stream.recordColumnOffset(0) != -1 {
				t.Fatal("invalid cursor exposed")
			}
			stream.release()
			if scope.Live() != 0 {
				t.Fatalf("leak %d", scope.Live())
			}
		})
	}
}

func TestSpillRecordCancelAndBudget(t *testing.T) {
	schema := MakeSchema([]string{"text"}, []dtype.Type{dtype.StringT()})
	body := make([]byte, 109)
	binary.LittleEndian.PutUint64(body[1:], 100)
	for _, mode := range []string{"cancel-before", "cancel-advance", "scope-frame", "scope-offset", "local-budget", "closed-file"} {
		t.Run(mode, func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			limit := int64(64 << 10)
			if mode == "scope-frame" {
				limit = 1024
			}
			if mode == "scope-offset" {
				limit = 1152
			}
			scope := mem.MakeAllocationScope(a, limit)
			scoped := mem.MakeAllocatorWithScope(a, scope)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			wire := append(spillRecordTestWire(body), spillRecordTestWire(body)...)
			stream := openSpillRecordTest(t, ctx, scoped, wire, 64<<10)
			want := errSpillBudget
			local := int64(64 << 10)
			switch mode {
			case "cancel-before":
				cancel()
				want = context.Canceled
			case "cancel-advance":
				if ok, err := stream.nextRecord(scoped, schema, local); !ok || err != nil {
					t.Fatalf("first %t %v", ok, err)
				}
				cancel()
				want = context.Canceled
			case "local-budget":
				local = 1
			case "closed-file":
				stream.file.Close()
				want = os.ErrClosed
			}
			ok, err := stream.nextRecord(scoped, schema, local)
			if ok || !errors.Is(err, want) {
				t.Fatalf("got %t %v want %v", ok, err, want)
			}
			if stream.recordBytes() != nil || stream.recordValue(schema, 0).valid() {
				t.Fatal("stale record after failure")
			}
			stream.release()
			stream.release()
			if scope.Live() != 0 || scope.Peak() > scope.Limit() {
				t.Fatalf("scope live %d peak %d", scope.Live(), scope.Peak())
			}
		})
	}
}

func TestSpillRecordOddBudgetOffsets(t *testing.T) {
	const budget = 701
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, budget)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	names, types := make([]string, 17), make([]dtype.Type, 17)
	body := make([]byte, len(names)*9)
	for column := range types {
		types[column] = dtype.Int64T()
		binary.LittleEndian.PutUint64(body[column*9+1:], uint64(column))
	}
	schema := MakeSchema(names, types)
	stream := openSpillRecordTest(t, context.Background(), scoped, spillRecordTestWire(body), budget)
	if ok, err := stream.nextRecord(scoped, schema, budget); !ok || err != nil {
		t.Fatalf("record %t %v", ok, err)
	}
	if stream.recordOffsets.Len()%8 != 0 {
		t.Fatal("offset alignment")
	}
	for column := range types {
		if got := stream.recordValue(schema, column); got != MakeI64Scalar(int64(column)) {
			t.Fatalf("col %d %#v", column, got)
		}
	}
	stream.release()
	if scope.Live() != 0 || scope.Peak() > budget {
		t.Fatalf("scope live=%d peak=%d", scope.Live(), scope.Peak())
	}
}

func TestSpillRecordAdvanceReusesMetadata(t *testing.T) {
	const budget = 64 << 10
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, budget)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	schema := MakeSchema([]string{"text"}, []dtype.Type{dtype.StringT()})
	body := make([]byte, 109)
	binary.LittleEndian.PutUint64(body[1:], 100)
	var wire []byte
	for range 103 {
		wire = append(wire, spillRecordTestWire(body)...)
	}
	stream := openSpillRecordTest(t, context.Background(), scoped, wire, budget)
	if ok, err := stream.nextRecord(scoped, schema, budget); !ok || err != nil {
		t.Fatalf("initial %t %v", ok, err)
	}
	frame, offsets, live := stream.recordFrame, stream.recordOffsets, scope.Live()
	allocations := testing.AllocsPerRun(100, func() {
		if ok, err := stream.nextRecord(scoped, schema, budget); !ok || err != nil {
			t.Fatalf("advance %t %v", ok, err)
		}
		if stream.recordValue(schema, 0).text != string(body[9:]) {
			t.Fatal("value")
		}
	})
	if allocations != 0 {
		t.Fatalf("per-record Go allocations %g", allocations)
	}
	if frame != stream.recordFrame || offsets != stream.recordOffsets || scope.Live() != live {
		t.Fatal("record storage not reused")
	}
	stream.release()
	if scope.Live() != 0 {
		t.Fatalf("live=%d", scope.Live())
	}
}

func TestSpillRecordMaximalBudget(t *testing.T) {
	const budget = int64(math.MaxInt64)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, budget)
	scoped := mem.MakeAllocatorWithScope(a, scope)
	schema := MakeSchema([]string{"value"}, []dtype.Type{dtype.Int64T()})
	body := make([]byte, 9)
	binary.LittleEndian.PutUint64(body[1:], 0x0123456789abcdef)
	stream := openSpillRecordTest(t, context.Background(), scoped, spillRecordTestWire(body), budget)
	if ok, err := stream.nextRecord(scoped, schema, budget); !ok || err != nil {
		t.Fatalf("record %t %v", ok, err)
	}
	if got := stream.recordValue(schema, 0); got != MakeI64Scalar(0x0123456789abcdef) {
		t.Fatalf("value %#v", got)
	}
	stream.release()
	if scope.Live() != 0 {
		t.Fatalf("live=%d", scope.Live())
	}
}
