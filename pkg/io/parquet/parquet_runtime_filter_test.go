package parquet

import (
	"bytes"
	"context"
	"math"
	"sync"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestRuntimePruningConservativeBounds(t *testing.T) {
	for _, allowNull := range []bool{false, true} {
		filter := MakeRuntimePruningFilter(0, allowNull)
		if filter.cannotMatch(parquetChunk{allNull: true}) {
			t.Fatal("unpublished filter rejected a null group")
		}
		filter.UpdateLowerBound(-7)
		filter.UpdateUpperBound(13)
		filter.UpdateLowerBound(-9)
		filter.UpdateUpperBound(15)
		for _, tc := range []struct {
			chunk parquetChunk
			skip  bool
		}{
			{parquetChunk{hasMinMax: true, noNull: true, min: -10, max: -8}, true},
			{parquetChunk{hasMinMax: true, noNull: true, min: -8, max: -7}, false},
			{parquetChunk{hasMinMax: true, noNull: true, min: 13, max: 14}, false},
			{parquetChunk{hasMinMax: true, noNull: true, min: 14, max: 17}, true},
			{parquetChunk{hasMinMax: true, min: 14, max: 17}, !allowNull},
			{parquetChunk{allNull: true}, !allowNull},
			{parquetChunk{noNull: true}, false},
		} {
			if got := filter.cannotMatch(tc.chunk); got != tc.skip {
				t.Fatalf("nulls=%t chunk=%+v skip=%t want=%t", allowNull, tc.chunk, got, tc.skip)
			}
		}
		filter.UpdateLowerBound(math.MaxInt64)
		filter.UpdateUpperBound(math.MinInt64)
		if got := filter.cannotMatch(parquetChunk{}); got == allowNull {
			t.Fatalf("impossible interval nulls=%t skip=%t", allowNull, got)
		}
	}
	r := &ParquetReader{columns: []parquetColumn{{typ: dtype.Float64T()}, {typ: dtype.Int64T()}}}
	if r.SetRuntimePruningFilter(MakeRuntimePruningFilter(0, false)) || r.SetRuntimePruningFilter(MakeRuntimePruningFilter(2, false)) || !r.SetRuntimePruningFilter(MakeRuntimePruningFilter(1, false)) {
		t.Fatal("runtime filter installation type/index validation")
	}
	r.row = 1
	if r.SetRuntimePruningFilter(nil) {
		t.Fatal("replaced a live reader's filter")
	}
}

func TestRuntimePruningConcurrentPublicationAndReaders(t *testing.T) {
	data := parquetPredicateFixture(t)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	guard := &guardedParquetRange{ReaderAt: bytes.NewReader(data)}
	r, failure := MakeParquetReaderProjected(guard, int64(len(data)), a, []int{0, 3}, ParquetOptions{BatchSize: 317})
	if failure != nil {
		t.Fatal(failure)
	}
	defer r.Close()
	filter := MakeRuntimePruningFilter(0, false)
	if !r.SetRuntimePruningFilter(filter) || len(r.groups) != 3 {
		t.Fatal("fixture or installation")
	}
	groupReader := r.MakeRowGroupReader(2, a)
	if groupReader == nil {
		t.Fatal("row-group reader")
	}
	defer groupReader.Close()
	rows := int64(0)
	for rows < r.groups[0].rows {
		batch, failure := r.Next(context.Background())
		if failure != nil || batch == nil {
			t.Fatalf("first group: %v", failure)
		}
		rows += int64(batch.Len())
		batch.Release()
	}
	guard.start, guard.end = math.MaxInt64, 0
	for _, group := range r.groups[1:] {
		for _, chunk := range group.chunks {
			guard.start, guard.end = min(guard.start, chunk.start), max(guard.end, chunk.end)
		}
	}
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 256 {
				filter.UpdateUpperBound(r.groups[0].chunks[0].max + int64(i+j))
				filter.cannotMatch(r.groups[2].chunks[0])
			}
		}()
	}
	wg.Wait()
	for _, reader := range []*ParquetReader{r, groupReader} {
		batch, failure := reader.Next(context.Background())
		if failure != nil || batch != nil {
			if batch != nil {
				batch.Release()
			}
			t.Fatalf("runtime bounds did not skip unread groups: %v", failure)
		}
	}
	if guard.blocked != 0 || filter.PrunedRowGroups() != 3 {
		t.Fatalf("blocked reads=%d skipped groups=%d", guard.blocked, filter.PrunedRowGroups())
	}
	r.Close()
	groupReader.Close()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatal("runtime scan retained allocator bytes")
	}
}
