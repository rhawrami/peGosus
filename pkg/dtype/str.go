package dtype

import "unsafe"

const strMaxInlineLen = 12

// String is a 16-byte object representing a string, taken after
// "German strings" from Umbra.
// https://cedardb.com/blog/german_strings/
//
// For strings with byte length longer than 12 bytes, `epilogue` points
// to the starting position of the string.
type String struct {
	length   uint32         // byte length
	prefix   [4]byte        // first 4 bytes (padded with '\0')
	epilogue unsafe.Pointer // length > 12 ? ptr : remaining bytes
}

// StrIsInlined returns true if string's length is less than or equal
// to 12 bytes.
func StrIsInlined(s String) bool { return s.length < 13 }

// StrPrefixAsBytes returns the string's prefix as a 4-byte array.
func StrPrefixAsBytes(s String) [4]byte { return s.prefix }

// StrPrefixAsU32T returns the string's prefix as an unsigned 32-bit integer.
func StrPrefixAsU32T(s String) uint32 { return *(*uint32)(unsafe.Pointer(&s.prefix[0])) }

// StrEpilogueAsBytes returns the string's remaining bytes as an 8-byte array.
func StrEpilogueAsBytes(s String) [8]byte { return *(*[8]byte)(s.epilogue) }

// StrEpilogueAsU64T returns the string's remaining bytes as an unsigned
// 64-bit integer; this function shouldn't be called on strings with length
// greater than 12 bytes.
func StrEpilogueAsU64T(s String) uint64 { return *(*uint64)(s.epilogue) }

// StrViewShort returns the string as a string; this function is only
// defined for strings with length less than or equal to 12 bytes.
func StrViewShort(s String) string { return unsafe.String(&s.prefix[0], s.length) }

// StrViewLong returns the string as a string; this function is only defined
// for strings with length greater than 12 bytes.
func StrViewLong(s String) string {
	return unsafe.String((*byte)(s.epilogue), s.length)
}

// StrView returns the string as a string.
func StrView(s String) string {
	if s.length <= strMaxInlineLen {
		return StrViewShort(s)
	}
	return StrViewLong(s)
}
