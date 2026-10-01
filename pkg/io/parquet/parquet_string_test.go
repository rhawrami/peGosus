package parquet

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makeParquetStringTestCursor(a *mem.Allocator, values [][]byte, valid []bool, dictionary bool) parquetCursor {
	c := parquetCursor{a: a, column: parquetColumn{typ: dtype.StringT(), optional: valid != nil}, group: 12, columnIndex: 3, pages: 7, pageRows: len(values), chunk: parquetChunk{values: int64(len(values))}}
	if valid != nil {
		c.levels = a.AllocSegTemp(len(values))
		clear(c.levels.AsBytes())
		for i := range values {
			if valid[i] {
				c.levels.AsBytes()[i] = 1
			}
		}
	}
	var body []byte
	for i, value := range values {
		if valid != nil && !valid[i] {
			continue
		}
		body = binary.LittleEndian.AppendUint32(body, uint32(len(value)))
		body = append(body, value...)
		c.nonNull++
	}
	c.data = a.AllocSegTemp(len(body))
	copy(c.data.AsBytes(), body)
	c.values = c.data.AsBytes()
	if dictionary {
		c.encoding = 8
		c.dictionary, c.data = c.data, nil
		c.dictRaw = c.dictionary.AsBytes()
		c.dictCount = c.nonNull
		c.dictOffsets = a.AllocSegTemp(c.dictCount * 8)
		c.indices = a.AllocSegTemp(c.nonNull * 4)
		position := 0
		for i := range c.dictCount {
			length := int(binary.LittleEndian.Uint32(c.dictRaw[position:]))
			start, stop := position+4, position+4+length
			c.dictOffsets.AsI64T()[i] = int64(uint64(start) | uint64(stop)<<32)
			binary.LittleEndian.PutUint32(c.indices.AsBytes()[i*4:], uint32(i))
			position = stop
		}
	}
	return c
}

func TestParquetStringMaterializationDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(93))
	for _, n := range []int{0, 1, 7, 8, 9, 63, 64, 65, 4096} {
		values := make([][]byte, n)
		valid := make([]bool, n)
		for i := range values {
			length := []int{0, 1, 4, 11, 12, 13, 31, 129, 4097}[rng.Intn(9)]
			values[i] = bytes.Repeat([]byte{byte('a' + rng.Intn(26))}, length)
			if i%11 == 0 {
				values[i] = []byte(strings.Repeat("é", length/2))
			}
			valid[i] = rng.Intn(4) != 0
		}
		for _, nullable := range []bool{false, true} {
			for _, dictionary := range []bool{false, true} {
				a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
				scope := mem.MakeAllocationScope(a, 32<<20)
				view := mem.MakeAllocatorWithScope(a, scope)
				var validity []bool
				if nullable {
					validity = valid
				}
				c := makeParquetStringTestCursor(view, values, validity, dictionary)
				v, failure := makeParquetStringVector(context.Background(), &c, n)
				if failure != nil {
					t.Fatalf("n=%d nullable=%v dictionary=%v: %v", n, nullable, dictionary, failure)
				}
				retained := v.Retain()
				v.Release()
				if c.dictionary != nil {
					clear(c.dictionary.AsBytes())
				} else {
					clear(c.data.AsBytes())
				}
				c.close()
				reference := store.MakeStringVector(view, values, validity)
				payload := 0
				for i := range n {
					if nullable && retained.Validity().IsSet(i) != valid[i] {
						t.Fatalf("row %d validity mismatch", i)
					}
					if retained.Strings()[i].View() != reference.Strings()[i].View() {
						t.Fatalf("n=%d dictionary=%v row %d payload mismatch", n, dictionary, i)
					}
					if (!nullable || valid[i]) && len(values[i]) > dtype.StringInlineSize {
						payload += len(values[i])
					}
				}
				if payload == 0 && retained.Backing() != nil || payload != 0 && (retained.Backing() == nil || retained.Backing().Len() != payload) {
					t.Fatal("long backing is not exactly sized")
				}
				reference.Release()
				retained.Release()
				if scope.Live() != 0 {
					t.Fatalf("leaked %d requested bytes", scope.Live())
				}
			}
		}
	}
}

func TestParquetStringMaterializationInlineBudget(t *testing.T) {
	for _, allNull := range []bool{false, true} {
		values, valid := make([][]byte, 65), make([]bool, 65)
		for i := range values {
			values[i], valid[i] = []byte("abcdefghijkl"), !allNull
		}
		a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
		c := makeParquetStringTestCursor(a, values, valid, false)
		scope := mem.MakeAllocationScope(a, int64(len(values)*dtype.StringSize+(len(values)+7)/8))
		c.a = mem.MakeAllocatorWithScope(a, scope)
		v, failure := makeParquetStringVector(context.Background(), &c, len(values))
		c.close()
		if failure != nil {
			t.Fatal(failure)
		}
		if v.Backing() != nil || scope.Peak() != scope.Limit() {
			t.Fatal("inline and null materialization must need only descriptors and validity")
		}
		if allNull && v.NiN() != len(values) {
			t.Fatal("all-null rows lost")
		}
		v.Release()
		if scope.Live() != 0 {
			t.Fatal("inline vector allocation leaked")
		}
	}
}

