package parquet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestParquetSelectedStringsDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(514))
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	for _, n := range []int{0, 1, 7, 8, 9, 63, 65, 4097} {
		for _, dictionary := range []bool{false, true} {
			values, valid := make([][]byte, n), make([]bool, n)
			mask := store.MakeBitMap(a, n)
			for row := range n {
				values[row] = []byte(fmt.Sprintf("binary\x00 utf8 ø row %05d %s", row, bytes.Repeat([]byte{'x'}, rng.Intn(193))))
				valid[row] = rng.Intn(5) != 0
				if rng.Intn(13) == 0 {
					mask.Set(row)
				}
			}
			c := makeParquetStringTestCursor(a, values, valid, dictionary)
			v, failure := makeParquetSelectedStringVector(context.Background(), &c, n, mask)
			if failure != nil {
				t.Fatal(failure)
			}
			backing := 0
			for row := range n {
				if v.Validity().IsSet(row) != valid[row] {
					t.Fatal("changed source nullability")
				}
				if valid[row] && mask.IsSet(row) {
					if v.StringAt(row).View() != string(values[row]) {
						t.Fatalf("n=%d dictionary=%t row=%d changed selected value", n, dictionary, row)
					}
					backing += len(values[row])
				} else if v.StringAt(row).Len() != 0 {
					t.Fatal("copied rejected payload")
				}
			}
			if v.Backing() != nil && v.Backing().Len() != backing {
				t.Fatalf("backing=%d want %d", v.Backing().Len(), backing)
			}
			retained := v.Retain()
			v.Release()
			c.close()
			for row := range n {
				if mask.IsSet(row) && valid[row] && retained.StringAt(row).View() != string(values[row]) {
					t.Fatal("retained payload changed after closing pages")
				}
			}
			retained.Release()
			mask.Release()
		}
	}
	if u := a.Usage(); u.GeneralUsed != 0 || u.ScratchUsed != 0 {
		t.Fatalf("leaked selected strings %+v", u)
	}
}

func TestParquetSelectedStringsBudgetAndInvalidRejectedValues(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	values := make([][]byte, 128)
	for row := range values {
		values[row] = bytes.Repeat([]byte{'x'}, 8192)
	}
	for _, selected := range []bool{false, true} {
		c := makeParquetStringTestCursor(a, values, nil, false)
		scope := mem.MakeAllocationScope(a, 64<<10)
		c.a = mem.MakeAllocatorWithScope(a, scope)
		mask := store.MakeBitMap(a, len(values))
		mask.Set(0)
		var use *store.BitMap
		if selected {
			use = mask
		}
		v, failure := makeParquetSelectedStringVector(context.Background(), &c, len(values), use)
		if selected && (failure != nil || scope.Exhausted()) || !selected && (failure == nil || failure.Code() != ParquetResourceExhausted) {
			t.Fatalf("selected=%t failure=%v live=%d", selected, failure, scope.Live())
		}
		v.Release()
		c.close()
		mask.Release()
		if scope.Live() != 0 {
			t.Fatalf("selected=%t leaked %d bytes", selected, scope.Live())
		}
	}
	for _, dictionary := range []bool{false, true} {
		c := makeParquetStringTestCursor(a, [][]byte{[]byte("valid"), []byte("other")}, nil, dictionary)
		if dictionary {
			binary.LittleEndian.PutUint32(c.indices.AsBytes()[4:], uint32(c.dictCount))
		} else {
			c.values[len(c.values)-1] = 0xff
		}
		mask := store.MakeBitMap(a, 2)
		mask.Set(0)
		v, failure := makeParquetSelectedStringVector(context.Background(), &c, 2, mask)
		if failure == nil || failure.Code() != ParquetInvalid {
			t.Fatalf("accepted rejected corrupt value dictionary=%t failure=%v", dictionary, failure)
		}
		v.Release()
		c.close()
		mask.Release()
	}
	if u := a.Usage(); u.GeneralUsed != 0 || u.ScratchUsed != 0 {
		t.Fatalf("leak %+v", u)
	}
}

