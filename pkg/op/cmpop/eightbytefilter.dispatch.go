//go:build !arm64

package cmpop

// GtI64 checks if `src` elements are greater than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func GtI64(src []int64, dst []byte, lit int64) {
	gtI64Impl(src, dst, lit)
}

// LtI64 checks if `src` elements are less than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func LtI64(src []int64, dst []byte, lit int64) {
	ltI64Impl(src, dst, lit)
}

// GeI64 checks if `src` elements are greater than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func GeI64(src []int64, dst []byte, lit int64) {
	geI64Impl(src, dst, lit)
}

// LeI64 checks if `src` elements are less than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func LeI64(src []int64, dst []byte, lit int64) {
	leI64Impl(src, dst, lit)
}

// EqI64 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func EqI64(src []int64, dst []byte, lit int64) {
	eqI64Impl(src, dst, lit)
}

// GtF64 checks if `src` elements are greater than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func GtF64(src []float64, dst []byte, lit float64) {
	gtF64Impl(src, dst, lit)
}

// LtF64 checks if `src` elements are less than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func LtF64(src []float64, dst []byte, lit float64) {
	ltF64Impl(src, dst, lit)
}

// GeF64 checks if `src` elements are greater than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func GeF64(src []float64, dst []byte, lit float64) {
	geF64Impl(src, dst, lit)
}

// LeF64 checks if `src` elements are less than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func LeF64(src []float64, dst []byte, lit float64) {
	leF64Impl(src, dst, lit)
}

// EqF64 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func EqF64(src []float64, dst []byte, lit float64) {
	eqF64Impl(src, dst, lit)
}

// NeqI64 checks if `src` elements are not equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func NeqI64(src []int64, dst []byte, lit int64) {
	neqI64Impl(src, dst, lit)
}

// NeqF64 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func NeqF64(src []float64, dst []byte, lit float64) {
	neqF64Impl(src, dst, lit)
}

// BetI64 checks if `src` elements are between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func BetI64(src []int64, dst []byte, min int64, max int64) {
	betI64Impl(src, dst, min, max)
}

// NBetI64 checks if `src` elements are not between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func NBetI64(src []int64, dst []byte, min int64, max int64) {
	nBetI64Impl(src, dst, min, max)
}

// BetF64 checks if `src` elements are between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func BetF64(src []float64, dst []byte, min float64, max float64) {
	betF64Impl(src, dst, min, max)
}

// NBetF64 checks if `src` elements are not between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
func NBetF64(src []float64, dst []byte, min float64, max float64) {
	nBetF64Impl(src, dst, min, max)
}
