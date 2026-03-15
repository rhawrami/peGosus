//go:build arm64

package op

//go:noescape
func broadcastB(dst []byte, lit byte)

//go:noescape
func broadcastI64(dst []int64, lit int64)

//go:noescape
func broadcastI32(dst []int32, lit int32)

//go:noescape
func broadcastF64(dst []float64, lit float64)

//go:noescape
func broadcastF32(dst []float32, lit float32)
