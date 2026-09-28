package parquet

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

// DuckDB 1.5.6, Snappy, two row groups (2048 and 2 rows).
const duckDBGroupsFixture = "UEFSMRUEFSwVMEwVAhUAAAAWVBIAAABhYmNkZWZhYmNkZWZhYmNkZWYVABWgBBVULBWAIBUEFQYVBgAAkAIcCAEAAEFt27ZyAwAAQXYfAAi2bUF6QQAAtv5jAP5jAJJjAAwBqhUAFQAVZBUwLBUEFQAVBhUGAAAyPAIAAAAEARIAAABhYmNkZWYuBgBWFgAVQBwcAAAcHAAAHBwAAAAAAIAAAAABAAQAAAABAAAAAAAAAiAAAAAABAAAAAQAABUCGSw1ABgNZHVja2RiX3NjaGVtYRUCABUMJQIYA3R4dCUAABaEIBksGRwmABwVDBkVBBkYA3R4dBUCFoAgFowFFsQBJlImCBwYEmFiY2RlZmFiY2RlZmFiY2RlZhgSYWJjZGVmYWJjZGVmYWJjZGVmFtYKFgIYEmFiY2RlZmFiY2RlZmFiY2RlZhgSYWJjZGVmYWJjZGVmYWJjZGVmEREAJp4CFV4AABaMBRaAICYIFsQBABkcJgAcFQwZFQAZGAN0eHQVAhYEFoYBFlImzAE8GBJhYmNkZWZhYmNkZWZhYmNkZWYYEmFiY2RlZmFiY2RlZmFiY2RlZhYAKBJhYmNkZWZhYmNkZWZhYmNkZWYYEmFiY2RlZmFiY2RlZmFiY2RlZhERAAAAFoYBFgQmzAEWUgAoKER1Y2tEQiB2ZXJzaW9uIHYxLjUuNiAoYnVpbGQgMDY5Y2M5ZjliNSkZHBwAAABjAQAAUEFSMQ=="

func TestParquetMultipleRowGroups(t *testing.T) {
	fixture, err := os.ReadFile("testdata/duckdb-groups.b64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := base64.StdEncoding.DecodeString(duckDBGroupsFixture)
	if !bytes.Equal(data, expected) {
		at := 0
		for at < min(len(data), len(expected)) && data[at] == expected[at] {
			at++
		}
		start := max(0, at/3*4-28)
		end := min(len(duckDBGroupsFixture), at/3*4+48)
		t.Fatalf("shared fixture differs at byte %d (file=%d, inline=%d): file %q inline %q", at, len(data), len(expected), strings.TrimSpace(string(fixture))[start:end], duckDBGroupsFixture[start:end])
	}
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	r, parseErr := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, nil, ParquetOptions{BatchSize: 31})
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	defer r.Close()
	count := 0
	for {
		batch, readErr := r.Next(context.Background())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if batch == nil {
			break
		}
		for row := range batch.Len() {
			v := batch.VectorAt(0)
			if v.Validity().IsSet(row) != (count%3 != 1) {
				t.Fatalf("wrong validity at %d", count)
			}
			if count%3 != 1 && v.Strings()[row].View() != "abcdefabcdefabcdef" {
				t.Fatalf("wrong string at %d", count)
			}
			count++
		}
		batch.Release()
	}
	if count != 2050 {
		t.Fatalf("read %d rows", count)
	}
}

func BenchmarkParquetMultipleRowGroups(b *testing.B) {
	fixture, err := os.ReadFile("testdata/duckdb-groups.b64")
	if err != nil {
		b.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(fixture)))
	if err != nil {
		b.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	input := bytes.NewReader(data)
	b.ReportAllocs()
	b.SetBytes(2050)
	for range b.N {
		r, failure := MakeParquetReaderProjected(input, int64(len(data)), a, nil, ParquetOptions{})
		if failure != nil {
			b.Fatal(failure)
		}
		for {
			batch, readErr := r.Next(context.Background())
			if readErr != nil {
				b.Fatal(readErr)
			}
			if batch == nil {
				break
			}
			batch.Release()
		}
		r.Close()
	}
}
