package plan

import (
	"fmt"
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestPackedRowsGrowAndOwnStrings(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	schema := MakeSchema([]string{"name", "id", "flag", "score"}, []dtype.Type{dtype.StringT(), dtype.Int32T(), dtype.BoolT(), dtype.Float64T()})
	state := &packedRows{}
	for start := 0; start < 257; start += 17 {
		length := min(17, 257-start)
		strings := make([][]byte, length)
		valid := make([]bool, length)
		id := store.MakeVector(a, length, dtype.Int32T(), true)
		flag := store.MakeVector(a, length, dtype.BoolT(), true)
		score := store.MakeVector(a, length, dtype.Float64T(), true)
		for row := range length {
			i := start + row
			switch i % 4 {
			case 0:
				strings[row] = []byte("")
			case 1:
				strings[row] = []byte("a\x00")
			default:
				strings[row] = []byte(fmt.Sprintf("long string value %04d", i))
			}
			valid[row] = i%13 != 0
			id.I32s()[row] = int32(i)
			if !valid[row] {
				id.Validity().Clear(row)
				flag.Validity().Clear(row)
				score.Validity().Clear(row)
			}
			if i%3 == 0 {
				flag.Bools()[row>>3] |= 1 << (row & 7)
			}
			score.F64s()[row] = float64(i) / 10
		}
		str := store.MakeStringVector(a, strings, valid)
		batch := store.MakeBatch([]store.Vector{str, id, flag, score})
		for row := range length {
			if !state.append(a, batch, row, math.MaxInt64) {
				t.Fatalf("append row %d failed", start+row)
			}
		}
		batch.Release()
	}
	if state.length != 257 {
		t.Fatalf("got %d rows", state.length)
	}
	output := state.makeBatch(a, schema)
	state.release()
	if output == nil {
		t.Fatal("could not materialize packed rows")
	}
	defer output.Release()
	for i := range 257 {
		if i%13 == 0 {
			for column := range output.NVectors() {
				if output.VectorAt(column).Validity().IsSet(i) {
					t.Fatalf("column %d row %d should be NULL", column, i)
				}
			}
			continue
		}
		want := ""
		switch i % 4 {
		case 1:
			want = "a\x00"
		case 2, 3:
			want = fmt.Sprintf("long string value %04d", i)
		}
		if output.VectorAt(0).Strings()[i].View() != want || output.VectorAt(1).I32s()[i] != int32(i) || (output.VectorAt(2).Bools()[i>>3]&(1<<(i&7)) != 0) != (i%3 == 0) || output.VectorAt(3).F64s()[i] != float64(i)/10 {
			t.Fatalf("row %d was not retained correctly", i)
		}
	}
	limit := &packedRows{}
	v := store.MakeVector(a, 1, dtype.Int32T(), false)
	b := store.MakeBatch([]store.Vector{v})
	if limit.append(a, b, 0, 1) {
		t.Fatal("allocated rows beyond the budget")
	}
	limit.release()
	b.Release()
}
