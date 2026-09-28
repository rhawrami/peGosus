package parquet

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

// Apache parquet-testing data/datapage_v2.snappy.parquet; only its flat
// string chunk is decoded because the file also contains a nested list.
const apacheV2Fixture = "UEFSMRUEFQ4VEkwVAhUAAAAHGAMAAABhYmMVBhUIFQxcFQoVAhUKFRAVBBUALBgDYWJjGANhYmMWAgAAAAMXAgQAAxUGFRQVGFwVChUAFQoVChUAFQAsGAQFAAAAGAQBAAAAFgAAAAAKJIABBAUCAgAAAAAVBBVAFTBMFQgVAAAAIAAACQEAQAkHAAgNCCQQQAAAAAAAABRAFQYVCBUMXBUKFQAVChUQFQAVACwYCAAAAAAAABRAGAgAAAAAAAAAQBYAAAAABAwCA+QAFQYVDBUQXBUKFQAVChUGFQAVACwYAQEYAQAWAAAAAAYUAgAAAAMXFQQVGBUcTBUGFQAAAAwsAQAAAAIAAAADAAAAFQYVGBUcXBUUFQQVChUQFQoVBiwYBAMAAAAYBAEAAAAWBAAAAAXGAgUqqAoABAwCAyRJFQIZjEgMc3Bhcmtfc2NoZW1hFQoAFQwlAhgBYSUAABUCJQAYAWIAFQolABgBYwAVACUAGAFkADUCGAFlFQIVBgA1BBgEbGlzdBUCABUCJQAYB2VsZW1lbnQAFgoZHBlcJggcFQwZJQAQGRgBYRUCFgoWdhZ+Jgg8GANhYmMYA2FiYxYCAAAAJoYBHBUCGRUKGRgBYhUCFgoWXhZiJoYBPBgEBQAAABgEAQAAABYAAAAAJugBHBUKGSUAEBkYAWMVAhYKFrwBFrABJugBPBgIAAAAAAAAFEAYCAAAAAAAAABAFgAAAAAmmAMcFQAZFQYZGAFkFQIWChZKFk4mmAM8GAEBGAEAFgAAAAAm5gMcFQIZJQAQGTgBZQRsaXN0B2VsZW1lbnQVAhYUFpQBFpwBJuYDPBgEAwAAABgEAQAAABYEAAAAFu4EFgoAGRwYKW9yZy5hcGFjaGUuc3Bhcmsuc3FsLnBhcnF1ZXQucm93Lm1ldGFkYXRhGP0CeyJ0eXBlIjoic3RydWN0IiwiZmllbGRzIjpbeyJuYW1lIjoiYSIsInR5cGUiOiJzdHJpbmciLCJudWxsYWJsZSI6dHJ1ZSwibWV0YWRhdGEiOnt9fSx7Im5hbWUiOiJiIiwidHlwZSI6ImludGVnZXIiLCJudWxsYWJsZSI6ZmFsc2UsIm1ldGFkYXRhIjp7fX0seyJuYW1lIjoiYyIsInR5cGUiOiJkb3VibGUiLCJudWxsYWJsZSI6ZmFsc2UsIm1ldGFkYXRhIjp7fX0seyJuYW1lIjoiZCIsInR5cGUiOiJib29sZWFuIiwibnVsbGFibGUiOmZhbHNlLCJtZXRhZGF0YSI6e319LHsibmFtZSI6ImUiLCJ0eXBlIjp7InR5cGUiOiJhcnJheSIsImVsZW1lbnRUeXBlIjoiaW50ZWdlciIsImNvbnRhaW5zTnVsbCI6ZmFsc2V9LCJudWxsYWJsZSI6dHJ1ZSwibWV0YWRhdGEiOnt9fV19ABhJcGFycXVldC1tciB2ZXJzaW9uIDEuOC4xIChidWlsZCA0YWJhNGRhZTdiYjBkNGVkYmNmNzkyM2FlMTMzOWYyOGZkM2Y3ZmNmKQBEAwAAUEFSMQ=="

// Apache parquet-testing data/datapage_v2_empty_datapage.snappy.parquet.
const apacheV2EmptyValuesFixture = "UEFSMRUGFQQVBFwVAhUCFQIVABUEFQAAAAMAGREBGRgAGRgAFQIZFgIAGRwWCBUuFgAAABUCGSxIDHNwYXJrX3NjaGVtYRUCABUIJQIYBXZhbHVlABYCGRwZHCYIHBUIGRUAGRgFdmFsdWUVAhYCFi4WLiYIPDYCABkcFQYVABUCAAAWVBUUFjYVHgAWLhYCJggWLhQAABksGBhvcmcuYXBhY2hlLnNwYXJrLnZlcnNpb24YBTMuNS41ABgpb3JnLmFwYWNoZS5zcGFyay5zcWwucGFycXVldC5yb3cubWV0YWRhdGEYWnsidHlwZSI6InN0cnVjdCIsImZpZWxkcyI6W3sibmFtZSI6InZhbHVlIiwidHlwZSI6ImZsb2F0IiwibnVsbGFibGUiOnRydWUsIm1ldGFkYXRhIjp7fX1dfQAYSnBhcnF1ZXQtbXIgdmVyc2lvbiAxLjEzLjEgKGJ1aWxkIGRiNDE4MzEwOWQ1YjczNGVjNTkzMGQ4NzBjZGFlMTYxZTQwOGRkYmEpGRwcAAAAYQEAAFBBUjE="

func TestParquetApacheV2FlatChunk(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(apacheV2Fixture)
	if err != nil {
		t.Fatal(err)
	}
	if reader, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192}), []int{0}, ParquetOptions{}); reader != nil || failure == nil || failure.Code() != ParquetUnsupported {
		t.Fatalf("nested schema was accepted: %v", failure)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	c := parquetCursor{chunk: parquetChunk{start: 4, end: 67, codec: 1, values: 5}, offset: 4, input: bytes.NewReader(data), a: a, options: ParquetOptions{MaxPageBytes: parquetPageLimit, MaxDecodedPageBytes: parquetDecodedLimit}, column: parquetColumn{name: "a", typ: dtype.StringT(), optional: true}}
	defer c.close()
	for i, want := range []string{"abc", "abc", "abc", "", "abc"} {
		value, valid, failure := c.next(context.Background())
		if failure != nil {
			t.Fatal(failure)
		}
		if valid != (i != 3) || valid && string(value) != want {
			t.Fatalf("row %d: %q valid %v", i, value, valid)
		}
	}
}

func TestParquetApacheV2EmptySnappyValues(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(apacheV2EmptyValuesFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	r, failure := MakeParquetReader(bytes.NewReader(data), int64(len(data)), a, ParquetOptions{})
	if failure != nil {
		t.Fatal(failure)
	}
	defer r.Close()
	batch, readErr := r.Next(context.Background())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if batch == nil || batch.Len() != 1 || batch.VectorAt(0).NiN() != 1 {
		t.Fatal("all-null V2 page lost its row")
	}
	batch.Release()
	batch, readErr = r.Next(context.Background())
	if batch != nil || readErr != nil {
		t.Fatalf("expected end of all-null file, got batch %v error %v", batch, readErr)
	}
}
