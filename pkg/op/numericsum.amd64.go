//go:build amd64

package op

//go:noescape
func sumI64(src, dst []int64)

//go:noescape
func sumI32(src []int32, dst []int64)

//go:noescape
func sumF64(src, dst []float64)

//go:noescape
func sumF32(src []float32, dst []float64)

//go:noescape
func sumI64WithValidity(src, dst []int64, validity []byte)

//go:noescape
func sumI32WithValidity(src []int32, dst []int64, validity []byte)

//go:noescape
func sumF64WithValidity(src, dst []float64, validity []byte)

//go:noescape
func sumF32WithValidity(src []float32, dst []float64, validity []byte)
