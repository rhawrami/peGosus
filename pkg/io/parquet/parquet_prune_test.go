package parquet

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestParquetRowGroupPruningBounds(t *testing.T) {
	r := &ParquetReader{columns: []parquetColumn{{typ: dtype.Int32T()}}}
	group := parquetGroup{rows: 4, chunks: []parquetChunk{{hasMinMax: true, min: -5, max: 8}}}
	cases := []struct {
		predicate PruningPredicate
		skip      bool
	}{
		{PruningPredicate{Column: 0, Op: PruneEqual, Literal: -6}, true},
		{PruningPredicate{Column: 0, Op: PruneEqual, Literal: 8}, false},
		{PruningPredicate{Column: 0, Op: PruneLess, Literal: -5}, true},
		{PruningPredicate{Column: 0, Op: PruneLessEqual, Literal: -5}, false},
		{PruningPredicate{Column: 0, Op: PruneGreater, Literal: 8}, true},
		{PruningPredicate{Column: 0, Op: PruneGreaterEqual, Literal: 8}, false},
		{PruningPredicate{Column: 0, Op: PruneNotEqual, Literal: 3}, false},
	}
	for _, tc := range cases {
		if !r.SetPruningPredicates([]PruningPredicate{tc.predicate}) {
			t.Fatal("rejected predicate")
		}
		if got := r.groupCannotMatch(group); got != tc.skip {
			t.Fatalf("predicate %+v skip=%t, want %t", tc.predicate, got, tc.skip)
		}
	}
	group.chunks[0].hasMinMax = false
	if !r.SetPruningPredicates([]PruningPredicate{{Column: 0, Op: PruneGreater, Literal: 8}}) || r.groupCannotMatch(group) {
		t.Fatal("pruned a group without usable statistics")
	}
	group.chunks[0].allNull = true
	if !r.groupCannotMatch(group) {
		t.Fatal("did not prune an all-null group")
	}
	group.chunks[0] = parquetChunk{hasMinMax: true, min: 17, max: 17}
	r.SetPruningPredicates([]PruningPredicate{{Column: 0, Op: PruneNotEqual, Literal: 17}})
	if !r.groupCannotMatch(group) {
		t.Fatal("a singleton equal to the excluded value cannot match")
	}
}

type guardedParquetRange struct {
	io.ReaderAt
	start, end int64
	blocked    int
}

func (r *guardedParquetRange) ReadAt(dst []byte, offset int64) (int, error) {
	if len(dst) != 0 && offset < r.end && offset+int64(len(dst)) > r.start {
		r.blocked++
		return 0, errors.New("read pruned row group")
	}
	return r.ReaderAt.ReadAt(dst, offset)
}

func TestExternalParquetRowGroupPruningSkipsIO(t *testing.T) {
	path := filepath.Join("../../../testdata/paired", "scan_snappy.parquet")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("prepare ignored data with scripts/prepare_io_testdata.py")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	guard := &guardedParquetRange{ReaderAt: f}
	r, failure := MakeParquetReaderProjected(guard, info.Size(), a, []int{0}, ParquetOptions{BatchSize: 257})
	if failure != nil {
		t.Fatal(failure)
	}
	defer r.Close()
	if !r.groups[0].chunks[0].hasMinMax || r.groups[0].chunks[0].min != 0 || r.groups[0].chunks[0].max != 2047 {
		t.Fatal("incorrect independent writer statistics")
	}
	guard.start, guard.end = r.groups[0].chunks[0].start, r.groups[0].chunks[0].end
	if !r.SetPruningPredicates([]PruningPredicate{{Column: 0, Op: PruneGreaterEqual, Literal: 2048}}) {
		t.Fatal("pruning setup failed")
	}
	rows := 0
	for {
		batch, failure := r.Next(context.Background())
		if failure != nil {
			t.Fatal(failure)
		}
		if batch == nil {
			break
		}
		for _, id := range batch.VectorAt(0).I32s() {
			if id < 2048 {
				t.Fatalf("unpruned row %d", id)
			}
			rows++
		}
		batch.Release()
	}
	if guard.blocked != 0 || rows != 16387-2048 {
		t.Fatalf("pruning made %d blocked reads and returned %d rows", guard.blocked, rows)
	}
}
