//go:build !arm64

package cmpop

// GtI32 checks if `src` elements are greater than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func GtI32(src []int32, dst []byte, lit int32) {
	gtI32Impl(src, dst, lit)
}

// LtI32 checks if `src` elements are less than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func LtI32(src []int32, dst []byte, lit int32) {
	ltI32Impl(src, dst, lit)
}

// GeI32 checks if `src` elements are greater than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func GeI32(src []int32, dst []byte, lit int32) {
	geI32Impl(src, dst, lit)
}

// LeI32 checks if `src` elements are less than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func LeI32(src []int32, dst []byte, lit int32) {
	leI32Impl(src, dst, lit)
}

// EqI32 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func EqI32(src []int32, dst []byte, lit int32) {
	eqI32Impl(src, dst, lit)
}

// GtF32 checks if `src` elements are greater than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func GtF32(src []float32, dst []byte, lit float32) {
	gtF32Impl(src, dst, lit)
}

// LtF32 checks if `src` elements are less than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func LtF32(src []float32, dst []byte, lit float32) {
	ltF32Impl(src, dst, lit)
}

// GeF32 checks if `src` elements are greater than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func GeF32(src []float32, dst []byte, lit float32) {
	geF32Impl(src, dst, lit)
}

// LeF32 checks if `src` elements are less than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func LeF32(src []float32, dst []byte, lit float32) {
	leF32Impl(src, dst, lit)
}

// EqF32 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func EqF32(src []float32, dst []byte, lit float32) {
	eqF32Impl(src, dst, lit)
}

// NeqI32 checks if `src` elements are not equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func NeqI32(src []int32, dst []byte, lit int32) {
	neqI32Impl(src, dst, lit)
}

// NeqF32 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func NeqF32(src []float32, dst []byte, lit float32) {
	neqF32Impl(src, dst, lit)
}

// BetI32 checks if `src` elements are between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func BetI32(src []int32, dst []byte, min int32, max int32) {
	betI32Impl(src, dst, min, max)
}

// NBetI32 checks if `src` elements are not between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func NBetI32(src []int32, dst []byte, min int32, max int32) {
	nBetI32Impl(src, dst, min, max)
}

// BetF32 checks if `src` elements are between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func BetF32(src []float32, dst []byte, min float32, max float32) {
	betF32Impl(src, dst, min, max)
}

// NBetF32 checks if `src` elements are not between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func NBetF32(src []float32, dst []byte, min float32, max float32) {
	nBetF32Impl(src, dst, min, max)
}
