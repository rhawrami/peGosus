package parquet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func makeParquetFixedTestCursor(a *mem.Allocator, typ dtype.Type, values []uint64, valid []bool, dictionary bool) parquetCursor {
	c := parquetCursor{a: a, column: parquetColumn{typ: typ, optional: valid != nil}, pageRows: len(values), chunk: parquetChunk{values: int64(len(values))}}
	if valid != nil {
		c.levels = a.AllocSegTemp(len(values))
		clear(c.levels.AsBytes())
	}
	size := typ.SlotBits() / 8
	var body []byte
	for i, value := range values {
		if valid != nil {
			if !valid[i] {
				continue
			}
			c.levels.AsBytes()[i] = 1
		}
		if typ.ID() == dtype.BOOLT {
			if c.nonNull&7 == 0 {
				body = append(body, 0)
			}
			body[c.nonNull/8] |= byte(value&1) << (c.nonNull & 7)
		} else if size == 4 {
			body = binary.LittleEndian.AppendUint32(body, uint32(value))
		} else {
			body = binary.LittleEndian.AppendUint64(body, value)
		}
		c.nonNull++
	}
	c.data = a.AllocSegTemp(len(body))
	copy(c.data.AsBytes(), body)
	c.values = c.data.AsBytes()
	if dictionary {
		c.encoding, c.dictCount = 8, c.nonNull
		c.dictionary, c.data = c.data, nil
		c.dictRaw, c.values = c.values, nil
		c.indices = a.AllocSegTemp(c.nonNull * 4)
		for i := range c.nonNull {
			binary.LittleEndian.PutUint32(c.indices.AsBytes()[i*4:], uint32(i))
		}
	}
	return c
}

func TestParquetFixedMaterializationDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(97, 43))
	for _, typ := range []dtype.Type{dtype.BoolT(), dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T(), dtype.DateT(), dtype.TimestampTZT()} {
		for _, shape := range []string{"required", "valid", "nullable", "null"} {
			for _, dictionary := range []bool{false, true} {
				t.Run(fmt.Sprintf("%v/%s/dictionary=%v", typ.ID(), shape, dictionary), func(t *testing.T) {
					for _, n := range []int{0, 1, 7, 8, 9, 65, 529, 4097} {
						values := make([]uint64, n)
						var valid []bool
						if shape != "required" {
							valid = make([]bool, n)
						}
						for i := range values {
							values[i] = rng.Uint64()
							if i < 6 {
								values[i] = []uint64{0, 1 << 63, 0x7ff0000000000000, 0xfff0000000000000, 0x7ff8000000000012, 0x7fc00012}[i]
							}
							if valid != nil {
								valid[i] = shape == "valid" || shape == "nullable" && rng.IntN(4) != 0
							}
						}
						a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
						c := makeParquetFixedTestCursor(a, typ, values, valid, dictionary)
						ref := makeParquetFixedTestCursor(a, typ, values, valid, dictionary)
						if dictionary {
							for i := range c.nonNull {
								index := uint32(rng.IntN(c.dictCount))
								binary.LittleEndian.PutUint32(c.indices.AsBytes()[i*4:], index)
								binary.LittleEndian.PutUint32(ref.indices.AsBytes()[i*4:], index)
							}
						}
						for offset := 0; offset < n; {
							length := min(n-offset, 1+rng.IntN(103))
							v := store.MakeVector(a, length, typ, valid != nil)
							if failure := c.materializeFixed(context.Background(), &v); failure != nil {
								t.Fatal(failure)
							}
							for i := range length {
								value, yes, failure := ref.next(context.Background())
								if failure != nil || yes != (v.Validity() == nil || v.Validity().IsSet(i)) {
									t.Fatalf("row %d: scalar=%v/%v, bulk=%v", offset+i, yes, failure, v.Validity())
								}
								if !yes {
									continue
								}
								if typ.ID() == dtype.BOOLT {
									if v.Bools()[i/8]>>(i&7)&1 != value[0] {
										t.Fatalf("boolean row %d", offset+i)
									}
								} else {
									size := typ.SlotBits() / 8
									var want [8]byte
									if size == 4 {
										binary.NativeEndian.PutUint32(want[:], binary.LittleEndian.Uint32(value))
									} else {
										binary.NativeEndian.PutUint64(want[:], binary.LittleEndian.Uint64(value))
									}
									if !bytes.Equal(v.Data().AsBytes()[i*size:(i+1)*size], want[:size]) {
										t.Fatalf("fixed row %d", offset+i)
									}
								}
							}
							v.Release()
							offset += length
						}
						if c.rows != ref.rows || c.pagePos != ref.pagePos || c.valuePos != ref.valuePos || c.nonNull != ref.nonNull {
							t.Fatal("cursor consumption differs")
						}
						c.close()
						ref.close()
					}
				})
			}
		}
	}
}

