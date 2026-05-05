package mem

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"unsafe"
)

func TestCalcOffset(t *testing.T) {
	const l int = 100

	// byte slice (element size is 1)
	a := make([]byte, l)
	a0 := &a[0]
	a50 := &a[50]
	o := calcOffset(a0, a50)
	oReal := 50
	if o != oReal {
		t.Errorf("got %d; expected %d", o, oReal)
	}

	// uint32 slice (element size is 4)
	b := make([]uint32, l)
	b20 := &b[20]
	b24 := &b[24]
	o = calcOffset((*byte)(unsafe.Pointer(b20)), (*byte)(unsafe.Pointer(b24)))
	oReal = 16
	if o != oReal {
		t.Errorf("got %d; expected %d", o, oReal)
	}

	// ensure panic on `base` > `addr`
	defer func() {
		if r := recover(); r == nil {
			t.Error("did not panic when base > addr")
		}
	}()
	calcOffset(a50, a0)
}

func TestIncPtr(t *testing.T) {
	const l int = 100
	// increment by 4 places
	a := make([]byte, l)
	a5 := &a[5]
	a9 := &a[9]
	p := incPtr(a5, 4)
	if p != a9 {
		t.Errorf("got %p; expected %p", a9, p)
	}

	// ensure panic on negative `l`
	defer func() {
		if r := recover(); r == nil {
			t.Error("did not panic on negative l")
		}
	}()
	incPtr(a5, -1)
}

func TestDecPtr(t *testing.T) {
	const l int = 100
	// increment by 4 places
	a := make([]byte, l)
	a26 := &a[26]
	a20 := &a[20]
	p := decPtr(a26, 6)
	if p != a20 {
		t.Errorf("got %p; expected %p", a20, p)
	}

	// ensure panic on negative `l`
	defer func() {
		if r := recover(); r == nil {
			t.Error("did not panic on negative l")
		}
	}()
	decPtr(a26, -1)
}

func TestIsAligned(t *testing.T) {
	const l int = 100

	// guaranteed to be 4 byte aligned
	a := make([]uint32, l)
	b := isAligned((*byte)(unsafe.Pointer(&a[0])), 4)
	if !b {
		t.Error("uint32 slice is aligned to 4 bytes, but fn found not aligned")
	}

	// guaranteed to be 8 byte aligned
	c := make([]uint64, l)
	d := isAligned((*byte)(unsafe.Pointer(&c[0])), 8)
	if !d {
		t.Error("uint64 slice is aligned to 4 bytes, but fn found not aligned")
	}
}

func TestMakeAlignedSlice(t *testing.T) {
	const n int = 50
	testSizes := make([]int, n)
	for i := 0; i < n; i++ {
		testSizes[i] = rand.IntN(1_000_000)
	}

	for _, s := range testSizes {
		t.Run(fmt.Sprintf("Size %d", s), func(t *testing.T) {
			a := makeAlignedSlice(s)
			if !isAligned(&a[0], alignSize) {
				t.Errorf("with size request %d, got unaligned slice", s)
			}
		})
	}
}
