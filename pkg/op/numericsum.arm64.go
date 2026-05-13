//go:build arm64

package op

//go:noescape
func SumI64(src, dst []int64)

//go:noescape
func SumI32(src []int32, dst []int64)

//go:noescape
func SumF64(src, dst []float64)

//go:noescape
func SumF32(src []float32, dst []float64)

//go:noescape
func SumI64WithValidity(src, dst []int64, validity []byte)

//go:noescape
func SumI32WithValidity(src []int32, dst []int64, validity []byte)

//go:noescape
func SumF64WithValidity(src, dst []float64, validity []byte)

//go:noescape
func SumF32WithValidity(src []float32, dst []float64, validity []byte)
