package plan

import (
	"context"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestCompactStringShortDuplicateOrdering(t *testing.T) {
	rng := rand.New(rand.NewPCG(3159, 5197))
	for _, values := range [][]string{
		{"cust-12345678"},
		{"a", "a\x00", "a\x00\x00", ""},
		{strings.Repeat("x", 15), strings.Repeat("x", 15) + "a", strings.Repeat("x", 15) + "b"},
		{strings.Repeat("\x00", 15), "a\x00" + strings.Repeat("b", 13)},
	} {
		for _, descending := range []bool{false, true} {
			a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
			texts := make([][]byte, 4097)
			ids := store.MakeVector(a, len(texts), dtype.Int32T(), false)
			want := make([]int, len(texts))
			for i := range texts {
				texts[i] = []byte(values[rng.IntN(len(values))])
				ids.I32s()[i] = int32(i)
				want[i] = i
			}
			sort.SliceStable(want, func(i, j int) bool {
				l, r := string(texts[want[i]]), string(texts[want[j]])
				if descending {
					return l > r
				}
				return l < r
			})
			batch := store.MakeBatch([]store.Vector{store.MakeStringVector(a, texts, nil), ids})
			table := store.MakeTable([]*store.Batch{batch})
			order := MakeOrderKey(MakeColumn("txt"))
			if descending {
				order = order.Desc()
			}
			p, failure := MakePhysicalPlan(MakeScan(table, MakeSchema([]string{"txt", "id"}, []dtype.Type{dtype.StringT(), dtype.Int32T()})).OrderBy(order))
			if failure != nil {
				t.Fatal(failure)
			}
			calls := 0
			result := p.ExecuteWithOptions(context.Background(), a, ExecutionOptions{Workers: 1, MemoryBudget: 4 << 20}, func(out *store.Batch) bool {
				calls++
				if out.Len() != len(want) {
					t.Fatal("lost sorted rows")
				}
				for row, id := range want {
					if out.VectorAt(1).I32s()[row] != int32(id) || out.VectorAt(0).StringAt(row).View() != string(texts[id]) {
						t.Fatalf("descending=%t row=%d changed exact ordering or stability", descending, row)
					}
				}
				return true
			})
			if result.Code() != ExecutionCompleted || calls != 1 {
				t.Fatalf("sort result=%v failure=%v calls=%d", result.Code(), result.Err(), calls)
			}
			p.Release()
			table.Release()
			batch.Release()
			if u := a.Usage(); u.GeneralUsed != 0 || u.ScratchUsed != 0 {
				t.Fatalf("short string sort leaked %+v", u)
			}
		}
	}
}