func TestParquetStringMaterializationFailures(t *testing.T) {
	values, valid := make([][]byte, 65), make([]bool, 65)
	for i := range values {
		values[i], valid[i] = bytes.Repeat([]byte("a"), 13+i*5), true
	}
	for _, limit := range []int64{0, 65 * 16, 65*16 + 9, 1800, 2600, 5000, 15000, 24000, 1 << 20} {
		a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
		c := makeParquetStringTestCursor(a, values, valid, false)
		scope := mem.MakeAllocationScope(a, limit)
		c.a = mem.MakeAllocatorWithScope(a, scope)
		v, failure := makeParquetStringVector(context.Background(), &c, len(values))
		c.close()
		if failure != nil {
			if failure.Code() != ParquetResourceExhausted || failure.RowGroup() != 12 || failure.Column() != 3 || failure.Page() != 7 || v.Kind() != store.VectorInvalid {
				t.Fatalf("limit %d: wrong resource failure: %v", limit, failure)
			}
		} else {
			if limit < 24000 {
				t.Fatalf("unexpected success with limit %d", limit)
			}
			v.Release()
		}
		if scope.Live() != 0 {
			t.Fatalf("limit %d: leaked %d bytes", limit, scope.Live())
		}
	}
	for _, invalid := range [][]byte{{'o', 'k', 0xff}, []byte("truncated-value")} {
		a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
		c := makeParquetStringTestCursor(a, [][]byte{[]byte("valid"), invalid}, nil, false)
		if string(invalid) == "truncated-value" {
			c.values = c.values[:len(c.values)-1]
		}
		scope := mem.MakeAllocationScope(a, 1<<20)
		c.a = mem.MakeAllocatorWithScope(a, scope)
		v, failure := makeParquetStringVector(context.Background(), &c, 2)
		c.close()
		if failure == nil || failure.Code() != ParquetInvalid || v.Kind() != store.VectorInvalid || scope.Live() != 0 {
			t.Fatalf("invalid value accepted or leaked: %v", failure)
		}
	}
}

func TestParquetStringMaterializationRetainedFileBatches(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(duckDBGroupsFixture)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(t.TempDir(), "strings-*.parquet")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		t.Fatal(err)
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 16}, []int{1 << 16})
	scope := mem.MakeAllocationScope(a, 1<<20)
	view := mem.MakeAllocatorWithScope(a, scope)
	r, failure := MakeParquetReader(f, int64(len(data)), view, ParquetOptions{BatchSize: 31})
	if failure != nil {
		t.Fatal(failure)
	}
	var retained []*store.Batch
	for {
		batch, failure := r.Next(context.Background())
		if failure != nil {
			t.Fatal(failure)
		}
		if batch == nil {
			break
		}
		retained = append(retained, batch.Retain())
		batch.Release()
	}
	r.Close()
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	row := 0
	for _, batch := range retained {
		v := batch.VectorAt(0)
		for i := range batch.Len() {
			if v.Validity().IsSet(i) != (row%3 != 1) || row%3 != 1 && v.Strings()[i].View() != "abcdefabcdefabcdef" {
				t.Fatalf("retained row %d changed after later batches, pages, row groups, or file close", row)
			}
			row++
		}
		batch.Release()
	}
	if row != 2050 || scope.Live() != 0 {
		t.Fatalf("retained rows %d live bytes %d", row, scope.Live())
	}
}

func TestParquetStringMaterializationWithinBatchPages(t *testing.T) {
	var source []byte
	var values [][]byte
	for page, count := range []int{1, 3, 16, 65, 52} {
		var body []byte
		for row := range count {
			length := []int{0, 11, 12, 13, 129, 4097}[(page+row)%6]
			value := bytes.Repeat([]byte{byte('a' + (page+row)%26)}, length)
			values = append(values, value)
			body = binary.LittleEndian.AppendUint32(body, uint32(length))
			body = append(body, value...)
		}
		// Compact-Thrift PageHeader with a required PLAIN DataPageHeader.
		source = append(source, 0x15, 0, 0x15)
		source = binary.AppendVarint(source, int64(len(body)))
		source = append(source, 0x15)
		source = binary.AppendVarint(source, int64(len(body)))
		source = append(source, 0x2c, 0x15)
		source = binary.AppendVarint(source, int64(count))
		source = append(source, 0x15, 0, 0x15, 6, 0x15, 6, 0, 0)
		source = append(source, body...)
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, 4<<20)
	view := mem.MakeAllocatorWithScope(a, scope)
	c := parquetCursor{a: view, column: parquetColumn{typ: dtype.StringT()}, chunk: parquetChunk{end: int64(len(source)), values: int64(len(values))}, input: bytes.NewReader(source), options: ParquetOptions{MaxPageBytes: parquetPageLimit, MaxDecodedPageBytes: parquetDecodedLimit}}
	v, failure := makeParquetStringVector(context.Background(), &c, len(values))
	if failure != nil {
		c.close()
		t.Fatal(failure)
	}
	if c.pages != 5 {
		t.Fatalf("one vector consumed %d pages", c.pages)
	}
	retained := v.Retain()
	v.Release()
	c.close()
	clear(source)
	reuse := view.AllocSegTemp(len(source))
	if reuse == nil {
		t.Fatal("page scratch reuse allocation failed")
	}
	reuse.MemSetU8('z')
	for i, expected := range values {
		if retained.Strings()[i].View() != string(expected) {
			t.Fatalf("row %d borrowed a released or reused data page", i)
		}
	}
	reuse.Dec()
	retained.Release()
	if scope.Live() != 0 {
		t.Fatalf("multi-page output leaked %d bytes", scope.Live())
	}
}
