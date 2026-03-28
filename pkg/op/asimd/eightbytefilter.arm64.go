//go:build arm64

package op

//go:noescape
func cmpGtI64(src []int64, dst []byte, lit int64)

//go:noescape
func cmpLtI64(src []int64, dst []byte, lit int64)

//go:noescape
func cmpGeI64(src []int64, dst []byte, lit int64)

//go:noescape
func cmpLeI64(src []int64, dst []byte, lit int64)

//go:noescape
func cmpEqI64(src []int64, dst []byte, lit int64)

//go:noescape
func cmpGtF64(src []float64, dst []byte, lit float64)

//go:noescape
func cmpLtF64(src []float64, dst []byte, lit float64)

//go:noescape
func cmpGeF64(src []float64, dst []byte, lit float64)

//go:noescape
func cmpLeF64(src []float64, dst []byte, lit float64)

//go:noescape
func cmpEqF64(src []float64, dst []byte, lit float64)

//go:noescape
func cmpNeqI64(src []int64, dst []byte, lit int64)

//go:noescape
func cmpNeqF64(src []float64, dst []byte, lit float64)

//go:noescape
func cmpBetI64(src []int64, dst []byte, min int64, max int64)

//go:noescape
func cmpNBetI64(src []int64, dst []byte, min int64, max int64)

//go:noescape
func cmpBetF64(src []float64, dst []byte, min float64, max float64)

//go:noescape
func cmpNBetF64(src []float64, dst []byte, min float64, max float64)
