package plan

import (
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPackedKeyLayoutMixedTypesAndOrder(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	integers := store.MakeVector(a, 9, dtype.Int32T(), true)
	copy(integers.I32s(), []int32{-30000, -30000, -30000, 35535, 4, 4, 4, 4, 4})
	integers.Validity().Clear(6)
	values := [][]byte{[]byte("a"), []byte("a\x00"), []byte("a\x00\x00\x00"), []byte("abcd"), []byte("a"), []byte("abcd"), []byte("abcd"), []byte("z"), []byte("z")}
	valid := []bool{true, true, true, true, true, true, true, false, true}
	strings := store.MakeStringVector(a, values, valid)
	vectors := []store.Vector{integers, strings}
	stats := makePackedKeyStats([]dtype.Type{dtype.Int32T(), dtype.StringT()})
	if !stats.observe(a, vectors, nil) {
		t.Fatal("cannot gather typed key statistics")
	}
	for _, intsDescending := range []bool{false, true} {
		for _, stringsDescending := range []bool{false, true} {
			for _, nullsFirst := range []bool{false, true} {
				layout, ok := stats.layout(keyOrder, []bool{intsDescending, stringsDescending}, []bool{nullsFirst, nullsFirst})
				if !ok || layout.width != 53 {
					t.Fatalf("expected 53-bit INT32+STRING layout, got %+v", layout)
				}
				indices := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
				compare := func(i, j int) int {
					for col := range vectors {
						v := &vectors[col]
						ni, nj := v.Validity() != nil && !v.Validity().IsSet(i), v.Validity() != nil && !v.Validity().IsSet(j)
						if ni != nj {
							if ni == nullsFirst {
								return -1
							}
							return 1
						}
						if ni {
							continue
						}
						result := 0
						if col == 0 {
							if v.I32s()[i] < v.I32s()[j] {
								result = -1
							} else if v.I32s()[i] > v.I32s()[j] {
								result = 1
							}
							if intsDescending {
								result = -result
							}
						} else {
							x, y := v.Strings()[i].View(), v.Strings()[j].View()
							if x < y {
								result = -1
							} else if x > y {
								result = 1
							}
							if stringsDescending {
								result = -result
							}
						}
						if result != 0 {
							return result
						}
					}
					return 0
				}
				sort.SliceStable(indices, func(i, j int) bool { return compare(indices[i], indices[j]) < 0 })
				var last uint64
				for i, row := range indices {
					packed, valid := layout.encode(vectors, row)
					if !valid {
						t.Fatalf("row %d unexpectedly unencodable", row)
					}
					if i != 0 {
						if packed < last || (packed == last) != (compare(indices[i-1], row) == 0) {
							t.Fatalf("sort order/equality mismatch at row %d: prev=%d current=%d", row, last, packed)
						}
					}
					last = packed
				}
			}
		}
	}
	other := store.MakeVector(a, 1, dtype.Int32T(), false)
	other.I32s()[0] = 35536
	if layout, ok := stats.layout(keyGroup, nil, nil); !ok {
		t.Fatal("group layout failed")
	} else if _, valid := layout.encode([]store.Vector{other, strings}, 0); valid {
		t.Fatal("out-of-range value was truncated into the packed key")
	}
	other.Release()
	strings.Release()
	integers.Release()
}

func TestPackedKeyLayoutFloatAndFallbacks(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	value := store.MakeVector(a, 6, dtype.Float32T(), true)
	copy(value.F32s(), []float32{0, float32(math.Copysign(0, -1)), math.Float32frombits(0x7fc00001), math.Float32frombits(0x7fc00002), float32(math.Inf(1)), 3})
	value.Validity().Clear(5)
	stats := makePackedKeyStats([]dtype.Type{dtype.Float32T()})
	if !stats.observe(a, []store.Vector{value}, nil) {
		t.Fatal("float statistics failed")
	}
	group, ok := stats.layout(keyGroup, nil, nil)
	if !ok || group.width != 33 {
		t.Fatalf("nullable FLOAT32 needs 33 bits, got %+v", group)
	}
	for _, pair := range [][2]int{{0, 1}, {2, 3}} {
		x, _ := group.encode([]store.Vector{value}, pair[0])
		y, _ := group.encode([]store.Vector{value}, pair[1])
		if x != y {
			t.Fatalf("grouping key differs for %d,%d", pair[0], pair[1])
		}
	}
	join, ok := stats.layout(keyJoin, nil, nil)
	if !ok || join.width != 32 {
		t.Fatalf("join key should be 32 bits, got %+v", join)
	}
	for _, row := range []int{2, 3, 5} {
		if _, valid := join.encode([]store.Vector{value}, row); valid {
			t.Fatalf("join accepted NaN/NULL row %d", row)
		}
	}
	x, _ := join.encode([]store.Vector{value}, 0)
	y, _ := join.encode([]store.Vector{value}, 1)
	if x != y {
		t.Fatal("signed zeros produced different join keys")
	}
	value.Release()

	f64 := store.MakeVector(a, 1, dtype.Float64T(), false)
	text := store.MakeStringVector(a, [][]byte{[]byte("x")}, nil)
	stats = makePackedKeyStats([]dtype.Type{dtype.Float64T(), dtype.StringT()})
	if !stats.observe(a, []store.Vector{f64, text}, nil) {
		t.Fatal("mixed statistics failed")
	}
	if _, ok := stats.layout(keyOrder, []bool{false, false}, []bool{false, false}); ok {
		t.Fatal("packed more than 64 bits")
	}
	f64.Release()
	text.Release()
	long := store.MakeStringVector(a, [][]byte{[]byte("longer than seven bytes")}, nil)
	stats = makePackedKeyStats([]dtype.Type{dtype.StringT()})
	if !stats.observe(a, []store.Vector{long}, nil) {
		t.Fatal("string stats failed")
	}
	if _, ok := stats.layout(keyGroup, nil, nil); ok {
		t.Fatal("packed an overlong string")
	}
	long.Release()
}

func TestPackedKeyLayoutRandomCombinations(t *testing.T) {
	rng := rand.New(rand.NewPCG(139, 97))
	a := mem.MakeAllocatorWithProfiles([]int{16384}, []int{16384})
	choices := []dtype.Type{dtype.Int32T(), dtype.Int64T(), dtype.Float32T(), dtype.BoolT(), dtype.StringT(), dtype.DateT()}
	covered := 0
	for round := range 160 {
		columnCount := 2 + rng.IntN(3)
		types := make([]dtype.Type, columnCount)
		vectors := make([]store.Vector, columnCount)
		descending, nullsFirst := make([]bool, columnCount), make([]bool, columnCount)
		const rows = 48
		for col := range vectors {
			types[col] = choices[rng.IntN(len(choices))]
			descending[col], nullsFirst[col] = rng.IntN(2) == 0, rng.IntN(2) == 0
			if types[col].ID() == dtype.STRT {
				strings := make([][]byte, rows)
				valid := make([]bool, rows)
				for row := range rows {
					strings[row] = make([]byte, rng.IntN(4))
					for i := range strings[row] {
						strings[row][i] = byte(rng.IntN(5))
					}
					valid[row] = rng.IntN(7) != 0
				}
				vectors[col] = store.MakeStringVector(a, strings, valid)
			} else {
				vectors[col] = store.MakeVector(a, rows, types[col], true)
				for row := range rows {
					if rng.IntN(7) == 0 {
						vectors[col].Validity().Clear(row)
					}
					value := int32(rng.IntN(101) - 50)
					switch types[col].ID() {
					case dtype.INT32T, dtype.DATET:
						vectors[col].I32s()[row] = value
					case dtype.INT64T:
						vectors[col].I64s()[row] = int64(value)
					case dtype.FLOAT32T:
						vectors[col].F32s()[row] = float32(value)
						if row%13 == 0 {
							vectors[col].F32s()[row] = math.Float32frombits(0x7fc00000 | uint32(row&3))
						}
						if row%17 == 0 {
							vectors[col].F32s()[row] = float32(math.Copysign(0, -1))
						}
					case dtype.BOOLT:
						if value&1 != 0 {
							vectors[col].Bools()[row>>3] |= 1 << (row & 7)
						}
					}
				}
			}
		}
		stats := makePackedKeyStats(types)
		if !stats.observe(a, vectors, nil) {
			t.Fatal("could not observe randomized key types")
		}
		layout, ok := stats.layout(keyOrder, descending, nullsFirst)
		if ok {
			covered++
			for left := range rows {
				first, _ := layout.encode(vectors, left)
				for right := range rows {
					second, _ := layout.encode(vectors, right)
					want := 0
					for col := range vectors {
						v := &vectors[col]
						ln, rn := !v.Validity().IsSet(left), !v.Validity().IsSet(right)
						if ln != rn {
							if ln == nullsFirst[col] {
								want = -1
							} else {
								want = 1
							}
							break
						}
						if ln {
							continue
						}
						cmp := 0
						switch v.TypeID() {
						case dtype.INT32T, dtype.DATET:
							if v.I32s()[left] < v.I32s()[right] {
								cmp = -1
							} else if v.I32s()[left] > v.I32s()[right] {
								cmp = 1
							}
						case dtype.INT64T:
							if v.I64s()[left] < v.I64s()[right] {
								cmp = -1
							} else if v.I64s()[left] > v.I64s()[right] {
								cmp = 1
							}
						case dtype.FLOAT32T:
							x, y := v.F32s()[left], v.F32s()[right]
							if math.IsNaN(float64(x)) && !math.IsNaN(float64(y)) {
								cmp = 1
							} else if !math.IsNaN(float64(x)) && math.IsNaN(float64(y)) {
								cmp = -1
							} else if x < y {
								cmp = -1
							} else if x > y {
								cmp = 1
							}
						case dtype.BOOLT:
							x, y := v.Bools()[left>>3]>>(left&7)&1, v.Bools()[right>>3]>>(right&7)&1
							if x < y {
								cmp = -1
							} else if x > y {
								cmp = 1
							}
						case dtype.STRT:
							x, y := v.Strings()[left].View(), v.Strings()[right].View()
							if x < y {
								cmp = -1
							} else if x > y {
								cmp = 1
							}
						}
						if cmp != 0 {
							if descending[col] {
								cmp = -cmp
							}
							want = cmp
							break
						}
					}
					got := 0
					if first < second {
						got = -1
					} else if first > second {
						got = 1
					}
					if got != want {
						t.Fatalf("round %d columns %v rows %d,%d: packed=%d/%d got %d, want %d", round, types, left, right, first, second, got, want)
					}
				}
			}
		}
		for i := range vectors {
			vectors[i].Release()
		}
	}
	if covered < 10 {
		t.Fatalf("too few packable combinations: %d", covered)
	}
}
