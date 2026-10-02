package parquet

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"testing"

	"github.com/golang/snappy"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParquetDictionaryToPlainPageBoundary(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	c := makeParquetStringTestCursor(a, [][]byte{[]byte("first"), []byte("retained long dictionary value")}, nil, true)
	c.chunk.values = 4
	c.options.MaxDecodedPageBytes = 4096
	r := &ParquetReader{cursors: []parquetCursor{c}, dictionaryColumns: []bool{true}}
	n, failure := r.dictionaryBatchLength(context.Background(), 4)
	if failure != nil || n != 2 {
		t.Fatal("batch crossed encoded page boundary")
	}
	c = r.cursors[0]
	encoded, failure := c.materializeDictionaryStrings(context.Background(), n)
	if failure != nil {
		t.Fatal(failure)
	}
	c.clearPage()
	var data []byte
	for _, text := range []string{"second", "owned long PLAIN value"} {
		data = binary.LittleEndian.AppendUint32(data, uint32(len(text)))
		data = append(data, text...)
	}
	header := compactValue{kind: 12, fields: map[int16]compactValue{5: {kind: 12, fields: map[int16]compactValue{1: {kind: 5, integer: 2}, 2: {kind: 5, integer: 0}, 4: {kind: 5, integer: 3}}}}}
	if failure := c.startPage(0, header, data, nil); failure != nil {
		t.Fatal(failure)
	}
	plain, failure := makeParquetStringVector(context.Background(), &c, 2)
	if failure != nil {
		t.Fatal(failure)
	}
	c.close()
	for i, value := range []string{"first", "retained long dictionary value"} {
		if encoded.StringAt(i).View() != value {
			t.Fatal("PLAIN transition changed old dictionary")
		}
	}
	for i, value := range []string{"second", "owned long PLAIN value"} {
		if plain.StringAt(i).View() != value {
			t.Fatal("dictionary transition changed PLAIN")
		}
	}
	encoded.Release()
	plain.Release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("transition leaked %+v", usage)
	}
}

func TestParquetDictionaryBatchDifferentialAndOwnership(t *testing.T) {
	data := parquetPredicateFixture(t)
	for _, size := range []int{1, 9, 317, 4096} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
		scope := mem.MakeAllocationScope(a, 8<<20)
		view := mem.MakeAllocatorWithScope(a, scope)
		for _, predicates := range [][]ScanPredicate{nil, {{Column: 3, Op: PruneNotEqual, Text: "north"}}} {
			r, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), view, []int{0, 3}, ParquetOptions{BatchSize: size})
			if failure != nil {
				t.Fatal(failure)
			}
			if r.SetDictionaryColumns([]int{0}) || r.SetDictionaryColumns([]int{-1}) || !r.SetDictionaryColumns([]int{3}) || !r.SetScanPredicates(predicates) {
				t.Fatal("invalid dictionary installation")
			}
			var outputs []*store.Batch
			encoded := false
			for {
				batch, failure := r.Next(context.Background())
				if failure != nil {
					t.Fatal(failure)
				}
				if batch == nil {
					break
				}
				encoded = encoded || batch.VectorAt(1).Kind() == store.VectorDictionaryString
				outputs = append(outputs, batch)
			}
			r.Close()
			if !encoded {
				t.Fatal("dictionary pages were flattened")
			}
			rows := 0
			for _, batch := range outputs {
				key := batch.VectorAt(1)
				for row := range batch.Len() {
					seq := int(batch.VectorAt(0).I32s()[row])
					want := parquetPredicateText(seq)
					valid := seq%23 != 0
					if key.Validity().IsSet(row) != valid || valid && key.StringAt(row).View() != want {
						t.Fatalf("row %d lost dictionary payload", seq)
					}
					active := batch.Selection() == nil
					if bitmap, ok := batch.Selection().AsBitMap(); ok {
						active = bitmap.IsSet(row)
					}
					if len(predicates) != 0 && active != (valid && want != "north") {
						t.Fatalf("row %d encoded predicate mismatch", seq)
					}
					rows++
				}
				batch.Release()
			}
			if len(predicates) == 0 && rows != 4097 {
				t.Fatalf("wrong row domain %d", rows)
			}
			if scope.Live() != 0 {
				t.Fatalf("dictionary reader leaked %d bytes", scope.Live())
			}
		}
	}
}

func TestParquetDictionaryNullsCorruptionCancellationAndBudget(t *testing.T) {
	for _, n := range []int{0, 1, 9, 65} {
		a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
		values, valid := make([][]byte, n), make([]bool, n)
		for row := range n {
			values[row], valid[row] = []byte("a long dictionary string ø"), row%3 != 0
		}
		c := makeParquetStringTestCursor(a, values, valid, true)
		v, failure := c.materializeDictionaryStrings(context.Background(), n)
		if failure != nil {
			t.Fatal(failure)
		}
		retained := v.Retain()
		v.Release()
		c.close()
		for row := range n {
			if retained.Validity().IsSet(row) != valid[row] || valid[row] && retained.StringAt(row).View() != string(values[row]) {
				t.Fatal("retained dictionary changed")
			}
		}
		retained.Release()
		c = makeParquetStringTestCursor(a, values, valid, true)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		v, failure = c.materializeDictionaryStrings(ctx, n)
		if n != 0 && (failure == nil || failure.Code() != ParquetCancelled) {
			t.Fatal("dictionary ignored cancellation")
		}
		v.Release()
		c.close()
		if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
			t.Fatalf("reader leaked %+v", usage)
		}
	}
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	for _, corrupt := range []bool{false, true} {
		c := makeParquetStringTestCursor(a, [][]byte{[]byte("value")}, nil, true)
		if corrupt {
			binary.LittleEndian.PutUint32(c.indices.AsBytes(), math.MaxUint32)
		} else {
			c.a = mem.MakeAllocatorWithScope(a, mem.MakeAllocationScope(a, 0))
		}
		v, failure := c.materializeDictionaryStrings(context.Background(), 1)
		if v.Kind() != store.VectorInvalid || failure == nil || corrupt && failure.Code() != ParquetInvalid || !corrupt && failure.Code() != ParquetResourceExhausted {
			t.Fatalf("invalid dictionary failure: %v", failure)
		}
		c.close()
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("failure leaked %+v", usage)
	}
}

func TestParquetDecodedPageReuseKeepsReservation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	scope := mem.MakeAllocationScope(a, 4096)
	c := parquetCursor{a: mem.MakeAllocatorWithScope(a, scope), chunk: parquetChunk{codec: 1}}
	first, failure := c.decompress(snappy.Encode(nil, bytes.Repeat([]byte{3}, 2048)), 2048)
	if failure != nil {
		t.Fatal(failure)
	}
	c.decoded = first
	c.clearPage()
	second, failure := c.decompress(snappy.Encode(nil, bytes.Repeat([]byte{7}, 1024)), 1024)
	if failure != nil || second != first || scope.Live() != 2048 || !bytes.Equal(second.AsBytes()[:1024], bytes.Repeat([]byte{7}, 1024)) {
		t.Fatalf("decoded reuse lost reservation or contents: %v live=%d", failure, scope.Live())
	}
	c.decoded = second
	c.close()
	if scope.Live() != 0 {
		t.Fatalf("cursor retained %d bytes", scope.Live())
	}
}
