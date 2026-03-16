//go:build arm64

package op

//go:noescape
func cmpGtI64VecI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpLtI64VecI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpGeI64VecI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpLeI64VecI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpEqI64VecI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpGtF64VecF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpLtF64VecF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpGeF64VecF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpLeF64VecF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpEqF64VecF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpNeqI64VecI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpNeqF64VecF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpBetI64VecI64Lit(src []int64, dst []byte, min int64, max int64)

//go:noescape
func cmpNBetI64VecI64Lit(src []int64, dst []byte, min int64, max int64)

//go:noescape
func cmpBetF64VecF64Lit(src []float64, dst []byte, min float64, max float64)

//go:noescape
func cmpNBetF64VecF64Lit(src []float64, dst []byte, min float64, max float64)
