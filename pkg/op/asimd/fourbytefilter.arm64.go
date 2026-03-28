//go:build arm64

package op

//go:noescape
func cmpGtI32(src []int32, dst []byte, lit int32)

//go:noescape
func cmpLtI32(src []int32, dst []byte, lit int32)

//go:noescape
func cmpGeI32(src []int32, dst []byte, lit int32)

//go:noescape
func cmpLeI32(src []int32, dst []byte, lit int32)

//go:noescape
func cmpEqI32(src []int32, dst []byte, lit int32)

//go:noescape
func cmpGtF32(src []float32, dst []byte, lit float32)

//go:noescape
func cmpLtF32(src []float32, dst []byte, lit float32)

//go:noescape
func cmpGeF32(src []float32, dst []byte, lit float32)

//go:noescape
func cmpLeF32(src []float32, dst []byte, lit float32)

//go:noescape
func cmpEqF32(src []float32, dst []byte, lit float32)

//go:noescape
func cmpNeqI32(src []int32, dst []byte, lit int32)

//go:noescape
func cmpNeqF32(src []float32, dst []byte, lit float32)

//go:noescape
func cmpBetI32(src []int32, dst []byte, min int32, max int32)

//go:noescape
func cmpNBetI32(src []int32, dst []byte, min int32, max int32)

//go:noescape
func cmpBetF32(src []float32, dst []byte, min float32, max float32)

//go:noescape
func cmpNBetF32(src []float32, dst []byte, min float32, max float32)
