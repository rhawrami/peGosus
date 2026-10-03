package plan

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/rhawrami/peGosus/pkg/mem"
)

func TestKeyControlMasksDifferential(t *testing.T) {
	rng := rand.New(rand.NewPCG(981, 716))
	for trial := range 10000 {
		controls := make([]byte, keyControlGroup+31)
		for i := range controls {
			controls[i] = byte(rng.Uint32())
		}
		for fingerprint := range 256 {
			window := controls[trial%32 : trial%32+keyControlGroup]
			var candidates, empties uint16
			for i, value := range window {
				if value == byte(fingerprint) {
					candidates |= 1 << i
				}
				if value == 0 {
					empties |= 1 << i
				}
			}
			for _, implementation := range []struct {
				name string
				mask func([]byte, byte) (uint16, uint16)
			}{{"scalar", keyControlMasksScalar}, {"dispatch", keyControlMasks}} {
				got, empty := implementation.mask(window, byte(fingerprint))
				if got != candidates || empty != empties {
					t.Fatalf("trial=%d fingerprint=%d %s=%x/%x want=%x/%x", trial, fingerprint, implementation.name, got, empty, candidates, empties)
				}
			}
		}
	}
}

func TestKeyIndexControlsGrowthAndFallback(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	scope := mem.MakeAllocationScope(a, 8<<20)
	h := keyIndex{scope: scope, tryControls: true}
	keys := keyArena{scope: scope}
	var encoded [][]byte
	for row := range 3000 {
		key := []byte(fmt.Sprintf("binary \x00\xff prefix key-%d", row))
		if !keys.append(a, key, 8<<20) || !h.put(a, key, row, 8<<20) {
			t.Fatalf("insert %d", row)
		}
		encoded = append(encoded, key)
	}
	if h.controls == nil {
		t.Fatal("larger index has no controls")
	}
	for row, key := range encoded {
		if got, found := h.lookup(key, &keys); !found || got != row {
			t.Fatalf("row=%d got=%d/%t", row, got, found)
		}
	}
	if _, found := h.lookup([]byte("absent"), &keys); found {
		t.Fatal("found absent key")
	}
	controls := h.controls.AsBytes()
	if !slices.Equal(controls[:keyControlGroup], controls[len(controls)-keyControlGroup:]) {
		t.Fatal("wrapping controls not mirrored")
	}
	before := scope.Live()
	h.controls.Dec()
	h.controls = nil
	if scope.Live() >= before {
		t.Fatal("optional cache not charged")
	}
	h.makeControls(a, h.bytes())
	if h.controls != nil || scope.Exhausted() {
		t.Fatal("declined cache changed exhaustion")
	}
	for row, key := range encoded {
		if got, found := h.lookup(key, &keys); !found || got != row {
			t.Fatalf("fallback row=%d got=%d/%t", row, got, found)
		}
	}
	h.release()
	keys.release()
	if scope.Live() != 0 {
		t.Fatalf("live=%d", scope.Live())
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestKeyIndexControlCollisionsWrapAndEmptyStops(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	h := keyIndex{tryControls: true}
	keys := keyArena{}
	for row := range 769 {
		key := []byte(fmt.Sprintf("collision key-%d", row))
		if !keys.append(a, key, 8<<20) || !h.put(a, key, row, 8<<20) {
			t.Fatal("insert")
		}
	}
	if h.controls == nil {
		t.Fatal("no controls")
	}
	slots := h.slots.AsU64T()
	clear(slots)
	clear(h.controls.AsBytes())
	wanted := keys.key(768)
	hash := h.hash(wanted)
	capacity := len(slots) / 2
	start := int(hash & uint64(capacity-1))
	// Many equal fingerprints and full hashes force comparisons across groups and wrap.
	for offset := range 65 {
		at := (start + offset) & (capacity - 1)
		id := offset
		if offset == 64 {
			id = 768
		}
		slots[2*at], slots[2*at+1] = hash, uint64(id+1)
		h.setControl(at, byte(hash>>57)+1)
	}
	if got, found := h.lookup(wanted, &keys); !found || got != 768 {
		t.Fatalf("collision=%d/%t", got, found)
	}
	// A matching key after a hole is unreachable under linear probing.
	h.controls.AsBytes()[start] = 0
	if start < keyControlGroup {
		h.controls.AsBytes()[capacity+start] = 0
	}
	if _, found := h.lookup(wanted, &keys); found {
		t.Fatal("crossed first empty control")
	}
	h.release()
	keys.release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestKeyIndexStringControlsNullAndBinary(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	h := keyIndex{stringKeys: true, tryControls: true}
	keys := keyArena{}
	texts := []string{"", "a", "a\x00", "a\x00\xff", "\xff\xfe"}
	for row := range 2000 {
		texts = append(texts, fmt.Sprintf("long common binary prefix \x00\xff %d", row))
	}
	for id, text := range texts {
		key := make([]byte, 9+len(text))
		key[0] = 1
		binary.LittleEndian.PutUint64(key[1:], uint64(len(text)))
		copy(key[9:], text)
		if !keys.append(a, key, 8<<20) || !h.put(a, key, id, 8<<20) {
			t.Fatal("string insertion")
		}
	}
	if !keys.append(a, []byte{0}, 8<<20) || !h.put(a, []byte{0}, len(texts), 8<<20) {
		t.Fatal("null insertion")
	}
	if h.controls == nil {
		t.Fatal("string controls missing")
	}
	for id, text := range texts {
		if got, found := h.lookupString(text, false, &keys); !found || got != id {
			t.Fatalf("string=%q got=%d/%t want=%d", text, got, found, id)
		}
		if _, found := h.lookupString(text+"absent", false, &keys); found {
			t.Fatalf("absent string=%q found", text)
		}
	}
	if got, found := h.lookupString("ignored null payload", true, &keys); !found || got != len(texts) {
		t.Fatalf("null got=%d/%t", got, found)
	}
	h.release()
	keys.release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}

func TestKeyIndexDefaultControlsDisabled(t *testing.T) {
	a := mem.MakeAllocatorWithProfiles([]int{1 << 20}, []int{1 << 20})
	h := keyIndex{}
	keys := keyArena{}
	for row := range 2000 {
		key := []byte(fmt.Sprintf("default key-%d", row))
		if !keys.append(a, key, 8<<20) || !h.put(a, key, row, 8<<20) {
			t.Fatal("insert")
		}
	}
	if h.controls != nil {
		t.Fatal("experimental controls enabled by default")
	}
	if got, found := h.lookup([]byte("default key-1999"), &keys); !found || got != 1999 {
		t.Fatalf("default lookup=%d/%t", got, found)
	}
	h.release()
	keys.release()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("leaked %+v", usage)
	}
}
