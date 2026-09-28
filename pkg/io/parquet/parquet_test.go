package parquet

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

// Generated independently with DuckDB 1.5.6, COPY ... (FORMAT parquet,
// COMPRESSION uncompressed). The middle string is NULL.
const duckDBFlatFixture = "UEFSMRUAFSQVJCwVBhUAFQYVBgAAAgAAAAYBAAAAAAEAAAACAAAAFQAVlgEVlgEsFQYVABUGFQYAACEAAABBBQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAPAAAAYWFhYWFhYWFhYWFhYWFhDwAAAGNjY2NjY2NjY2NjY2NjYxUCGTw1ABgNZHVja2RiX3NjaGVtYRUEABUCJQIYAmlkJSIAFQwlAhgDdHh0JQAAFgYZHBksJgAcFQIZFQAZGAJpZBUAFgYWRhZGJgg8GAQCAAAAGAQAAAAAFgAoBAIAAAAYBAAAAAAREQAAACYAHBUMGRUAGRgDdHh0FQAWBha8ARa8ASZOPBgPY2NjY2NjY2NjY2NjY2NjGA9hYWFhYWFhYWFhYWFhYWEWAigPY2NjY2NjY2NjY2NjY2NjGA9hYWFhYWFhYWFhYWFhYWEREQAAABaCAhYGJggWggIAKChEdWNrREIgdmVyc2lvbiB2MS41LjYgKGJ1aWxkIDA2OWNjOWY5YjUpGSwcAAAcAAAAEAEAAFBBUjE="

const duckDBNullFixture = "UEFSMRUAFQwVDCwVBhUAFQYVBgAAAgAAAAYAFQIZLDUAGA1kdWNrZGJfc2NoZW1hFQIAFQIlAhgBdiUiABYGGRwZHCYAHBUCGRUAGRgBdhUAFgYWLhYuJgg8NgYAAAAWLhYGJggWLgAoKER1Y2tEQiB2ZXJzaW9uIHYxLjUuNiAoYnVpbGQgMDY5Y2M5ZjliNSkZHBwAAAB9AAAAUEFSMQ=="
const duckDBEmptyFixture = "UEFSMRUCGSw1ABgNZHVja2RiX3NjaGVtYRUCABUCJQIYAXYlIgAWABkMKChEdWNrREIgdmVyc2lvbiB2MS41LjYgKGJ1aWxkIDA2OWNjOWY5YjUpGRwcAAAAVgAAAFBBUjE="
const duckDBFloatFixture = "UEFSMRUAFSQVJCwVBhUAFQYVBgAAAgAAAAYBAAAAgAAAwH8AAIB/FQAVPBU8LBUGFQAVBhUGAAACAAAABgEAAAAAAAAAgAAAAAAAAPh/AAAAAAAA8P8VAhk8NQAYDWR1Y2tkYl9zY2hlbWEVBAAVCCUCGAFmABUKJQIYAWQAFgYZHBksJgAcFQgZFQAZGAFmFQAWBhZGFkYmCDw2AAAAACYAHBUKGRUAGRgBZBUAFgYWXhZeJk48NgAAAAAWpAEWBiYIFqQBACgoRHVja0RCIHZlcnNpb24gdjEuNS42IChidWlsZCAwNjljYzlmOWI1KRksHAAAHAAAAKQAAABQQVIx"

