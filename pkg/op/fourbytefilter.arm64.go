//go:build arm64

package op

// GtI32 checks if `src` elements are greater than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func GtI32(src []int32, dst []byte, lit int32)

// LtI32 checks if `src` elements are less than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func LtI32(src []int32, dst []byte, lit int32)

// GeI32 checks if `src` elements are greater than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func GeI32(src []int32, dst []byte, lit int32)

// LeI32 checks if `src` elements are less than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func LeI32(src []int32, dst []byte, lit int32)

// EqI32 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func EqI32(src []int32, dst []byte, lit int32)

// GtF32 checks if `src` elements are greater than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func GtF32(src []float32, dst []byte, lit float32)

// LtF32 checks if `src` elements are less than `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func LtF32(src []float32, dst []byte, lit float32)

// GeF32 checks if `src` elements are greater than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func GeF32(src []float32, dst []byte, lit float32)

// LeF32 checks if `src` elements are less than or equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func LeF32(src []float32, dst []byte, lit float32)

// EqF32 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func EqF32(src []float32, dst []byte, lit float32)

// NeqI32 checks if `src` elements are not equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func NeqI32(src []int32, dst []byte, lit int32)

// NeqF32 checks if `src` elements are equal to `lit`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func NeqF32(src []float32, dst []byte, lit float32)

// BetI32 checks if `src` elements are between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func BetI32(src []int32, dst []byte, min int32, max int32)

// NBetI32 checks if `src` elements are not between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func NBetI32(src []int32, dst []byte, min int32, max int32)

// BetF32 checks if `src` elements are between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func BetF32(src []float32, dst []byte, min float32, max float32)

// NBetF32 checks if `src` elements are not between `min` and `max`, and places
// the bitpacked results in `dst` (bit is set for elements passing condition).
//
//go:noescape
func NBetF32(src []float32, dst []byte, min float32, max float32)
