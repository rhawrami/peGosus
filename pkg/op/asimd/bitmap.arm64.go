//go:build arm64

package asimd

//go:noescape
func bitmapANDRetPopCount(src1, src2, dst []byte) uint64

//go:noescape
func bitmapORRetPopCount(src1, src2, dst []byte) uint64

//go:noescape
func bitmapPopCount(src []byte) uint64
