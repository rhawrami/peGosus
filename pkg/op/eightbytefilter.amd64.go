//go:build amd64

package op

//go:noescape
func cmpGtI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpLtI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpGeI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpLeI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpEqI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpGtF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpLtF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpGeF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpLeF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpEqF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpNeqI64Lit(src []int64, dst []byte, lit int64)

//go:noescape
func cmpNeqF64Lit(src []float64, dst []byte, lit float64)

//go:noescape
func cmpBetI64Lit(src []int64, dst []byte, min int64, max int64)

//go:noescape
func cmpNBetI64Lit(src []int64, dst []byte, min int64, max int64)

//go:noescape
func cmpBetF64Lit(src []float64, dst []byte, min float64, max float64)

//go:noescape
func cmpNBetF64Lit(src []float64, dst []byte, min float64, max float64)
