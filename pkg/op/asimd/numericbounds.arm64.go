//go:build arm64

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

//go:noescape
func minmaxI64(src, dst []int64)

//go:noescape
func minmaxI32(src, dst []int32)

//go:noescape
func minmaxF64(src, dst []float64)

//go:noescape
func minmaxF32(src, dst []float32)

//go:noescape
func maxI64WithValidity(src, dst []int64, validity []byte)

//go:noescape
func minI64WithValidity(src, dst []int64, validity []byte)

//go:noescape
func maxI32WithValidity(src, dst []int32, validity []byte)

//go:noescape
func minI32WithValidity(src, dst []int32, validity []byte)

//go:noescape
func maxF64WithValidity(src, dst []float64, validity []byte)

//go:noescape
func minF64WithValidity(src, dst []float64, validity []byte)

//go:noescape
func maxF32WithValidity(src, dst []float32, validity []byte)

//go:noescape
func minF32WithValidity(src, dst []float32, validity []byte)

//go:noescape
func minmaxI64WithValidity(src, dst []int64, validity []byte)

//go:noescape
func minmaxI32WithValidity(src, dst []int32, validity []byte)

//go:noescape
func minmaxF64WithValidity(src, dst []float64, validity []byte)

//go:noescape
func minmaxF32WithValidity(src, dst []float32, validity []byte)
