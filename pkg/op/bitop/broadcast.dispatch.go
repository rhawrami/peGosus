//go:build !arm64

package bitop

// BroadcastU8 broadcasts `lit` to every element in `dst`.
func BroadcastU8(dst []byte, lit byte) {
	broadcastU8Impl(dst, lit)
}

// BroadcastI64 broadcasts `lit` to every element in `dst`.
func BroadcastI64(dst []int64, lit int64) {
	broadcastI64Impl(dst, lit)
}

// BroadcastI32 broadcasts `lit` to every element in `dst`.
func BroadcastI32(dst []int32, lit int32) {
	broadcastI32Impl(dst, lit)
}

// BroadcastF64 broadcasts `lit` to every element in `dst`.
func BroadcastF64(dst []float64, lit float64) {
	broadcastF64Impl(dst, lit)
}

// BroadcastF32 broadcasts `lit` to every element in `dst`.
func BroadcastF32(dst []float32, lit float32) {
	broadcastF32Impl(dst, lit)
}
