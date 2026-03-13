//go:build amd64

package op

//go:noescape
func sqrtF64Vec(src, dst []float64)

//go:noescape
func sqrtF32Vec(src, dst []float32)

//go:noescape
func sqF64Vec(src, dst []float64)

//go:noescape
func sqF32Vec(src, dst []float32)

//go:noescape
func absF64Vec(src, dst []float64)

//go:noescape
func absF32Vec(src, dst []float32)

//go:noescape
func negF64Vec(src, dst []float64)

//go:noescape
func negF32Vec(src, dst []float32)

//go:noescape
func recipF64Vec(src, dst []float64)

//go:noescape
func recipF32Vec(src, dst []float32)
