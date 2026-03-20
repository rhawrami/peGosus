//go:build amd64

package op

//go:noescape
func maxI64(src, dst []int64)

//go:noescape
func minI64(src, dst []int64)

//go:noescape
func maxI32(src, dst []int32)

//go:noescape
func minI32(src, dst []int32)

//go:noescape
func maxF64(src, dst []float64)

//go:noescape
func minF64(src, dst []float64)

//go:noescape
func maxF32(src, dst []float32)

//go:noescape
func minF32(src, dst []float32)
