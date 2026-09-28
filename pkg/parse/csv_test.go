package parse

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestCSVReaderStreamingTypesAndOwnership(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	input := "name,age,date,stamp,flag\r\n" +
		"\"a long,quoted\nvalue\",42,2024-02-29,2024-02-29T03:04:05.123456Z,true\r\n" +
		"simple,\\N,1970-01-01,1969-12-31T23:59:59Z,false\r\n" +
		"\"escaped \"\"quote\"\"\",-7,1969-12-31,1970-01-01T00:00:00Z,1\r\n"
	r, err := MakeCSVReader(strings.NewReader(input), a, []dtype.Type{dtype.StringT(), dtype.Int32T(), dtype.DateT(), dtype.TimestampTZT(), dtype.BoolT()}, []bool{false, true, false, false, false}, CSVOptions{HasHeader: true, BatchSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	first, err := r.Next(context.Background())
	if err != nil || first == nil || first.Len() != 2 {
		t.Fatalf("first batch: %v", err)
	}
	if first.VectorAt(0).Strings()[0].View() != "a long,quoted\nvalue" || first.VectorAt(1).I32s()[0] != 42 || first.VectorAt(1).Validity().IsSet(1) || first.VectorAt(2).I32s()[0] != 19782 || first.VectorAt(3).I64s()[0] != time.Date(2024, 2, 29, 3, 4, 5, 123456000, time.UTC).UnixMicro() {
		t.Fatal("incorrect decoded quoted, numeric, date or timestamp values")
	}
	second, err := r.Next(context.Background())
	if err != nil || second == nil || second.Len() != 1 || second.VectorAt(0).Strings()[0].View() != "escaped \"quote\"" || second.VectorAt(1).I32s()[0] != -7 || second.VectorAt(2).I32s()[0] != -1 {
		t.Fatalf("second batch: %v", err)
	}
	if end, err := r.Next(context.Background()); end != nil || err != nil {
		t.Fatal("expected end of source")
	}
	if first.VectorAt(0).Strings()[0].View() != "a long,quoted\nvalue" {
		t.Fatal("borrowed first batch was overwritten by later reads")
	}
	second.Release()
	first.Release()
}

func TestCSVReaderDifferentialAgainstStdlib(t *testing.T) {
	rng := rand.New(rand.NewPCG(701, 21))
	for _, separator := range []rune{',', '|', 'é'} {
		var text bytes.Buffer
		writer := csv.NewWriter(&text)
		writer.Comma = separator
		for row := range 160 {
			fields := make([]string, 3)
			for col := range fields {
				length := rng.IntN(70)
				for range length {
					fields[col] += string([]rune{'a', '"', '\r', '\n', separator, '\x00', 'é'}[rng.IntN(7)])
				}
			}
			if row%13 == 0 {
				fields[1] = "\\N"
			}
			if err := writer.Write(fields); err != nil {
				t.Fatal(err)
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			t.Fatal(err)
		}
		reference := csv.NewReader(bytes.NewReader(text.Bytes()))
		reference.Comma = separator
		reference.FieldsPerRecord = 3
		var want [][]string
		for {
			row, err := reference.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			want = append(want, row)
		}
		a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
		r, err := MakeCSVReader(bytes.NewReader(text.Bytes()), a, []dtype.Type{dtype.StringT(), dtype.StringT(), dtype.StringT()}, []bool{false, true, false}, CSVOptions{Comma: separator, BatchSize: 17})
		if err != nil {
			t.Fatal(err)
		}
		on := 0
		for {
			batch, err := r.Next(context.Background())
			if err != nil {
				t.Fatalf("separator %q record %d: %v", separator, on, err)
			}
			if batch == nil {
				break
			}
			for i := range batch.Len() {
				for col := range 3 {
					null := col == 1 && want[on][col] == "\\N"
					vector := batch.VectorAt(col)
					if null {
						if vector.Validity() == nil || vector.Validity().IsSet(i) {
							t.Fatalf("row %d col %d lost NULL", on, col)
						}
					} else if got := vector.Strings()[i].View(); got != want[on][col] {
						t.Fatalf("separator %q row %d col %d: got %q want %q", separator, on, col, got, want[on][col])
					}
				}
				on++
			}
			batch.Release()
		}
		if on != len(want) {
			t.Fatalf("got %d rows, want %d", on, len(want))
		}
	}
}

func TestCSVReaderUnquotedFastPath(t *testing.T) {
	rng := rand.New(rand.NewPCG(47, 133))
	for _, separator := range []byte{',', '|'} {
		var input bytes.Buffer
		input.WriteString("#ignored comment\r\n\r\n")
		for range 257 {
			for column := range 4 {
				if column != 0 {
					input.WriteByte(separator)
				}
				for range rng.IntN(80) {
					input.WriteByte([]byte{'a', 'b', 0, 0xff, '\r'}[rng.IntN(5)])
				}
			}
			input.WriteString("\r\n")
		}
		standard := csv.NewReader(bytes.NewReader(input.Bytes()))
		standard.Comma = rune(separator)
		standard.Comment = '#'
		standard.FieldsPerRecord = 4
		var want [][]string
		for {
			row, err := standard.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			want = append(want, row)
		}
		a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
		reader, err := MakeCSVReader(bytes.NewReader(input.Bytes()), a, []dtype.Type{dtype.StringT(), dtype.StringT(), dtype.StringT(), dtype.StringT()}, []bool{false, false, false, false}, CSVOptions{Comma: rune(separator), Comment: '#', BatchSize: 31})
		if err != nil {
			t.Fatal(err)
		}
		on := 0
		for {
			batch, err := reader.Next(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if batch == nil {
				break
			}
			if reader.fallback != nil {
				t.Fatal("unquoted CSV unexpectedly fell back")
			}
			for row := range batch.Len() {
				for column := range 4 {
					if got := batch.VectorAt(column).Strings()[row].View(); got != want[on][column] {
						t.Fatalf("row %d col %d: got %q want %q", on, column, got, want[on][column])
					}
				}
				on++
			}
			batch.Release()
		}
		if on != 257 {
			t.Fatalf("got %d rows", on)
		}
	}
}

func TestCSVReaderStructuredErrorsAndBudget(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	for _, test := range []struct {
		data     string
		types    []dtype.Type
		nullable []bool
		code     CSVErrorCode
		column   int
	}{
		{"x,1\n", []dtype.Type{dtype.Int32T(), dtype.Int32T()}, []bool{false, false}, CSVInvalidValue, 0},
		{"\\N\n", []dtype.Type{dtype.Int32T()}, []bool{false}, CSVNullViolation, 0},
		{"x,y\n", []dtype.Type{dtype.StringT()}, []bool{false}, CSVInvalidRecord, -1},
		{"\"unterminated\n", []dtype.Type{dtype.StringT()}, []bool{false}, CSVInvalidRecord, -1},
	} {
		r, err := MakeCSVReader(strings.NewReader(test.data), a, test.types, test.nullable, CSVOptions{})
		if err != nil {
			t.Fatal(err)
		}
		batch, failure := r.Next(context.Background())
		if batch != nil || failure == nil || failure.Code() != test.code || failure.Column() != test.column || failure.Record() != 1 {
			t.Fatalf("input %q: got %v, want %v", test.data, failure, test.code)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := MakeCSVReader(strings.NewReader("a\n"), a, []dtype.Type{dtype.StringT()}, []bool{false}, CSVOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, failure := r.Next(ctx); failure == nil || failure.Code() != CSVCancelled || !errors.Is(failure, context.Canceled) {
		t.Fatal("reader ignored cancellation")
	}
	scope := mem.MakeAllocationScope(a, 32)
	view := mem.MakeAllocatorWithScope(a, scope)
	r, err = MakeCSVReader(strings.NewReader(strings.Repeat("a", 100)+"\n"), view, []dtype.Type{dtype.StringT()}, []bool{false}, CSVOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if batch, failure := r.Next(context.Background()); batch != nil || failure == nil || failure.Code() != CSVResourceExhausted || scope.Live() != 0 {
		t.Fatalf("budget failure leaked memory: %v live=%d", failure, scope.Live())
	}
	r, err = MakeCSVReader(strings.NewReader("\""+strings.Repeat("x", 70<<10)+"\",1\n"), view, []dtype.Type{dtype.StringT(), dtype.Int32T()}, []bool{false, false}, CSVOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if batch, failure := r.Next(context.Background()); batch != nil || failure == nil || failure.Code() != CSVResourceExhausted || scope.Live() != 0 {
		t.Fatalf("record buffer exceeded budget: %v live=%d", failure, scope.Live())
	}
	boundaryScope := mem.MakeAllocationScope(a, 70<<10)
	boundaryView := mem.MakeAllocatorWithScope(a, boundaryScope)
	r, err = MakeCSVReader(strings.NewReader("\""+strings.Repeat("x", 70<<10)+"\",1\n"), boundaryView, []dtype.Type{dtype.StringT(), dtype.Int32T()}, []bool{false, false}, CSVOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if batch, failure := r.Next(context.Background()); batch != nil || failure == nil || failure.Code() != CSVResourceExhausted || boundaryScope.Live() != 0 {
		t.Fatalf("record buffer growth leaked memory: %v live=%d", failure, boundaryScope.Live())
	}
}

func TestCSVReaderInvalidValueRetainsError(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	scope := mem.MakeAllocationScope(a, 1<<16)
	view := mem.MakeAllocatorWithScope(a, scope)
	r, err := MakeCSVReader(strings.NewReader("bad-integer\n"), view, []dtype.Type{dtype.Int32T()}, []bool{false}, CSVOptions{})
	if err != nil {
		t.Fatal(err)
	}
	batch, failure := r.Next(context.Background())
	var number *strconv.NumError
	if batch != nil || failure == nil || failure.Code() != CSVInvalidValue || !errors.As(failure, &number) || number.Num != "bad-integer" || scope.Live() != 0 {
		t.Fatalf("invalid number: %v, live=%d", failure, scope.Live())
	}
	segment := view.AllocSegTemp(4096)
	if segment == nil {
		t.Fatal("cannot reuse released scratch memory")
	}
	for i := range segment.AsBytes() {
		segment.AsBytes()[i] = 'x'
	}
	if number.Num != "bad-integer" || !strings.Contains(failure.Error(), "bad-integer") {
		t.Fatalf("error references released record buffer: %v", failure)
	}
	segment.Dec()
}

func TestCSVReaderProjectedColumns(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	types := []dtype.Type{dtype.StringT(), dtype.Int32T(), dtype.Int64T()}
	nullable := []bool{false, false, false}
	r, err := MakeCSVReaderProjected(strings.NewReader("name,n,unused\nalpha,7,not-an-integer\n"), a, types, nullable, []int{1, 0}, CSVOptions{HasHeader: true})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := r.Next(context.Background())
	if err != nil || batch == nil || batch.NVectors() != 2 || batch.VectorAt(0).I32s()[0] != 7 || batch.VectorAt(1).Strings()[0].View() != "alpha" {
		t.Fatalf("projected CSV batch: %v", err)
	}
	batch.Release()
	if _, err := MakeCSVReaderProjected(strings.NewReader("x\n"), a, types, nullable, []int{1, 1}, CSVOptions{}); err == nil || err.Code() != CSVInvalidConfiguration {
		t.Fatal("accepted repeated source offset")
	}
	r, err = MakeCSVReaderProjected(strings.NewReader("x,bad,3\n"), a, types, nullable, []int{1}, CSVOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if batch, err := r.Next(context.Background()); batch != nil || err == nil || err.Code() != CSVInvalidValue || err.Column() != 1 {
		t.Fatalf("projected error column: %v", err)
	}
}

func TestCSVReaderLongMultilineRecord(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 18}, []int{1 << 18})
	value := strings.Repeat("a", 70<<10) + "\nquoted,second line"
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if err := writer.Write([]string{value, "42"}); err != nil {
		t.Fatal(err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	reader, err := MakeCSVReader(bytes.NewReader(buffer.Bytes()), a, []dtype.Type{dtype.StringT(), dtype.Int32T()}, []bool{false, false}, CSVOptions{BatchSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := reader.Next(context.Background())
	if err != nil || batch == nil || batch.VectorAt(0).Strings()[0].View() != value || batch.VectorAt(1).I32s()[0] != 42 {
		t.Fatalf("long quoted record: %v", err)
	}
	if reader.fallback != nil {
		t.Fatal("quoted record switched the entire stream to the fallback reader")
	}
	batch.Release()
	if batch, err := reader.Next(context.Background()); batch != nil || err != nil {
		t.Fatal("long record did not terminate")
	}
}

func TestCSVReaderChunkBoundariesAndQuotedRecovery(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 19}, []int{1 << 19})
	scope := mem.MakeAllocationScope(a, 1<<19)
	view := mem.MakeAllocatorWithScope(a, scope)
	long := strings.Repeat("x", 70<<10)
	want := [][]string{
		{strings.Repeat("a", 62) + "\",part\nnext\"", "1"},
		{"plain", "2"},
		{long, "3"},
		{"\"across\nlines\"", "4"},
		{"last", "5"},
	}
	var input bytes.Buffer
	input.WriteByte('#')
	input.WriteString(strings.Repeat("#\"", 35<<10))
	input.WriteString("\r\n\r\n")
	writer := csv.NewWriter(&input)
	for _, record := range want {
		if err := writer.Write(record); err != nil {
			t.Fatal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}
	data := bytes.TrimSuffix(input.Bytes(), []byte{'\n'})
	reader, err := MakeCSVReader(bytes.NewReader(data), view, []dtype.Type{dtype.StringT(), dtype.Int32T()}, []bool{false, false}, CSVOptions{Comment: '#', BatchSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	row := 0
	for {
		batch, err := reader.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if batch == nil {
			break
		}
		if reader.fallback != nil {
			t.Fatal("ASCII stream switched to the fallback reader")
		}
		for i := range batch.Len() {
			if got := batch.VectorAt(0).Strings()[i].View(); got != want[row][0] || batch.VectorAt(1).I32s()[i] != int32(row+1) {
				t.Fatalf("row %d: got %q, want %q", row, got, want[row][0])
			}
			row++
		}
		batch.Release()
	}
	if row != len(want) || scope.Live() != 0 {
		t.Fatalf("read %d rows, want %d; live bytes: %d", row, len(want), scope.Live())
	}
}
