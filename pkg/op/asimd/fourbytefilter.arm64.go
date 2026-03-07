//go:build arm64

package asimd

//go:noescape
func cmpGtI32VecI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpLtI32VecI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpGeI32VecI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpLeI32VecI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpEqI32VecI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpGtF32VecF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpLtF32VecF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpGeF32VecF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpLeF32VecF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpEqF32VecF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpNeqI32VecI32Lit(src []int32, dst []byte, lit int32)

//go:noescape
func cmpNeqF32VecF32Lit(src []float32, dst []byte, lit float32)

//go:noescape
func cmpBetI32VecI32Lit(src []int32, dst []byte, min int32, max int32)

//go:noescape
func cmpNBetI32VecI32Lit(src []int32, dst []byte, min int32, max int32)

//go:noescape
func cmpBetF32VecF32Lit(src []float32, dst []byte, min float32, max float32)

//go:noescape
func cmpNBetF32VecF32Lit(src []float32, dst []byte, min float32, max float32)
