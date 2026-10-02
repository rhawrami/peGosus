package parquet

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestParquetMetadataCacheExactIdentity(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBFlatFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	cache := MakeParquetMetadataCache()
	open := func(input []byte) *ParquetReader {
		t.Helper()
		r, failure := MakeParquetReaderProjectedCached(bytes.NewReader(input), int64(len(input)), a, []int{0}, ParquetOptions{}, cache)
		if failure != nil {
			t.Fatal(failure)
		}
		return r
	}
	r := open(data)
	columns, groups := &r.columns[0], &r.groups[0]
	r.Close()
	segment := a.AllocSegTemp(4096)
	clear(segment.AsBytes())
	segment.Dec()
	r = open(data)
	if columns != &r.columns[0] || groups != &r.groups[0] {
		t.Fatal("identical footer did not reuse immutable metadata")
	}
	r.Close()
	footerStart := len(data) - 8 - int(binary.LittleEndian.Uint32(data[len(data)-8:]))
	changed := bytes.Clone(data)
	footer := bytes.ReplaceAll(changed[footerStart:len(data)-8], []byte("id"), []byte("ix"))
	if len(footer) != len(data)-8-footerStart || bytes.Equal(footer, data[footerStart:len(data)-8]) {
		t.Fatal("fixture footer replacement failed")
	}
	copy(changed[footerStart:], footer)
	r = open(changed)
	name, _, _ := r.Column(0)
	if name != "ix" || &r.columns[0] == columns {
		t.Fatal("same-size changed footer reused stale schema")
	}
	r.Close()
	r = open(data)
	columns = &r.columns[0]
	r.Close()
	changed = bytes.Clone(data)
	binary.LittleEndian.PutUint32(changed[27:31], 17)
	r = open(changed)
	if &r.columns[0] != columns {
		t.Fatal("payload-only change invalidated footer metadata")
	}
	batch, failure := r.Next(context.Background())
	if failure != nil {
		t.Fatal(failure)
	}
	if batch.VectorAt(0).I32s()[0] != 17 {
		t.Fatal("cached metadata retained stale payload")
	}
	batch.Release()
	r.Close()
	changed = append(bytes.Clone(data[:footerStart]), 0)
	changed = append(changed, data[footerStart:]...)
	r = open(changed)
	if &r.columns[0] == columns {
		t.Fatal("different file size reused metadata without validating chunk bounds")
	}
	r.Close()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("reader retained allocator bytes: %+v", usage)
	}
}

func TestParquetMetadataCacheWarmValidation(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBFlatFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	cache := MakeParquetMetadataCache()
	r, failure := MakeParquetReaderProjectedCached(bytes.NewReader(data), int64(len(data)), a, []int{0}, ParquetOptions{}, cache)
	if failure != nil {
		t.Fatal(failure)
	}
	r.Close()
	footerSize := int(binary.LittleEndian.Uint32(data[len(data)-8:]))
	for _, selected := range [][]int{{-1}, {2}, {0, 0}} {
		if _, failure := MakeParquetReaderProjectedCached(bytes.NewReader(data), int64(len(data)), a, selected, ParquetOptions{}, cache); failure == nil || failure.Code() != ParquetInvalid {
			t.Fatalf("warm invalid projection %v: %v", selected, failure)
		}
	}
	if _, failure := MakeParquetReaderProjectedCached(bytes.NewReader(data), int64(len(data)), a, []int{0}, ParquetOptions{MaxFooterBytes: footerSize - 1}, cache); failure == nil || failure.Code() != ParquetResourceExhausted {
		t.Fatalf("warm footer limit: %v", failure)
	}
	scope := mem.MakeAllocationScope(a, int64(footerSize-1))
	if _, failure := MakeParquetReaderProjectedCached(bytes.NewReader(data), int64(len(data)), mem.MakeAllocatorWithScope(a, scope), []int{0}, ParquetOptions{}, cache); failure == nil || failure.Code() != ParquetResourceExhausted || scope.Live() != 0 {
		t.Fatalf("warm footer allocation admission: %v, live=%d", failure, scope.Live())
	}
	for _, offset := range []int{0, len(data) - 1, len(data) - 8 - footerSize + 1} {
		changed := bytes.Clone(data)
		changed[offset] ^= 0xff
		if _, failure := MakeParquetReaderProjectedCached(bytes.NewReader(changed), int64(len(changed)), a, []int{0}, ParquetOptions{}, cache); failure == nil {
			t.Fatalf("warm malformed input at %d was accepted", offset)
		}
	}
	r, failure = MakeParquetReaderProjectedCached(bytes.NewReader(data), int64(len(data)), a, []int{0}, ParquetOptions{}, cache)
	if failure != nil {
		t.Fatalf("failed open damaged valid cache: %v", failure)
	}
	r.Close()
}

func TestParquetMetadataCacheConcurrent(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBFlatFixture)
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	cache := MakeParquetMetadataCache()
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := range 16 {
				r, failure := MakeParquetReaderProjectedCached(bytes.NewReader(data), int64(len(data)), a, []int{worker % 2}, ParquetOptions{BatchSize: 1 + iteration%3}, cache)
				if failure != nil {
					failures <- failure
					return
				}
				rows := 0
				for {
					batch, failure := r.Next(context.Background())
					if failure != nil {
						failures <- failure
						r.Close()
						return
					}
					if batch == nil {
						break
					}
					rows += batch.Len()
					batch.Release()
				}
				r.Close()
				if rows != 3 {
					failures <- fmt.Errorf("decoded %d rows", rows)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}

func BenchmarkParquetMetadataCache(b *testing.B) {
	data, err := base64.StdEncoding.DecodeString(duckDBFlatFixture)
	if path := os.Getenv("PEG_BENCH_PARQUET"); path != "" {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		b.Fatal(err)
	}
	for _, cached := range []bool{false, true} {
		b.Run(fmt.Sprint(cached), func(b *testing.B) {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			var cache *ParquetMetadataCache
			if cached {
				cache = MakeParquetMetadataCache()
			}
			input := bytes.NewReader(data)
			selected := []int{0}
			r, failure := MakeParquetReaderProjectedCached(input, int64(len(data)), a, selected, ParquetOptions{}, cache)
			if failure != nil {
				b.Fatal(failure)
			}
			r.Close()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				r, failure := MakeParquetReaderProjectedCached(input, int64(len(data)), a, selected, ParquetOptions{}, cache)
				if failure != nil {
					b.Fatal(failure)
				}
				r.Close()
			}
		})
	}
}
