package dtype

import (
	"math"
	"runtime"
	"unsafe"
)

const (
	// StringInlineSize is the maximum number of bytes stored in a descriptor.
	StringInlineSize = 12
	// StringSize is the byte width of a String descriptor.
	StringSize = 16
)

// MakeString returns a German-string descriptor for `src`. Values of at most
// StringInlineSize bytes are copied into the descriptor. Longer values borrow
// `src`; retaining the descriptor does not retain that storage, so its owner
// must remain live and unchanged while the descriptor is used.
func MakeString(src []byte) String {
	if uint64(len(src)) > math.MaxUint32 {
		panic("MakeString: input exceeds maximum string length")
	}

	s := String{length: uint32(len(src))}
	copy(s.prefix[:], src)
	if len(src) <= StringInlineSize {
		if len(src) > len(s.prefix) {
			copy(s.tailBytes(), src[len(s.prefix):])
		}
		return s
	}

	s.tail = uint64(uintptr(unsafe.Pointer(unsafe.SliceData(src))))
	runtime.KeepAlive(src)
	return s
}

// String is a 16-byte German-string descriptor. Values of at most 12 bytes
// are inline; longer values contain a four-byte prefix and a borrowed pointer.
type String struct {
	length uint32
	prefix [4]byte
	tail   uint64
}

// Len returns the string's byte length.
func (s *String) Len() int { return int(s.length) }

// IsInline returns whether the complete value is stored in the descriptor.
func (s *String) IsInline() bool { return s.length <= StringInlineSize }

// Prefix returns the first four bytes, padded with zero bytes when necessary.
func (s *String) Prefix() [4]byte { return s.prefix }

// View returns a borrowed Go string. The descriptor and any referenced long
// payload must remain live and unchanged while the returned string is used.
func (s *String) View() string {
	if s.length == 0 {
		return ""
	}
	if s.IsInline() {
		return unsafe.String(&s.prefix[0], s.length)
	}
	return unsafe.String(s.pointer(), s.length)
}

func (s *String) tailBytes() []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&s.tail)), 8)
}

//go:nocheckptr
func (s *String) pointer() *byte {
	return (*byte)(unsafe.Pointer(uintptr(s.tail)))
}
