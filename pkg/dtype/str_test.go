package dtype

import (
	"bytes"
	"runtime"
	"strconv"
	"testing"
	"unsafe"
)

func TestStringLayout(t *testing.T) {
	var s String
	if got := unsafe.Sizeof(s); got != StringSize {
		t.Fatalf("size: got %d, expected %d", got, StringSize)
	}
	if got, want := unsafe.Alignof(s), unsafe.Alignof(uint64(0)); got != want {
		t.Fatalf("alignment: got %d, expected %d", got, want)
	}
	if got := unsafe.Offsetof(s.prefix); got != 4 {
		t.Fatalf("prefix offset: got %d, expected 4", got)
	}
	if got := unsafe.Offsetof(s.tail); got != 8 {
		t.Fatalf("tail offset: got %d, expected 8", got)
	}
}

func TestMakeString(t *testing.T) {
	lengths := []int{0, 1, 3, 4, 5, 11, 12, 13, 31, 255}
	for _, length := range lengths {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			src := make([]byte, length)
			for i := range src {
				src[i] = byte('a' + i%26)
			}
			want := string(src)
			s := MakeString(src)

			if got := s.Len(); got != length {
				t.Fatalf("Len: got %d, expected %d", got, length)
			}
			if got := s.IsInline(); got != (length <= StringInlineSize) {
				t.Fatalf("IsInline: got %t", got)
			}
			if got := s.View(); got != want {
				t.Fatalf("View: got %q, expected %q", got, want)
			}

			var prefix [4]byte
			copy(prefix[:], src)
			if got := s.Prefix(); got != prefix {
				t.Fatalf("Prefix: got %v, expected %v", got, prefix)
			}
			runtime.KeepAlive(src)
		})
	}
}

func TestStringInlineCopiesSource(t *testing.T) {
	src := []byte("hello world!")
	s := MakeString(src)
	copy(src, bytes.Repeat([]byte{'x'}, len(src)))
	if got := s.View(); got != "hello world!" {
		t.Fatalf("View: got %q, expected %q", got, "hello world!")
	}
}

func TestStringLongBorrowsSource(t *testing.T) {
	src := []byte("this string is longer than twelve bytes")
	s := MakeString(src)
	src[5] = 'S'
	runtime.GC()

	if got, want := s.View(), string(src); got != want {
		t.Fatalf("View: got %q, expected %q", got, want)
	}
	runtime.KeepAlive(src)
}

func TestStringDescriptorCopy(t *testing.T) {
	src := []byte("another string longer than twelve bytes")
	s := MakeString(src)
	copyOfString := s

	if got, want := copyOfString.View(), string(src); got != want {
		t.Fatalf("View: got %q, expected %q", got, want)
	}
	runtime.KeepAlive(src)
}
