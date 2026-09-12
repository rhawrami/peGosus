//go:build !arm64

package numop

func SumI64(src, dst []int64) { sumI64Impl(src, dst) }

func SumI32(src []int32, dst []int64) { sumI32Impl(src, dst) }

func SumF64(src, dst []float64) { sumF64Impl(src, dst) }

func SumF32(src []float32, dst []float64) { sumF32Impl(src, dst) }

func SumI64WithValidity(src, dst []int64, validity []byte) {
	sumI64WithValidityImpl(src, dst, validity)
}

func SumI32WithValidity(src []int32, dst []int64, validity []byte) {
	sumI32WithValidityImpl(src, dst, validity)
}

func SumF64WithValidity(src, dst []float64, validity []byte) {
	sumF64WithValidityImpl(src, dst, validity)
}

func SumF32WithValidity(src []float32, dst []float64, validity []byte) {
	sumF32WithValidityImpl(src, dst, validity)
}
