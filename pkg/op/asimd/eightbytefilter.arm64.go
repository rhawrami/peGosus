//go:build arm64

package op

// GtI64 checks if `src` elements are greater than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func GtI64(src []int64, dst []byte, lit int64)

// LtI64 checks if `src` elements are less than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func LtI64(src []int64, dst []byte, lit int64)

// GeI64 checks if `src` elements are greater than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func GeI64(src []int64, dst []byte, lit int64)

// LeI64 checks if `src` elements are less than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func LeI64(src []int64, dst []byte, lit int64)

// EqI64 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func EqI64(src []int64, dst []byte, lit int64)

// GtF64 checks if `src` elements are greater than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func GtF64(src []float64, dst []byte, lit float64)

// LtF64 checks if `src` elements are less than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func LtF64(src []float64, dst []byte, lit float64)

// GeF64 checks if `src` elements are greater than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func GeF64(src []float64, dst []byte, lit float64)

// LeF64 checks if `src` elements are less than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func LeF64(src []float64, dst []byte, lit float64)

// EqF64 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func EqF64(src []float64, dst []byte, lit float64)

// NeqI64 checks if `src` elements are not equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func NeqI64(src []int64, dst []byte, lit int64)

// NeqF64 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func NeqF64(src []float64, dst []byte, lit float64)

// BetI64 checks if `src` elements are between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func BetI64(src []int64, dst []byte, min int64, max int64)

// NBetI64 checks if `src` elements are not between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func NBetI64(src []int64, dst []byte, min int64, max int64)

// BetF64 checks if `src` elements are between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func BetF64(src []float64, dst []byte, min float64, max float64)

// NBetF64 checks if `src` elements are not between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func NBetF64(src []float64, dst []byte, min float64, max float64)
