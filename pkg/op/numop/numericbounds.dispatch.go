//go:build !arm64

package numop

// MaxI64 finds the maximum signed element in `src`, placing it in `dst[0]`.
func MaxI64(src, dst []int64) { maxI64Impl(src, dst) }

// MinI64 finds the minimum signed element in `src`, placing it in `dst[0]`.
func MinI64(src, dst []int64) { minI64Impl(src, dst) }

// MaxI32 finds the maximum signed element in `src`, placing it in `dst[0]`.
func MaxI32(src, dst []int32) { maxI32Impl(src, dst) }

// MinI32 finds the minimum signed element in `src`, placing it in `dst[0]`.
func MinI32(src, dst []int32) { minI32Impl(src, dst) }

// MaxF64 finds the maximum  element in `src` (ignoring NaN elements), placing it in `dst[0]`.
func MaxF64(src, dst []float64) { maxF64Impl(src, dst) }

// MinF64 finds the minimum  element in `src` (ignoring NaN elements), placing it in `dst[0]`.
func MinF64(src, dst []float64) { minF64Impl(src, dst) }

// MaxF32 finds the maximum  element in `src` (ignoring NaN elements), placing it in `dst[0]`.
func MaxF32(src, dst []float32) { maxF32Impl(src, dst) }

// MinF32 finds the minimum  element in `src` (ignoring NaN elements), placing it in `dst[0]`.
func MinF32(src, dst []float32) { minF32Impl(src, dst) }

// MinMaxI64 finds the minimum and maximum signed elements in `src`, placing the "min" in `dst[0]`
// and the "max" in `dst[1]`.
func MinMaxI64(src, dst []int64) { minMaxI64Impl(src, dst) }

// MinMaxI32 finds the minimum and maximum signed elements in `src`, placing the "min" in `dst[0]`
// and the "max" in `dst[1]`.
func MinMaxI32(src, dst []int32) { minMaxI32Impl(src, dst) }

// MinMaxF64 finds the minimum and maximum elements in `src` (ignoring NaN elements), placing
// the "min" in `dst[0]` and the "max" in `dst[1]`.
func MinMaxF64(src, dst []float64) { minMaxF64Impl(src, dst) }

// MinMaxF32 finds the minimum and maximum elements in `src` (ignoring NaN elements), placing
// the "min" in `dst[0]` and the "max" in `dst[1]`.
func MinMaxF32(src, dst []float32) { minMaxF32Impl(src, dst) }

// MaxI64WithValidity finds the maximum signed element in `src`, placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
func MaxI64WithValidity(src, dst []int64, validity []byte) {
	maxI64WithValidityImpl(src, dst, validity)
}

// MinI64WithValidity finds the minimum signed element in `src`, placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
func MinI64WithValidity(src, dst []int64, validity []byte) {
	minI64WithValidityImpl(src, dst, validity)
}

// MaxI32WithValidity finds the maximum signed element in `src`, placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
func MaxI32WithValidity(src, dst []int32, validity []byte) {
	maxI32WithValidityImpl(src, dst, validity)
}

// MinI32WithValidity finds the minimum signed element in `src`, placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
func MinI32WithValidity(src, dst []int32, validity []byte) {
	minI32WithValidityImpl(src, dst, validity)
}

// MaxF64WithValidity finds the maximum  element in `src` (ignoring NaN elements), placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
func MaxF64WithValidity(src, dst []float64, validity []byte) {
	maxF64WithValidityImpl(src, dst, validity)
}

// MinF64WithValidity finds the minimum  element in `src` (ignoring NaN elements), placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
func MinF64WithValidity(src, dst []float64, validity []byte) {
	minF64WithValidityImpl(src, dst, validity)
}

// MaxF32WithValidity finds the maximum  element in `src` (ignoring NaN elements), placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
func MaxF32WithValidity(src, dst []float32, validity []byte) {
	maxF32WithValidityImpl(src, dst, validity)
}

// MinF32WithValidity finds the minimum  element in `src` (ignoring NaN elements), placing it in `dst[0]`. `validity`
// represents a validity bitmap, where only elements corresponding to set bits will be included in the
// calculation.
func MinF32WithValidity(src, dst []float32, validity []byte) {
	minF32WithValidityImpl(src, dst, validity)
}

// MinMaxI64WithValidity finds the minimum and maximum signed elements in `src`, placing the "min" in `dst[0]`
// and the "max" in `dst[1]`. `validity` represents a validity bitmap, where only elements
// corresponding to set bits will be included in the calculation.
func MinMaxI64WithValidity(src, dst []int64, validity []byte) {
	minMaxI64WithValidityImpl(src, dst, validity)
}

// MinMaxI32WithValidity finds the minimum and maximum signed elements in `src`, placing the "min" in `dst[0]`
// and the "max" in `dst[1]`. `validity` represents a validity bitmap, where only elements
// corresponding to set bits will be included in the calculation.
func MinMaxI32WithValidity(src, dst []int32, validity []byte) {
	minMaxI32WithValidityImpl(src, dst, validity)
}

// MinMaxF64WithValidity finds the minimum and maximum elements in `src` (ignoring NaN elements), placing
// the "min" in `dst[0]` and the "max" in `dst[1]`. `validity` represents a validity bitmap,
// where only elements corresponding to set bits will be included in the calculation.
func MinMaxF64WithValidity(src, dst []float64, validity []byte) {
	minMaxF64WithValidityImpl(src, dst, validity)
}

// MinMaxF32WithValidity finds the minimum and maximum elements in `src` (ignoring NaN elements), placing
// the "min" in `dst[0]` and the "max" in `dst[1]`. `validity` represents a validity bitmap,
// where only elements corresponding to set bits will be included in the calculation.
func MinMaxF32WithValidity(src, dst []float32, validity []byte) {
	minMaxF32WithValidityImpl(src, dst, validity)
}
