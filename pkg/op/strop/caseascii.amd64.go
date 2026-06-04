//go:build amd64

package strop

//go:noescape
func toUpperASCII(src []byte, dst []byte)

//go:noescape
func toLowerASCII(src []byte, dst []byte)
