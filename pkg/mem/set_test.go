package mem

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"unsafe"
)

func TestSetU8(t *testing.T) {
	const n int = 50
	values := make([]uint8, n)
	for i := 0; i < n; i++ {
		values[i] = uint8(rand.Uint32N(255))
	}

	const l int = 1_000
	for _, v := range values {
		t.Run(fmt.Sprintf("Value %d", v), func(t *testing.T) {
			s := make([]uint8, l)
			setU8(&s[0], uint64(l), v)

			for i, v2 := range s {
				if v2 != v {
					t.Errorf("On %d: got %d; expected %d", i, v2, v)
				}
			}
		})
	}
}

func TestSetU32(t *testing.T) {
	const n int = 50
	values := make([]uint32, n)
	for i := 0; i < n; i++ {
		values[i] = rand.Uint32()
	}

	const l int = 1_000
	for _, v := range values {
		t.Run(fmt.Sprintf("Value %d", v), func(t *testing.T) {
			s := make([]uint32, l)
			setU32((*byte)(unsafe.Pointer(&s[0])), uint64(l), v)

			for i, v2 := range s {
				if v2 != v {
					t.Errorf("On %d: got %d; expected %d", i, v2, v)
				}
			}
		})
	}
}

func TestSetU64(t *testing.T) {
	const n int = 50
	values := make([]uint64, n)
	for i := 0; i < n; i++ {
		values[i] = rand.Uint64()
	}

	const l int = 1_000
	for _, v := range values {
		t.Run(fmt.Sprintf("Value %d", v), func(t *testing.T) {
			s := make([]uint64, l)
			setU64((*byte)(unsafe.Pointer(&s[0])), uint64(l), v)

			for i, v2 := range s {
				if v2 != v {
					t.Errorf("On %d: got %d; expected %d", i, v2, v)
				}
			}
		})
	}
}
