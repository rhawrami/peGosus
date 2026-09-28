package plan

import (
	"fmt"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestKeyIndexDifferentialAndBudget(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	h := &keyIndex{}
	keys := &keyArena{}
	defer func() {
		h.release()
		keys.release()
	}()
	inputs := [][]byte{nil, []byte("a"), []byte("a\x00"), []byte("a\x00\x00"), []byte("long string value")}
	for i := range 300 {
		inputs = append(inputs, []byte(fmt.Sprintf("key-%05d", i)))
	}
	for i, key := range inputs {
		if _, found := h.lookup(key, keys); found {
			t.Fatalf("key %q already exists", key)
		}
		if !h.put(a, key, i, 1<<20) {
			t.Fatalf("insert %d failed", i)
		}
		if !keys.append(a, key, 1<<20) {
			t.Fatalf("arena append %d failed", i)
		}
		if got, found := h.lookup(key, keys); !found || got != i {
			t.Fatalf("key %q: got %d/%t, want %d", key, got, found, i)
		}
	}
	for i, key := range inputs {
		if got, found := h.lookup(key, keys); !found || got != i {
			t.Fatalf("rehash lost key %q", key)
		}
	}
	if _, found := h.lookup([]byte("not found"), keys); found {
		t.Fatal("found absent key")
	}

	small := &keyIndex{}
	defer small.release()
	if small.put(a, []byte("x"), 0, 255) {
		t.Fatal("created a table beyond the budget")
	}
	for i := range 12 {
		if !small.put(a, []byte{byte(i)}, i, 700) {
			t.Fatalf("small insertion %d failed", i)
		}
	}
	if small.put(a, []byte("overflow"), 12, 700) || small.bytes() != 256 {
		t.Fatal("resized table above peak budget")
	}
	if !small.put(a, []byte("now fits"), 12, 768) {
		t.Fatal("valid resize failed")
	}
}