func TestParquetPredicateFirstMaterialization(t *testing.T) {
	data := parquetPredicateFixture(t)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	var copied [2]int
	for mode := range 2 {
		r, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, []int{3, 1, 0}, ParquetOptions{BatchSize: 317})
		if failure != nil {
			t.Fatal(failure)
		}
		if !r.SetSelectedMaterialization(mode != 0) || !r.SetScanPredicates([]ScanPredicate{{Column: 1, Op: PruneEqual, Integer: -7}}) {
			t.Fatal("rejected configuration")
		}
		rows := 0
		var held []*store.Batch
		for {
			batch, failure := r.Next(context.Background())
			if failure != nil {
				t.Fatal(failure)
			}
			if batch == nil {
				break
			}
			if r.SetSelectedMaterialization(false) {
				t.Fatal("changed started reader")
			}
			v := batch.VectorAt(0)
			if v.Backing() != nil {
				copied[mode] += v.Backing().Len()
			}
			mask, _ := batch.Selection().AsBitMap()
			for row := range batch.Len() {
				seq := int(batch.VectorAt(2).I32s()[row])
				active := mask == nil || mask.IsSet(row)
				if active {
					rows++
					if seq%17 == 0 || seq*48271%101-50 != -7 || v.Validity().IsSet(row) != (seq%23 != 0) || seq%23 != 0 && v.StringAt(row).View() != parquetPredicateText(seq) {
						t.Fatalf("incorrect active row %d", seq)
					}
				} else if mode != 0 && v.StringAt(row).Len() != 0 {
					t.Fatal("predicate executed after payload copy")
				}
			}
			held = append(held, batch.Retain())
			batch.Release()
		}
		r.Close()
		for _, b := range held {
			mask, _ := b.Selection().AsBitMap()
			for row := range b.Len() {
				if mask.IsSet(row) && b.VectorAt(0).Validity().IsSet(row) && b.VectorAt(0).StringAt(row).View() != parquetPredicateText(int(b.VectorAt(2).I32s()[row])) {
					t.Fatal("retained selected payload changed")
				}
			}
			b.Release()
		}
		expected := 0
		for seq := range 4097 {
			if seq%17 != 0 && seq*48271%101-50 == -7 {
				expected++
			}
		}
		if rows != expected {
			t.Fatalf("selected rows=%d", rows)
		}
	}
	if copied[1] == 0 || copied[1]*10 >= copied[0] {
		t.Fatalf("payload copying did not decrease: dense=%d selected=%d", copied[0], copied[1])
	}
	if u := a.Usage(); u.GeneralUsed != 0 || u.ScratchUsed != 0 {
		t.Fatalf("retained reader leaked %+v", u)
	}
}

func TestParquetSkipDifferential(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	for _, typ := range []dtype.Type{dtype.BoolT(), dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.Float64T()} {
		for _, dictionary := range []bool{false, true} {
			values, valid := make([]uint64, 4097), make([]bool, 4097)
			for row := range values {
				values[row], valid[row] = uint64(row%2), row%3 != 0
			}
			c := makeParquetFixedTestCursor(a, typ, values, valid, dictionary)
			ref := makeParquetFixedTestCursor(a, typ, values, valid, dictionary)
			for on := 0; on < len(values); {
				n := min(317, len(values)-on)
				if failure := c.skip(context.Background(), n); failure != nil {
					t.Fatal(failure)
				}
				for range n {
					if _, _, failure := ref.next(context.Background()); failure != nil {
						t.Fatal(failure)
					}
				}
				if c.rows != ref.rows || c.pagePos != ref.pagePos || c.valuePos != ref.valuePos || c.nonNull != ref.nonNull {
					t.Fatalf("skip differs typ=%v dictionary=%t", typ, dictionary)
				}
				on += n
			}
			c.close()
			ref.close()
		}
	}
	if u := a.Usage(); u.GeneralUsed != 0 || u.ScratchUsed != 0 {
		t.Fatalf("skip leaked %+v", u)
	}
}

