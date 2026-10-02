package parquet

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

// DuckDB 1.5.6, Snappy, dictionary strings/integers, nullable BOOL, three groups.
func parquetPredicateFixture(t *testing.T) []byte {
	t.Helper()
	encoded, err := os.ReadFile("testdata/duckdb-predicates.b64")
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestParquetScanPredicateDifferential(t *testing.T) {
	data := parquetPredicateFixture(t)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	var cases [][]ScanPredicate
	for op := PruneEqual; op <= PruneGreaterEqual; op++ {
		cases = append(cases, []ScanPredicate{{Column: 1, Op: op, Integer: -7}}, []ScanPredicate{{Column: 3, Op: op, Text: "north"}})
	}
	cases = append(cases, nil,
		[]ScanPredicate{{Column: 0, Op: PruneGreaterEqual, Integer: 0}},
		[]ScanPredicate{{Column: 0, Op: PruneGreater, Integer: 5000}},
		[]ScanPredicate{{Column: 2, Op: PruneEqual, Integer: 1}},
		[]ScanPredicate{{Column: 2, Op: PruneNotEqual, Integer: 1}},
		[]ScanPredicate{{Column: 3, Op: PruneEqual, Text: ""}},
		[]ScanPredicate{{Column: 3, Op: PruneEqual, Text: "a\x00b"}},
		[]ScanPredicate{{Column: 1, Op: PruneGreaterEqual, Integer: -50}, {Column: 2, Op: PruneEqual, Integer: 1}, {Column: 3, Op: PruneNotEqual, Text: ""}})
	for _, predicates := range cases {
		for _, selected := range [][]int{nil, {0, 3}, {0}, {}} {
			r, failure := MakeParquetReaderProjected(bytes.NewReader(data), int64(len(data)), a, selected, ParquetOptions{BatchSize: 317})
			if failure != nil {
				t.Fatal(failure)
			}
			if !r.SetScanPredicates(predicates) {
				t.Fatal("predicate rejected")
			}
			rows, expected := 0, 0
			matches := func(seq int) bool {
				for _, p := range predicates {
					var cmp int
					switch p.Column {
					case 0:
						if int64(seq) < p.Integer {
							cmp = -1
						} else if int64(seq) > p.Integer {
							cmp = 1
						}
					case 1:
						if seq%17 == 0 {
							return false
						}
						value := int64(seq*48271%101 - 50)
						if value < p.Integer {
							cmp = -1
						} else if value > p.Integer {
							cmp = 1
						}
					case 2:
						if seq%19 == 0 {
							return false
						}
						value := int64(0)
						if seq%3 == 0 {
							value = 1
						}
						if value < p.Integer {
							cmp = -1
						} else if value > p.Integer {
							cmp = 1
						}
					case 3:
						if seq%23 == 0 {
							return false
						}
						cmp = strings.Compare(parquetPredicateText(seq), p.Text)
					}
					// Independent scalar comparison, not the reader helper.
					pass := p.Op == PruneEqual && cmp == 0 || p.Op == PruneNotEqual && cmp != 0 || p.Op == PruneLess && cmp < 0 || p.Op == PruneLessEqual && cmp <= 0 || p.Op == PruneGreater && cmp > 0 || p.Op == PruneGreaterEqual && cmp >= 0
					if !pass {
						return false
					}
				}
				return true
			}
			for seq := range 4097 {
				if matches(seq) {
					expected++
				}
			}
			for {
				batch, failure := r.Next(context.Background())
				if failure != nil {
					t.Fatal(failure)
				}
				if batch == nil {
					break
				}
				base := int(r.row) - batch.Len()
				for group := 0; group < r.group; group++ {
					base += int(r.groups[group].rows)
				}
				bitmap, _ := batch.Selection().AsBitMap()
				for row := range batch.Len() {
					seq := base + row
					active := bitmap == nil || bitmap.IsSet(row)
					if active != matches(seq) {
						t.Fatalf("predicates=%v projection=%v seq=%d active=%t", predicates, selected, seq, active)
					}
					if active {
						rows++
					}
					for at, col := range r.selected {
						v := batch.VectorAt(at)
						switch col {
						case 0:
							if v.I32s()[row] != int32(seq) {
								t.Fatal("physical seq changed")
							}
						case 1:
							if v.Validity().IsSet(row) != (seq%17 != 0) || seq%17 != 0 && v.I32s()[row] != int32(seq*48271%101-50) {
								t.Fatal("physical id changed")
							}
						case 2:
							if v.Validity().IsSet(row) != (seq%19 != 0) || seq%19 != 0 && (v.Bools()[row/8]>>(row&7)&1 != 0) != (seq%3 == 0) {
								t.Fatal("physical bool changed")
							}
						case 3:
							if v.Validity().IsSet(row) != (seq%23 != 0) || seq%23 != 0 && v.Strings()[row].View() != parquetPredicateText(seq) {
								t.Fatal("physical string changed")
							}
						}
					}
				}
				retained := batch.Retain()
				batch.Release()
				if retained.ActiveLen() < 1 && len(predicates) != 0 {
					t.Fatal("emitted entirely rejected batch")
				}
				retained.Release()
			}
			r.Close()
			if rows != expected {
				t.Fatalf("matched %d rows, want %d predicates=%v", rows, expected, predicates)
			}
			if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
				t.Fatalf("leaked reader state: %+v", usage)
			}
		}
	}
}

