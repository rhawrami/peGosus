//go:build arm64

package op

// BroadcastU8 broadcasts `lit` to every element in `dst`.
//
//go:noescape
func BroadcastU8(dst []byte, lit byte)

// BroadcastI64 broadcasts `lit` to every element in `dst`.
//
//go:noescape
func BroadcastI64(dst []int64, lit int64)

// BroadcastI32 broadcasts `lit` to every element in `dst`.
//
//go:noescape
func BroadcastI32(dst []int32, lit int32)

// BroadcastF64 broadcasts `lit` to every element in `dst`.
//
//go:noescape
func BroadcastF64(dst []float64, lit float64)

// BroadcastF32 broadcasts `lit` to every element in `dst`.
//
//go:noescape
func BroadcastF32(dst []float32, lit float32)