func TestParquetSelectedRowGroupReaders(t *testing.T) {
	data := parquetPredicateFixture(t)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	for _, dictionary := range []bool{false, true} {
		for _, predicates := range [][]ScanPredicate{
			{{Column: 1, Op: PruneEqual, Integer: -7}},
			{{Column: 1, Op: PruneEqual, Integer: -7}, {Column: 3, Op: PruneNotEqual, Text: "north"}},
			{{Column: 1, Op: PruneEqual, Integer: -7}, {Column: 0, Op: PruneLess, Integer: 50}},
			{{Column: 2, Op: PruneEqual, Integer: 1}, {Column: 3, Op: PruneEqual, Text: "a\x00b"}},
		} {
			var readers [2]*ParquetReader
			for mode := range readers {
				r, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, []int{3, 1, 0}, ParquetOptions{BatchSize: 317})
				if failure != nil {
					t.Fatal(failure)
				}
				r.SetScanPredicates(predicates)
				r.SetSelectedMaterialization(mode != 0)
				if dictionary && !r.SetDictionaryColumns([]int{3}) {
					t.Fatal("dictionary configuration")
				}
				readers[mode] = r
			}
			for group := range readers[0].RowGroupCount() {
				clones := [2]*ParquetReader{readers[0].MakeRowGroupReader(group, a), readers[1].MakeRowGroupReader(group, a)}
				if !clones[1].selectedMaterialization {
					t.Fatal("clone lost selected materialization")
				}
				for {
					left, failure := clones[0].Next(context.Background())
					if failure != nil {
						t.Fatal(failure)
					}
					right, failure := clones[1].Next(context.Background())
					if failure != nil {
						t.Fatal(failure)
					}
					if left == nil || right == nil {
						if left != right {
							t.Fatal("different emitted batch count")
						}
						break
					}
					if left.Len() != right.Len() || left.ActiveLen() != right.ActiveLen() {
						t.Fatal("changed physical row domain")
					}
					lm, _ := left.Selection().AsBitMap()
					rm, _ := right.Selection().AsBitMap()
					for row := range left.Len() {
						la, ra := lm == nil || lm.IsSet(row), rm == nil || rm.IsSet(row)
						if la != ra {
							t.Fatal("changed row selection")
						}
						if !la {
							continue
						}
						for col := range left.NVectors() {
							lv, rv := left.VectorAt(col), right.VectorAt(col)
							if (lv.Validity() == nil || lv.Validity().IsSet(row)) != (rv.Validity() == nil || rv.Validity().IsSet(row)) {
								t.Fatal("changed active null")
							}
							if lv.Validity() != nil && !lv.Validity().IsSet(row) {
								continue
							}
							if col == 0 && lv.StringAt(row).View() != rv.StringAt(row).View() || col != 0 && lv.I32s()[row] != rv.I32s()[row] {
								t.Fatal("changed active payload")
							}
						}
					}
					left.Release()
					right.Release()
				}
				for _, r := range clones {
					r.Close()
				}
			}
			for _, r := range readers {
				r.Close()
			}
		}
	}
	if u := a.Usage(); u.GeneralUsed != 0 || u.ScratchUsed != 0 {
		t.Fatalf("clones leaked %+v", u)
	}
}

func TestParquetSkipInvalidAndCancelled(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	for _, dictionary := range []bool{false, true} {
		c := makeParquetFixedTestCursor(a, dtype.Int32T(), []uint64{12, 29}, nil, dictionary)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if failure := c.skip(ctx, 2); failure == nil || failure.Code() != ParquetCancelled {
			t.Fatalf("cancelled skip failure=%v", failure)
		}
		if dictionary {
			binary.LittleEndian.PutUint32(c.indices.AsBytes()[4:], uint32(c.dictCount))
		} else {
			c.values = c.values[:len(c.values)-1]
		}
		if failure := c.skip(context.Background(), 2); failure == nil || failure.Code() != ParquetInvalid {
			t.Fatalf("corrupt skip dictionary=%t failure=%v", dictionary, failure)
		}
		c.close()
	}
	if u := a.Usage(); u.GeneralUsed != 0 || u.ScratchUsed != 0 {
		t.Fatalf("skip leaked %+v", u)
	}
}