func TestParquetFixedMaterializationFailures(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	for _, dictionary := range []bool{false, true} {
		c := makeParquetFixedTestCursor(a, dtype.Int32T(), []uint64{12, 29}, nil, dictionary)
		if dictionary {
			binary.LittleEndian.PutUint32(c.indices.AsBytes()[4:], uint32(c.dictCount))
		} else {
			c.values = c.values[:7]
		}
		v := store.MakeVector(a, 2, dtype.Int32T(), false)
		failure := c.materializeFixed(context.Background(), &v)
		if failure == nil || failure.Code() != ParquetInvalid {
			t.Fatalf("accepted corrupt dictionary=%v: %v", dictionary, failure)
		}
		v.Release()
		c.close()
	}
	c := makeParquetFixedTestCursor(a, dtype.BoolT(), []uint64{1}, nil, false)
	v := store.MakeVector(a, 1, dtype.BoolT(), false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if failure := c.materializeFixed(ctx, &v); failure == nil || failure.Code() != ParquetCancelled || c.rows != 0 {
		t.Fatalf("cancellation: %v, rows %d", failure, c.rows)
	}
	v.Release()
	c.close()
}

func TestParquetHybridRunsRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(53, 29))
	for width := byte(0); width <= 32; width++ {
		for _, length := range []int{0, 1, 7, 8, 9, 63, 65, 4097} {
			mask := uint32(uint64(1)<<width - 1)
			var encoded []byte
			var want []uint32
			for len(want) < length {
				remaining := length - len(want)
				if rng.IntN(2) == 0 {
					count := min(remaining, 1+rng.IntN(97))
					value := rng.Uint32() & mask
					encoded = binary.AppendUvarint(encoded, uint64(count)<<1)
					for i := range int((width + 7) / 8) {
						encoded = append(encoded, byte(value>>(i*8)))
					}
					for range count {
						want = append(want, value)
					}
				} else {
					groups := min((remaining+7)/8, 1+rng.IntN(5))
					encoded = binary.AppendUvarint(encoded, uint64(groups)<<1|1)
					packed := make([]byte, groups*int(width))
					for i := range min(remaining, groups*8) {
						value := rng.Uint32() & mask
						want = append(want, value)
						for bit := range int(width) {
							at := i*int(width) + bit
							packed[at/8] |= byte(value>>bit&1) << (at & 7)
						}
					}
					encoded = append(encoded, packed...)
				}
			}
			dst := make([]byte, length*4)
			if err := decodeParquetIndices(encoded, width, dst); err != nil {
				t.Fatalf("width %d length %d: %v", width, length, err)
			}
			for i, value := range want {
				if got := binary.LittleEndian.Uint32(dst[i*4:]); got != value {
					t.Fatalf("width %d length %d row %d: %d != %d", width, length, i, got, value)
				}
			}
			if width == 1 {
				levels := make([]byte, length)
				if err := decodeParquetRLE(encoded, width, levels); err != nil {
					t.Fatal(err)
				}
				for i, value := range want {
					if levels[i] != byte(value) {
						t.Fatal("level decoder differs")
					}
				}
			}
			if err := decodeParquetIndices(append(encoded, 0), width, dst); err == nil {
				t.Fatal("accepted trailing bytes")
			}
			if len(encoded) > 0 {
				if err := decodeParquetIndices(encoded[:len(encoded)-1], width, dst); err == nil {
					t.Fatalf("accepted truncated width %d length %d", width, length)
				}
			}
		}
	}
}

func TestParquetAllValidLevelsElision(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	for _, levels := range [][]byte{{6, 1}, {6, 1, 0}, {8, 1}, {6, 2}, {3, 7}} {
		c := parquetCursor{a: a, column: parquetColumn{typ: dtype.Int32T(), optional: true}, chunk: parquetChunk{values: 3}, options: ParquetOptions{MaxDecodedPageBytes: 4096}}
		info := compactValue{kind: 12, fields: map[int16]compactValue{
			1: {kind: 5, integer: 3}, 2: {kind: 5, integer: 0}, 3: {kind: 5, integer: 3}, 4: {kind: 5, integer: 3},
		}}
		header := compactValue{kind: 12, fields: map[int16]compactValue{5: info}}
		body := binary.LittleEndian.AppendUint32(nil, uint32(len(levels)))
		body = append(body, levels...)
		for _, value := range []uint32{11, 22, 33} {
			body = binary.LittleEndian.AppendUint32(body, value)
		}
		failure := c.startPage(0, header, body, nil)
		valid := bytes.Equal(levels, []byte{6, 1}) || bytes.Equal(levels, []byte{3, 7})
		if (failure == nil) != valid {
			t.Fatalf("levels %v: %v", levels, failure)
		}
		if bytes.Equal(levels, []byte{6, 1}) && c.levels != nil {
			t.Fatal("all-valid RLE levels materialized")
		}
		if valid {
			v := store.MakeVector(a, 3, dtype.Int32T(), true)
			if failure := c.materializeFixed(context.Background(), &v); failure != nil || v.NiN() != 0 || v.I32s()[2] != 33 {
				t.Fatalf("all-valid materialization: %v", failure)
			}
			v.Release()
		}
		c.close()
	}
}

