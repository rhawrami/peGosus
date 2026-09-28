package plan

import (
	"math"
	"testing"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func TestScopedOperatorStateReleasesAllReservations(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{8192}, []int{8192})
	key := store.MakeVector(a, 9, dtype.Int32T(), false)
	for i := range key.I32s() {
		key.I32s()[i] = int32(i % 3)
	}
	text := store.MakeStringVector(a, [][]byte{[]byte("alpha long string"), []byte("beta long string"), []byte("gamma long string"), []byte("alpha long string"), []byte("beta long string"), []byte("gamma long string"), []byte("alpha long string"), []byte("beta long string"), []byte("gamma long string")}, nil)
	batch := store.MakeBatch([]store.Vector{key, text})
	scope := mem.MakeAllocationScope(a, 1<<20)
	distinct := &distinctState{}
	distinct.setScope(scope)
	for row := range batch.Len() {
		if _, ok := distinct.add(a, batch, row, math.MaxInt64); !ok {
			t.Fatal("scoped DISTINCT failed")
		}
	}
	if scope.Live() == 0 {
		t.Fatal("scoped DISTINCT did not charge payload")
	}
	distinct.release()
	if scope.Live() != 0 {
		t.Fatalf("DISTINCT leaked %d bytes", scope.Live())
	}
	table := store.MakeTable([]*store.Batch{batch})
	scan := MakeScan(table, MakeSchema([]string{"key", "text"}, []dtype.Type{dtype.Int32T(), dtype.StringT()}))
	plan, err := MakePhysicalPlan(scan.Project(MakeColumn("key")).OrderBy(MakeOrderKey(MakeColumn("key"))))
	if err != nil {
		t.Fatal(err)
	}
	projected := batch.Project([]int{0})
	step := plan.steps[1]
	state := makeCompactSortState(step)
	if state == nil {
		t.Fatal("fixed-width projection did not select compact sorting")
	}
	state.scope = scope
	if !state.add(a, projected, step, math.MaxInt64) {
		t.Fatal("scoped sort failed")
	}
	output := state.finish(a, step)
	if output == nil || output.Len() != batch.Len() {
		t.Fatal("scoped sort output failed")
	}
	state.release()
	if scope.Live() != 0 {
		t.Fatalf("sort leaked %d bytes", scope.Live())
	}
	output.Release()
	plan.Release()
	plan, err = MakePhysicalPlan(scan.Project(MakeColumn("key")).OrderBy(MakeOrderKey(MakeColumn("key"))).Limit(2))
	if err != nil {
		t.Fatal(err)
	}
	step = plan.steps[1]
	top := makeCompactTopNState(step)
	if top == nil {
		t.Fatal("fixed-width TopN not selected")
	}
	top.scope, top.rows.scope = scope, scope
	if !top.add(a, projected, step, math.MaxInt64) {
		t.Fatal("scoped TopN failed")
	}
	output = top.finish(a, step)
	if output == nil || output.Len() != 2 {
		t.Fatal("scoped TopN output failed")
	}
	top.release()
	if scope.Live() != 0 {
		t.Fatalf("TopN leaked %d bytes", scope.Live())
	}
	output.Release()
	projected.Release()
	plan.Release()
	table.Release()
	batch.Release()
}