// DuckDB 1.5.6, all supported primitive carriers, Snappy compressed.
const duckDBTypesFixture = "UEFSMRUAFSQVKCwVBhUAFQYVBgAAEkQCAAAABgEAAAAAAQAAAAIAAAAVABU8FSwsFQYVABUGFQYAAB4YAgAAAAYBAA0BEQkcAgAAAAAAAAAVABUkFSgsFQYVABUGFQYAABJEAgAAAAYBAAAAAAAAgD8AAABAFQAVPBUuLBUGFQAVBhUGAAAeGAIAAAAGAQAyAQAk8D8AAAAAAAAAQBUAFQ4VEiwVBhUAFQYVBgAABxgCAAAABgEFFQAVJBUoLBUGFQAVBhUGAAASRAIAAAAGAQEAAAACAAAAAwAAABUAFTwVLCwVBhUAFQYVBgAAHhgCAAAABgEADQERCRwCAAAAAAAAABUAFZYBFTQsFQYVABUGFQYAAEsYIQAAAEEFAHYBABgPAAAAYWJjLgMAShMAFQIZnDUAGA1kdWNrZGJfc2NoZW1hFRAAFQIlAhgBeCUiABUEJQIYAXklJAAVCCUCGAFmABUKJQIYAWQAFQAlAhgBYgAVAiUCGAJkdCUMABUEJQIYAnRzJRRMjBEcLAAAAAAAFQwlAhgDdHh0JQAAFgYZHBmMJgAcFQIZFQAZGAF4FQIWBhZGFkomCDwYBAIAAAAYBAAAAAAWACgEAgAAABgEAAAAABERAAAAJgAcFQQZFQAZGAF5FQIWBhZeFk4mUjwYCAIAAAAAAAAAGAgAAAAAAAAAABYAKAgCAAAAAAAAABgIAAAAAAAAAAAREQAAACYAHBUIGRUAGRgBZhUCFgYWRhZKJqABPBgEAAAAQBgEAAAAABYAKAQAAABAGAQAAAAAEREAAAAmABwVChkVABkYAWQVAhYGFl4WUCbqATwYCAAAAAAAAABAGAgAAAAAAAAAABYAKAgAAAAAAAAAQBgIAAAAAAAAAAAREQAAACYAHBUAGRUAGRgBYhUCFgYWMBY0JroCPBgBARgBABYAKAEBGAEAEREAAAAmABwVAhkVABkYAmR0FQIWBhZGFkom7gI8GAQDAAAAGAQBAAAAFgAoBAMAAAAYBAEAAAAREQAAACYAHBUEGRUAGRgCdHMVAhYGFl4WTia4AzwYCAIAAAAAAAAAGAgAAAAAAAAAABYAKAgCAAAAAAAAABgIAAAAAAAAAAAREQAAACYAHBUMGRUAGRgDdHh0FQIWBha6ARZYJoYEPBgPYWJjYWJjYWJjYWJjYWJjGA9hYmNhYmNhYmNhYmNhYmMWAigPYWJjYWJjYWJjYWJjYWJjGA9hYmNhYmNhYmNhYmNhYmMREQAAABbWBRYGJggW1gQAKChEdWNrREIgdmVyc2lvbiB2MS41LjYgKGJ1aWxkIDA2OWNjOWY5YjUpGYwcAAAcAAAcAAAcAAAcAAAcAAAcAAAcAAAA0AIAAFBBUjE="

type parquetProtectedReader struct{ data []byte }

type parquetCountingReader struct {
	data      []byte
	pageReads int
}

func (r *parquetCountingReader) ReadAt(p []byte, offset int64) (int, error) {
	if offset >= 4 && offset < 39 {
		r.pageReads++
	}
	return bytes.NewReader(r.data).ReadAt(p, offset)
}

func (r parquetProtectedReader) ReadAt(p []byte, offset int64) (int, error) {
	if offset < 133 && offset+int64(len(p)) > 39 {
		return 0, errors.New("read unselected column")
	}
	return bytes.NewReader(r.data).ReadAt(p, offset)
}

var _ io.ReaderAt = parquetProtectedReader{}

func TestParquetDuckDBFlat(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBFlatFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	r, parseErr := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, nil, ParquetOptions{BatchSize: 2})
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	defer r.Close()
	var strings []string
	for {
		batch, readErr := r.Next(context.Background())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if batch == nil {
			break
		}
		for i := range batch.Len() {
			if batch.VectorAt(1).Validity().IsSet(i) {
				strings = append(strings, string([]byte(batch.VectorAt(1).Strings()[i].View())))
			} else {
				strings = append(strings, "NULL")
			}
		}
		batch.Release()
	}
	if len(strings) != 3 || strings[0] != "aaaaaaaaaaaaaaa" || strings[1] != "NULL" || strings[2] != "ccccccccccccccc" {
		t.Fatalf("decoded strings %v", strings)
	}
}

func TestParquetDuckDBTypes(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBTypesFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	r, parseErr := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, nil, ParquetOptions{BatchSize: 2})
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	defer r.Close()
	count := 0
	for {
		b, readErr := r.Next(context.Background())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if b == nil {
			break
		}
		if b.NVectors() != 8 {
			t.Fatal("wrong vector count")
		}
		for row := range b.Len() {
			i := count
			if b.VectorAt(0).I32s()[row] != int32(i) || b.VectorAt(1).I64s()[row] != int64(i) || b.VectorAt(2).F32s()[row] != float32(i) || b.VectorAt(3).F64s()[row] != float64(i) || (b.VectorAt(4).Bools()[row>>3]&(1<<(row&7)) != 0) != (i%2 == 0) || b.VectorAt(5).I32s()[row] != int32(i+1) || b.VectorAt(6).I64s()[row] != int64(i) {
				t.Fatalf("row %d decoded incorrectly", i)
			}
			if i == 1 {
				if b.VectorAt(7).Validity().IsSet(row) {
					t.Fatal("missing null")
				}
			} else if b.VectorAt(7).Strings()[row].View() != "abcabcabcabcabc" {
				t.Fatal("wrong string")
			}
			count++
		}
		b.Release()
	}
	if count != 3 {
		t.Fatalf("read %d rows", count)
	}
}