func TestParquetFixedMaterializationAcrossPagesOwnership(t *testing.T) {
	for _, typ := range []dtype.Type{dtype.Int32T(), dtype.Float64T(), dtype.BoolT()} {
		var source []byte
		var values []uint64
		var valid []bool
		for page, count := range []int{1, 3, 16, 65, 52} {
			var body []byte
			levels := make([]byte, (count+7)/8)
			var payload []byte
			nonNull := 0
			for row := range count {
				value := uint64(page*97 + row*23)
				values = append(values, value)
				yes := page%2 == 0 || row%7 != 0
				valid = append(valid, yes)
				if !yes {
					continue
				}
				levels[row/8] |= 1 << (row & 7)
				if typ.ID() == dtype.BOOLT {
					if nonNull&7 == 0 {
						payload = append(payload, 0)
					}
					payload[nonNull/8] |= byte(value&1) << (nonNull & 7)
				} else if typ.SlotBits() == 32 {
					payload = binary.LittleEndian.AppendUint32(payload, uint32(value))
				} else {
					payload = binary.LittleEndian.AppendUint64(payload, value)
				}
				nonNull++
			}
			var encoded []byte
			if page%2 == 0 {
				encoded = binary.AppendUvarint(nil, uint64(count)<<1)
				encoded = append(encoded, 1)
			} else {
				encoded = binary.AppendUvarint(nil, uint64((count+7)/8)<<1|1)
				encoded = append(encoded, levels...)
			}
			body = binary.LittleEndian.AppendUint32(body, uint32(len(encoded)))
			body = append(body, encoded...)
			body = append(body, payload...)
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
		scope := mem.MakeAllocationScope(a, 1<<20)
		view := mem.MakeAllocatorWithScope(a, scope)
		c := parquetCursor{a: view, column: parquetColumn{typ: typ, optional: true}, chunk: parquetChunk{end: int64(len(source)), values: int64(len(values))}, input: bytes.NewReader(source), options: ParquetOptions{MaxPageBytes: parquetPageLimit, MaxDecodedPageBytes: parquetDecodedLimit}}
		v := store.MakeVector(view, len(values), typ, true)
		if failure := c.materializeFixed(context.Background(), &v); failure != nil {
			t.Fatal(failure)
		}
		if c.pages != 5 || c.nonNull != 0 {
			t.Fatal("pages not fully consumed")
		}
		retained := v.Retain()
		v.Release()
		c.close()
		clear(source)
		for i, want := range values {
			if retained.Validity().IsSet(i) != valid[i] {
				t.Fatal("cross-page validity")
			}
			if !valid[i] {
				continue
			}
			var got uint64
			if typ.ID() == dtype.BOOLT {
				got = uint64(retained.Bools()[i/8] >> (i & 7) & 1)
				want &= 1
			} else if typ.SlotBits() == 32 {
				got = uint64(binary.NativeEndian.Uint32(retained.Data().AsBytes()[i*4:]))
			} else {
				got = binary.NativeEndian.Uint64(retained.Data().AsBytes()[i*8:])
			}
			if got != want {
				t.Fatalf("type %v cross-page row %d: %d != %d", typ.ID(), i, got, want)
			}
		}
		retained.Release()
		if scope.Live() != 0 {
			t.Fatalf("leaked %d bytes", scope.Live())
		}
	}
}

func TestParquetEmptyPageRejectsEncodedZeroRun(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	c := parquetCursor{a: a, column: parquetColumn{typ: dtype.Int32T(), optional: true}, options: ParquetOptions{MaxDecodedPageBytes: 4096}}
	defer c.close()
	info := compactValue{kind: 12, fields: map[int16]compactValue{
		1: {kind: 5, integer: 0}, 2: {kind: 5, integer: 0}, 3: {kind: 5, integer: 0},
		4: {kind: 5, integer: 0}, 5: {kind: 5, integer: 2}, 6: {kind: 5, integer: 0},
	}}
	header := compactValue{kind: 12, fields: map[int16]compactValue{8: info}}
	if failure := c.startPage(3, header, []byte{0, 1}, nil); failure == nil || failure.Code() != ParquetInvalid {
		t.Fatalf("accepted an encoded zero-length RLE run: %v", failure)
	}
}
