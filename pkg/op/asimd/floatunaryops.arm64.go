//go:build arm64

package op

//go:noescape
func sqrtF64(src, dst []float64)

//go:noescape
func sqrtF32(src, dst []float32)

//go:noescape
func sqF64(src, dst []float64)

//go:noescape
func sqF32(src, dst []float32)

//go:noescape
func absF64(src, dst []float64)

//go:noescape
func absF32(src, dst []float32)

//go:noescape
func negF64(src, dst []float64)

//go:noescape
func negF32(src, dst []float32)

//go:noescape
func recipF64(src, dst []float64)

//go:noescape
func recipF32(src, dst []float32)