func TestParquetInvalidAndBudgetedInput(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBFlatFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	for _, input := range [][]byte{nil, data[:8], append([]byte(nil), data[:len(data)-1]...), append([]byte(nil), data...)} {
		if len(input) == len(data) {
			input[len(input)-1] = 0
		}
		r, failure := MakeParquetReaderProjected(bytes.NewReader(input), int64(len(input)), a, nil, ParquetOptions{})
		if r != nil || failure == nil || failure.Code() != ParquetInvalid {
			t.Fatalf("accepted broken footer, reader=%v error=%v", r, failure)
		}
	}
	if r, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, nil, ParquetOptions{MaxFooterBytes: 1}); r != nil || failure == nil {
		t.Fatal("oversized footer accepted")
	}
	budget := mem.MakeAllocationScope(a, 16)
	view := mem.MakeAllocatorWithScope(a, budget)
	if r, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), view, nil, ParquetOptions{}); r != nil || failure == nil || failure.Code() != ParquetResourceExhausted {
		t.Fatalf("budget admission: %v", failure)
	}
	r, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, []int{0}, ParquetOptions{MaxPageBytes: 2})
	if failure != nil {
		t.Fatal(failure)
	}
	defer r.Close()
	if b, readErr := r.Next(context.Background()); b != nil || readErr == nil || readErr.Code() != ParquetResourceExhausted {
		t.Fatalf("page bound: %v", readErr)
	}
}

func TestParquetProjectionSkipsUnselectedPageIO(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBFlatFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	r, failure := MakeParquetReaderProjected(parquetProtectedReader{data}, int64(len(data)), a, []int{0}, ParquetOptions{})
	if failure != nil {
		t.Fatal(failure)
	}
	defer r.Close()
	batch, readErr := r.Next(context.Background())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if batch == nil || batch.NVectors() != 1 || batch.VectorAt(0).I32s()[2] != 2 {
		t.Fatal("projected column missing")
	}
	batch.Release()
}

func TestParquetPageHeaderPrefetchReused(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBFlatFixture)
	if err != nil {
		t.Fatal(err)
	}
	source := &parquetCountingReader{data: data}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	r, failure := MakeParquetReaderProjected(source, int64(len(data)), a, []int{0}, ParquetOptions{})
	if failure != nil {
		t.Fatal(failure)
	}
	defer r.Close()
	batch, readErr := r.Next(context.Background())
	if readErr != nil {
		t.Fatal(readErr)
	}
	batch.Release()
	if source.pageReads != 1 {
		t.Fatalf("one page required %d range reads", source.pageReads)
	}
}

func TestParquetAllNullAndEmpty(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	for _, fixture := range []struct {
		encoded string
		count   int
	}{{duckDBNullFixture, 3}, {duckDBEmptyFixture, 0}} {
		data, err := base64.StdEncoding.DecodeString(fixture.encoded)
		if err != nil {
			t.Fatal(err)
		}
		r, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, nil, ParquetOptions{})
		if failure != nil {
			t.Fatal(failure)
		}
		b, readErr := r.Next(context.Background())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if fixture.count == 0 && b != nil {
			t.Fatal("empty file produced rows")
		}
		if fixture.count > 0 {
			if b == nil || b.Len() != fixture.count || b.VectorAt(0).NiN() != fixture.count {
				t.Fatal("all-null column lost validity")
			}
			b.Release()
			b, readErr = r.Next(context.Background())
			if readErr != nil || b != nil {
				t.Fatalf("all-null reader did not finish: %v", readErr)
			}
		}
		r.Close()
	}
}

func TestParquetFloatingPayloads(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBFloatFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	r, failure := MakeParquetReader(bytes.NewReader(data), int64(len(data)), a, ParquetOptions{})
	if failure != nil {
		t.Fatal(failure)
	}
	defer r.Close()
	batch, readErr := r.Next(context.Background())
	if readErr != nil {
		t.Fatal(readErr)
	}
	defer batch.Release()
	f, d := batch.VectorAt(0).F32s(), batch.VectorAt(1).F64s()
	if !math.Signbit(float64(f[0])) || !math.Signbit(d[0]) || !math.IsNaN(float64(f[1])) || !math.IsNaN(d[1]) || !math.IsInf(float64(f[2]), 1) || !math.IsInf(d[2], -1) {
		t.Fatalf("float payloads changed: %v %v", f, d)
	}
}

func TestParquetCorruptBytesNeverPanic(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBFlatFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	for at := 4; at < len(data)-8; at += 3 {
		changed := append([]byte(nil), data...)
		changed[at] ^= 0xff
		view := mem.MakeAllocatorWithScope(a, mem.MakeAllocationScope(a, 1<<20))
		r, failure := MakeParquetReaderProjected(bytes.NewReader(changed), int64(len(changed)), view, nil, ParquetOptions{})
		if failure != nil {
			continue
		}
		for range 4 {
			batch, readErr := r.Next(context.Background())
			if batch != nil {
				batch.Release()
			}
			if batch == nil || readErr != nil {
				break
			}
		}
		r.Close()
	}
}
