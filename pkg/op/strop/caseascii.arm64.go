//go:build arm64

package strop

// ToUpperASCII converts all lowercase single-byte characters to
// uppercase; only works on ASCII characters.
//
//go:noescape
func ToUpperASCII(src []byte, dst []byte)

// ToLowerASCII converts all uppercase single-byte characters to
// lowercase; only works on ASCII characters.
//
//go:noescape
func ToLowerASCII(src []byte, dst []byte)