func parquetPredicateText(seq int) string {
	switch seq % 7 {
	case 0:
		return ""
	case 1:
		return "north"
	case 2:
		return "a\x00b"
	default:
		return "long category with utf8 ø " + string(rune('0'+seq%7))
	}
}

func TestParquetScanPredicateProofSkipsColumn(t *testing.T) {
	data := parquetPredicateFixture(t)
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	guard := &guardedParquetRange{ReaderAt: bytes.NewReader(data)}
	r, failure := MakeParquetReaderProjected(guard, int64(len(data)), a, []int{3}, ParquetOptions{BatchSize: 257})
	if failure != nil {
		t.Fatal(failure)
	}
	defer r.Close()
	guard.start, guard.end = r.groups[0].chunks[0].start, r.groups[0].chunks[0].end
	r.SetScanPredicates([]ScanPredicate{{Column: 0, Op: PruneGreaterEqual, Integer: 0}})
	rows := 0
	for {
		b, e := r.Next(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		if b == nil {
			break
		}
		rows += b.Len()
		b.Release()
	}
	if guard.blocked != 0 || rows != 4097 {
		t.Fatalf("proof read column, reads=%d rows=%d", guard.blocked, rows)
	}
	group := parquetGroup{rows: 3, chunks: []parquetChunk{{hasMinMax: true, min: -3, max: 8, noNull: false}}}
	proof := &ParquetReader{columns: []parquetColumn{{typ: dtype.Int32T(), optional: true}}}
	p := ScanPredicate{Column: 0, Op: PruneGreaterEqual, Integer: -3}
	if proof.predicateAll(p, group) {
		t.Fatal("ignored unknown or nonzero null count")
	}
	group.chunks[0].noNull = true
	if !proof.predicateAll(p, group) {
		t.Fatal("missed all-match proof")
	}
	group.chunks[0].hasMinMax = false
	if proof.predicateAll(p, group) {
		t.Fatal("used absent bounds")
	}
}

func TestParquetPredicateDictionaryValidationAndCancellation(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	for _, dictionary := range []bool{false, true} {
		c := makeParquetFixedTestCursor(a, dtype.Int32T(), []uint64{0, 1, 2, 3}, nil, dictionary)
		mask := store.MakeBitMap(a, 4)
		state := parquetPredicateState{predicates: []ScanPredicate{{Op: PruneGreaterEqual, Integer: 0}}}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if e := state.scan(ctx, &c, 4, mask); e == nil || e.Code() != ParquetCancelled {
			t.Fatalf("cancelled scan: %v", e)
		}
		if dictionary {
			binary.LittleEndian.PutUint32(c.indices.AsBytes()[4:], uint32(c.dictCount))
			if e := state.scan(context.Background(), &c, 4, mask); e == nil || e.Code() != ParquetInvalid {
				t.Fatalf("masked corrupt index: %v", e)
			}
		}
		if state.matches != nil {
			state.matches.Dec()
		}
		mask.Release()
		c.close()
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leak %+v", usage)
	}
}

func TestParquetZeroColumnDomain(t *testing.T) {
	data := parquetPredicateFixture(t)
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	footer := len(data) - 8 - int(binary.LittleEndian.Uint32(data[len(data)-8:]))
	guard := &guardedParquetRange{ReaderAt: bytes.NewReader(data), start: 4, end: int64(footer)}
	r, failure := MakeParquetReaderProjected(guard, int64(len(data)), a, []int{}, ParquetOptions{BatchSize: 31})
	if failure != nil {
		t.Fatal(failure)
	}
	defer r.Close()
	rows := 0
	for {
		b, e := r.Next(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		if b == nil {
			break
		}
		if b.NVectors() != 0 || b.ActiveLen() != b.Len() {
			t.Fatal("lost row domain")
		}
		rows += b.Len()
		b.Release()
	}
	if rows != 4097 || guard.blocked != 0 {
		t.Fatalf("rows=%d payload reads=%d", rows, guard.blocked)
	}
}

func TestParquetBooleanPredicatePackedSelection(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	for _, n := range []int{1, 7, 8, 9, 65, 4097} {
		values := make([]uint64, n)
		for i := range values {
			if i%3 == 0 {
				values[i] = 1
			}
		}
		for _, p := range []ScanPredicate{{Op: PruneEqual, Integer: 0}, {Op: PruneEqual, Integer: 1}, {Op: PruneNotEqual, Integer: 0}, {Op: PruneNotEqual, Integer: 1}} {
			for _, split := range []int{7, 317, 4096} {
				c := makeParquetFixedTestCursor(a, dtype.BoolT(), values, nil, false)
				state := parquetPredicateState{predicates: []ScanPredicate{p}}
				for on := 0; on < n; {
					length := min(split, n-on)
					mask := store.MakeBitMap(a, length)
					for row := range length {
						if (on+row)%5 != 0 {
							mask.Set(row)
						}
					}
					if e := state.scan(context.Background(), &c, length, mask); e != nil {
						t.Fatal(e)
					}
					count := 0
					for row := range length {
						match := int64(values[on+row]) == p.Integer
						if p.Op == PruneNotEqual {
							match = !match
						}
						match = match && (on+row)%5 != 0
						if mask.IsSet(row) != match {
							t.Fatalf("n=%d split=%d row=%d predicate=%v", n, split, on+row, p)
						}
						if match {
							count++
						}
					}
					if mask.ViN() != count {
						t.Fatalf("stale bitmap count %d want %d", mask.ViN(), count)
					}
					mask.Release()
					on += length
				}
				c.close()
			}
		}
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leak %+v", usage)
	}
}

func TestParquetScanProofRequiresUsableNullStatistics(t *testing.T) {
	data := parquetPredicateFixture(t)
	footerStart := len(data) - 8 - int(binary.LittleEndian.Uint32(data[len(data)-8:]))
	footer := data[footerStart : len(data)-8]
	at := bytes.Index(footer, []byte{0x16, 0, 0x28, 4})
	if at < 0 {
		t.Fatal("missing first-column null-count fixture field")
	}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	for _, value := range []struct {
		kind  byte
		count int64
	}{{5, 0}, {6, -1}, {6, 2049}} {
		changedFooter := bytes.Clone(footer[:at])
		changedFooter = append(changedFooter, 0x10|value.kind)
		changedFooter = binary.AppendUvarint(changedFooter, uint64(value.count<<1)^uint64(value.count>>63))
		changedFooter = append(changedFooter, footer[at+2:]...)
		changed := bytes.Clone(data[:footerStart])
		changed = append(changed, changedFooter...)
		changed = binary.LittleEndian.AppendUint32(changed, uint32(len(changedFooter)))
		changed = append(changed, "PAR1"...)
		guard := &guardedParquetRange{ReaderAt: bytes.NewReader(changed)}
		r, failure := MakeParquetReaderProjected(guard, int64(len(changed)), a, []int{3}, ParquetOptions{})
		if failure != nil {
			t.Fatal(failure)
		}
		chunk := r.groups[0].chunks[0]
		if !r.columns[0].optional || chunk.noNull || chunk.allNull {
			t.Fatalf("trusted unusable null statistics %+v", chunk)
		}
		guard.start, guard.end = chunk.start, chunk.end
		r.SetScanPredicates([]ScanPredicate{{Column: 0, Op: PruneGreaterEqual, Integer: 0}})
		if b, e := r.Next(context.Background()); b != nil || e == nil || e.Code() != ParquetReadFailure || guard.blocked != 1 {
			t.Fatalf("missing conservative column read: batch=%v error=%v blocked=%d", b, e, guard.blocked)
		}
		r.Close()
	}
}
