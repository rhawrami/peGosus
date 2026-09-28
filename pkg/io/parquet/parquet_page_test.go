package parquet

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestParquetHybridRuns(t *testing.T) {
	var values [3]byte
	if err := decodeParquetRLE([]byte{6, 1}, 1, values[:]); err != nil || values != [3]byte{1, 1, 1} {
		t.Fatalf("RLE: %v %v", values, err)
	}
	if err := decodeParquetRLE([]byte{3, 0x55}, 1, values[:]); err != nil || values != [3]byte{1, 0, 1} {
		t.Fatalf("packed: %v %v", values, err)
	}
	var indices [12]byte
	if err := decodeParquetIndices([]byte{3, 0x21, 0x00}, 2, indices[:]); err != nil {
		t.Fatal(err)
	}
	for i, want := range []uint32{1, 0, 2} {
		if got := binary.LittleEndian.Uint32(indices[i*4:]); got != want {
			t.Fatalf("index %d = %d", i, got)
		}
	}
	for _, invalid := range [][]byte{{0}, {2}, {3}} {
		if err := decodeParquetRLE(invalid, 1, values[:]); err == nil {
			t.Fatalf("accepted %v", invalid)
		}
	}
}

func TestParquetV2LevelsAndDictionary(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	dictionary := a.AllocSegTemp(8)
	binary.LittleEndian.PutUint32(dictionary.AsBytes(), 12)
	binary.LittleEndian.PutUint32(dictionary.AsBytes()[4:], 29)
	c := parquetCursor{a: a, column: parquetColumn{typ: dtype.Int32T(), optional: true}, chunk: parquetChunk{values: 4}, options: ParquetOptions{MaxDecodedPageBytes: 4096}, dictionary: dictionary, dictCount: 2, dictRaw: dictionary.AsBytes()}
	defer c.close()
	info := compactValue{kind: 12, fields: map[int16]compactValue{
		1: {kind: 5, integer: 3}, 2: {kind: 5, integer: 1}, 3: {kind: 5, integer: 3},
		4: {kind: 5, integer: 8}, 5: {kind: 5, integer: 2}, 6: {kind: 5, integer: 0},
	}}
	header := compactValue{kind: 12, fields: map[int16]compactValue{8: info}}
	// Definition levels 1,0,1; dictionary indices 1,0.
	if err := c.startPage(3, header, []byte{3, 5}, []byte{1, 3, 1}); err != nil {
		t.Fatal(err)
	}
	for i, want := range []int32{29, -1, 12} {
		value, valid, err := c.next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if want == -1 {
			if valid {
				t.Fatal("null row was valid")
			}
			continue
		}
		if !valid || int32(binary.LittleEndian.Uint32(value)) != want {
			t.Fatalf("row %d: %v valid %v", i, value, valid)
		}
	}
	c.clearPage()
	plain := compactValue{kind: 12, fields: map[int16]compactValue{
		1: {kind: 5, integer: 1}, 2: {kind: 5, integer: 0}, 3: {kind: 5, integer: 3}, 4: {kind: 5, integer: 3},
	}}
	page := compactValue{kind: 12, fields: map[int16]compactValue{5: plain}}
	if err := c.startPage(0, page, []byte{2, 0, 0, 0, 2, 1, 77, 0, 0, 0}, nil); err != nil {
		t.Fatal(err)
	}
	value, valid, failure := c.next(context.Background())
	if failure != nil || !valid || binary.LittleEndian.Uint32(value) != 77 {
		t.Fatalf("PLAIN fallback after dictionary: %v, %v, %v", value, valid, failure)
	}
}
