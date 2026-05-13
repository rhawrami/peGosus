//go:build amd64

package op

//go:noescape
func cmpGtI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpLtI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpGeI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpLeI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpEqI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpGtF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpLtF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpGeF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpLeF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpEqF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpNeqI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpNeqF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpBetI32Lit(src []int32, dst []byte, min int32, max int32)

//go:noescape
func cmpNBetI32Lit(src []int32, dst []byte, min int32, max int32)

//go:noescape
func cmpBetF32Lit(src []float32, dst []byte, min float32, max float32)

//go:noescape
func cmpNBetF32Lit(src []float32, dst []byte, min float32, max float32)
