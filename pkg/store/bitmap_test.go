package store

import (
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestBitMapSetClearAndRanges(t *testing.T) {
	lengths := []int{0, 1, 7, 8, 9, 15, 16, 17, 63, 64, 65}
	for _, length := range lengths {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
			m := makeTestBitMap(a, make([]bool, length))

			m.SetAll()
			if m.ViN() != length || m.NiN() != 0 {
				t.Fatalf("set all counts: got %d/%d, expected %d/0", m.ViN(), m.NiN(), length)
			}
			assertBitMapTail(t, m)
			for start := -1; start <= length+1; start++ {
				for stop := -1; stop <= length+1; stop++ {
					cleanStart, cleanStop := cleanTestRange(length, start, stop)
					want := cleanStop - cleanStart
					if got := m.RangeViN(start, stop); got != want {
						t.Fatalf("set range [%d,%d): got %d, expected %d", start, stop, got, want)
					}
					if got := m.RangeNiN(start, stop); got != 0 {
						t.Fatalf("clear range [%d,%d): got %d, expected 0", start, stop, got)
					}
				}
			}

			m.ClearAll()
			if m.ViN() != 0 || m.NiN() != length {
				t.Fatalf("clear all counts: got %d/%d, expected 0/%d", m.ViN(), m.NiN(), length)
			}
			for start := 0; start <= length; start++ {
				for stop := start; stop <= length; stop++ {
					if got := m.RangeNiN(start, stop); got != stop-start {
						t.Fatalf("null range [%d,%d): got %d, expected %d", start, stop, got, stop-start)
					}
				}
			}
			m.Release()
		})
	}
}

func TestBitMapBinaryOperations(t *testing.T) {
	operations := []struct {
		name string
		fn   func(*BitMap, *BitMap, *BitMap) int
		ref  func(bool, bool) bool
	}{
		{"and", AndViNFromBMs, func(x, y bool) bool { return x && y }},
		{"or", OrViNFromBMs, func(x, y bool) bool { return x || y }},
		{"xor", XorViNFromBMs, func(x, y bool) bool { return x != y }},
		{"and-not", AndNViNFromBMs, func(x, y bool) bool { return x && !y }},
	}
	lengths := []int{0, 1, 7, 8, 9, 31, 32, 33, 65}
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})

	for _, length := range lengths {
		for iteration := range 100 {
			xBits := randomBits(length)
			yBits := randomBits(length)
			for _, operation := range operations {
				t.Run(operation.name+"/"+strconv.Itoa(length)+"/"+strconv.Itoa(iteration), func(t *testing.T) {
					expected := make([]bool, length)
					var expectedCount int
					for i := range expected {
						expected[i] = operation.ref(xBits[i], yBits[i])
						if expected[i] {
							expectedCount++
						}
					}

					x := makeTestBitMap(a, xBits)
					y := makeTestBitMap(a, yBits)
					z := makeTestBitMap(a, make([]bool, length))
					if got := operation.fn(x, y, z); got != expectedCount {
						t.Fatalf("count: got %d, expected %d", got, expectedCount)
					}
					assertBitMap(t, z, expected)

					xAlias := makeTestBitMap(a, xBits)
					if got := operation.fn(xAlias, y, xAlias); got != expectedCount {
						t.Fatalf("left alias count: got %d, expected %d", got, expectedCount)
					}
					assertBitMap(t, xAlias, expected)

					yAlias := makeTestBitMap(a, yBits)
					if got := operation.fn(x, yAlias, yAlias); got != expectedCount {
						t.Fatalf("right alias count: got %d, expected %d", got, expectedCount)
					}
					assertBitMap(t, yAlias, expected)

					x.Release()
					y.Release()
					z.Release()
					xAlias.Release()
					yAlias.Release()
				})
			}
		}
	}
}

func TestBitMapRetainRelease(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	m := makeTestBitMap(a, []bool{true, false, true})
	retained := m.Retain()
	if got := m.Data().RefCount(); got != 2 {
		t.Fatalf("reference count: got %d, expected 2", got)
	}

	m.Release()
	m.Release()
	if got := retained.Data().RefCount(); got != 1 {
		t.Fatalf("reference count after release: got %d, expected 1", got)
	}
	assertBitMap(t, retained, []bool{true, false, true})
	retained.Release()
}

func TestBitMapCleansInputShape(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	short := a.AllocSeg(1)
	short.AsBytes()[0] = 0xff
	m := MakeBitMapWithUnknownNiN(20, short)
	if m.Len() != 8 || m.ViN() != 8 {
		t.Fatalf("cleaned bitmap: got length/count %d/%d, expected 8/8", m.Len(), m.ViN())
	}

	x := makeTestBitMap(a, []bool{true, true})
	z := makeTestBitMap(a, []bool{true, true, true})
	if got := AndViNFromBMs(m, x, z); got != 0 || z.ViN() != 0 {
		t.Fatalf("mismatched shapes: got count %d and output count %d", got, z.ViN())
	}
	m.Release()
	x.Release()
	z.Release()
}

func makeTestBitMap(a *mem.Allocator, values []bool) *BitMap {
	segment := a.AllocSeg((len(values) + 7) >> 3)
	data := segment.AsBytes()
	for i := range data {
		data[i] = 0
	}
	for i, value := range values {
		if value {
			data[i>>3] |= 1 << (i & 7)
		}
	}
	return MakeBitMapWithUnknownNiN(len(values), segment)
}

func randomBits(length int) []bool {
	values := make([]bool, length)
	for i := range values {
		values[i] = rand.IntN(2) == 1
	}
	return values
}

func assertBitMap(t *testing.T, m *BitMap, expected []bool) {
	t.Helper()
	if m.Len() != len(expected) {
		t.Fatalf("length: got %d, expected %d", m.Len(), len(expected))
	}
	var count int
	for i, expectedValue := range expected {
		got := (m.Bytes()[i>>3]>>(i&7))&1 == 1
		if got != expectedValue {
			t.Fatalf("bit %d: got %t, expected %t", i, got, expectedValue)
		}
		if got {
			count++
		}
	}
	if m.ViN() != count || m.NiN() != len(expected)-count {
		t.Fatalf("counts: got %d/%d, expected %d/%d", m.ViN(), m.NiN(), count, len(expected)-count)
	}
	assertBitMapTail(t, m)
}

func assertBitMapTail(t *testing.T, m *BitMap) {
	t.Helper()
	if rem := m.Len() & 7; rem != 0 {
		last := m.Bytes()[len(m.Bytes())-1]
		if last&^byte((1<<rem)-1) != 0 {
			t.Fatalf("nonzero tail bits: %08b", last)
		}
	}
}

func cleanTestRange(length, start, stop int) (int, int) {
	if start < 0 {
		start = 0
	}
	if stop > length {
		stop = length
	}
	if stop < start || stop < 0 || start > length {
		return 0, 0
	}
	return start, stop
}
