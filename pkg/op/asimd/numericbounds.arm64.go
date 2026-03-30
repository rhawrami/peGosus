//go:build arm64

package op

// MaxI64 finds the maximum signed element in `src`, placing it in `dst[0]`.
//
//go:noescape
func MaxI64(src, dst []int64)

// MinI64 finds the minimum signed element in `src`, placing it in `dst[0]`.
//
//go:noescape
func MinI64(src, dst []int64)

// MaxI32 finds the maximum signed element in `src`, placing it in `dst[0]`.
//
//go:noescape
func MaxI32(src, dst []int32)

// MinI32 finds the minimum signed element in `src`, placing it in `dst[0]`.
//
//go:noescape
func MinI32(src, dst []int32)

// MaxF64 finds the maximum  element in `src` (ignoring NaN elements), placing it in `dst[0]`.
//
//go:noescape
func MaxF64(src, dst []float64)

// MinF64 finds the minimum  element in `src` (ignoring NaN elements), placing it in `dst[0]`.
//
//go:noescape
func MinF64(src, dst []float64)

// MaxF32 finds the maximum  element in `src` (ignoring NaN elements), placing it in `dst[0]`.
//
//go:noescape
func MaxF32(src, dst []float32)

// MinF32 finds the minimum  element in `src` (ignoring NaN elements), placing it in `dst[0]`.
//
//go:noescape
func MinF32(src, dst []float32)

// MinMaxI64 finds the minimum and maximum signed elements in `src`, placing the "min" in `dst[0]`
// and the "max" in `dst[1]`.
//
//go:noescape
func MinMaxI64(src, dst []int64)

// MinMaxI32 finds the minimum and maximum signed elements in `src`, placing the "min" in `dst[0]`
// and the "max" in `dst[1]`.
//
//go:noescape
func MinMaxI32(src, dst []int32)

// MinMaxF64 finds the minimum and maximum elements in `src` (ignoring NaN elements), placing
// the "min" in `dst[0]` and the "max" in `dst[1]`.
//
//go:noescape
func MinMaxF64(src, dst []float64)

// MinMaxF32 finds the minimum and maximum elements in `src` (ignoring NaN elements), placing
// the "min" in `dst[0]` and the "max" in `dst[1]`.
//
//go:noescape
func MinMaxF32(src, dst []float32)

// MaxI64WithValidity finds the maximum signed element in `src`, placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
//
//go:noescape
func MaxI64WithValidity(src, dst []int64, validity []byte)

// MinI64WithValidity finds the minimum signed element in `src`, placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
//
//go:noescape
func MinI64WithValidity(src, dst []int64, validity []byte)

// MaxI32WithValidity finds the maximum signed element in `src`, placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
//
//go:noescape
func MaxI32WithValidity(src, dst []int32, validity []byte)

// MinI32WithValidity finds the minimum signed element in `src`, placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
//
//go:noescape
func MinI32WithValidity(src, dst []int32, validity []byte)

// MaxF64WithValidity finds the maximum  element in `src` (ignoring NaN elements), placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
//
//go:noescape
func MaxF64WithValidity(src, dst []float64, validity []byte)

// MinF64WithValidity finds the minimum  element in `src` (ignoring NaN elements), placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
//
//go:noescape
func MinF64WithValidity(src, dst []float64, validity []byte)

// MaxF32WithValidity finds the maximum  element in `src` (ignoring NaN elements), placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
//
//go:noescape
func MaxF32WithValidity(src, dst []float32, validity []byte)

// MinF32WithValidity finds the minimum  element in `src` (ignoring NaN elements), placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
//
//go:noescape
func MinF32WithValidity(src, dst []float32, validity []byte)

// MinMaxI64WithValidity finds the minimum and maximum signed elements in `src`, placing the "min" in `dst[0]`
// and the "max" in `dst[1]`. `validity` represents a validity bitmap, where only elements
// corresponding to set bits will be included in the calculation.
//
//go:noescape
func MinMaxI64WithValidity(src, dst []int64, validity []byte)

// MinMaxI32WithValidity finds the minimum and maximum signed elements in `src`, placing the "min" in `dst[0]`
// and the "max" in `dst[1]`. `validity` represents a validity bitmap, where only elements
// corresponding to set bits will be included in the calculation.
//
//go:noescape
func MinMaxI32WithValidity(src, dst []int32, validity []byte)

// MinMaxF64WithValidity finds the minimum and maximum elements in `src` (ignoring NaN elements), placing
// the "min" in `dst[0]` and the "max" in `dst[1]`. `validity` represents a validity bitmap,
// where only elements corresponding to set bits will be included in the calculation.
//
//go:noescape
func MinMaxF64WithValidity(src, dst []float64, validity []byte)

// MinMaxF32WithValidity finds the minimum and maximum elements in `src` (ignoring NaN elements), placing
// the "min" in `dst[0]` and the "max" in `dst[1]`. `validity` represents a validity bitmap,
// where only elements corresponding to set bits will be included in the calculation.
//
//go:noescape
func MinMaxF32WithValidity(src, dst []float32, validity []byte)
