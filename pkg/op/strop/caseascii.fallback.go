//go:build !arm64

package strop

// ToUpperASCII converts all lowercase single-byte characters to
// uppercase; only works on ASCII characters.
func ToUpperASCII(src []byte, dst []byte) {
	toUpperASCIIImpl(src, dst)
}

// ToLowerASCII converts all uppercase single-byte characters to
// lowercase; only works on ASCII characters.
func ToLowerASCII(src []byte, dst []byte) {
	toLowerASCIIImpl(src, dst)
}
