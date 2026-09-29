//go:build amd64

package numop

import "golang.org/x/sys/cpu"

var (
	addF64LitImpl = addF64Lit
	subF64LitImpl = subF64Lit
	mulF64LitImpl = mulF64Lit
	divF64LitImpl = divF64Lit
	addF32LitImpl = addF32Lit
	subF32LitImpl = subF32Lit
	mulF32LitImpl = mulF32Lit
	divF32LitImpl = divF32Lit
	addF64VecImpl = addF64Vec
	subF64VecImpl = subF64Vec
	mulF64VecImpl = mulF64Vec
	divF64VecImpl = divF64Vec
	addF32VecImpl = addF32Vec
	subF32VecImpl = subF32Vec
	mulF32VecImpl = mulF32Vec
	divF32VecImpl = divF32Vec

	sqrtF64Impl  = sqrtF64
	sqF64Impl    = sqF64
	absF64Impl   = absF64
	negF64Impl   = negF64
	recipF64Impl = recipF64
	sqrtF32Impl  = sqrtF32
	sqF32Impl    = sqF32
	absF32Impl   = absF32
	negF32Impl   = negF32
	recipF32Impl = recipF32

	addI64LitImpl = addI64Lit
	subI64LitImpl = subI64Lit
	mulI64LitImpl = mulI64Lit
	divI64LitImpl = divI64Lit
	addI32LitImpl = addI32Lit
	subI32LitImpl = subI32Lit
	mulI32LitImpl = mulI32Lit
	divI32LitImpl = divI32Lit
	addI64VecImpl = addI64Vec
	subI64VecImpl = subI64Vec
	mulI64VecImpl = mulI64Vec
	divI64VecImpl = divI64Vec
	addI32VecImpl = addI32Vec
	subI32VecImpl = subI32Vec
	mulI32VecImpl = mulI32Vec
	divI32VecImpl = divI32Vec

	sqrtI64Impl  = sqrtI64
	sqI64Impl    = sqI64
	absI64Impl   = absI64
	negI64Impl   = negI64
	recipI64Impl = recipI64
	sqrtI32Impl  = sqrtI32
	sqI32Impl    = sqI32
	absI32Impl   = absI32
	negI32Impl   = negI32
	recipI32Impl = recipI32

	maxI64Impl    = maxI64
	minI64Impl    = minI64
	maxI32Impl    = maxI32
	minI32Impl    = minI32
	maxF64Impl    = maxF64
	minF64Impl    = minF64
	maxF32Impl    = maxF32
	minF32Impl    = minF32
	minMaxI64Impl = minMaxI64
	minMaxI32Impl = minMaxI32
	minMaxF64Impl = minMaxF64
	minMaxF32Impl = minMaxF32

	maxI64WithValidityImpl    = maxI64WithValidity
	minI64WithValidityImpl    = minI64WithValidity
	maxI32WithValidityImpl    = maxI32WithValidity
	minI32WithValidityImpl    = minI32WithValidity
	maxF64WithValidityImpl    = maxF64WithValidity
	minF64WithValidityImpl    = minF64WithValidity
	maxF32WithValidityImpl    = maxF32WithValidity
	minF32WithValidityImpl    = minF32WithValidity
	minMaxI64WithValidityImpl = minMaxI64WithValidity
	minMaxI32WithValidityImpl = minMaxI32WithValidity
	minMaxF64WithValidityImpl = minMaxF64WithValidity
	minMaxF32WithValidityImpl = minMaxF32WithValidity

	castI64ToF64Impl = castI64ToF64
	castI32ToF32Impl = castI32ToF32
	castF64ToI64Impl = castF64ToI64
	castF32ToI32Impl = castF32ToI32
	castF32ToF64Impl = castF32ToF64
	castI32ToI64Impl = castI32ToI64
	castI32ToF64Impl = castI32ToF64
	castF32ToI64Impl = castF32ToI64
	castI64ToF32Impl = castI64ToF32
	castF64ToI32Impl = castF64ToI32

	clipF64WithF64BoundsImpl = clipF64WithF64BoundsChecked
	clipF32WithF32BoundsImpl = clipF32WithF32BoundsChecked
	clipI32WithI32BoundsImpl = clipI32WithI32Bounds
	clipI64WithI64BoundsImpl = clipI64WithI64Bounds
	clipI64WithF64BoundsImpl = clipI64WithF64BoundsChecked
	clipI32WithF32BoundsImpl = clipI32WithF32BoundsChecked

	sumI64Impl             = sumI64
	sumI32Impl             = sumI32
	sumF64Impl             = sumF64
	sumF32Impl             = sumF32
	sumI64WithValidityImpl = sumI64WithValidity
	sumI32WithValidityImpl = sumI32WithValidity
	sumF64WithValidityImpl = sumF64WithValidity
	sumF32WithValidityImpl = sumF32WithValidity
)

func init() {
	if cpu.X86.HasAVX2 {
		return
	}

	addF64LitImpl = addF64LitFallback
	subF64LitImpl = subF64LitFallback
	mulF64LitImpl = mulF64LitFallback
	divF64LitImpl = divF64LitFallback
	addF32LitImpl = addF32LitFallback
	subF32LitImpl = subF32LitFallback
	mulF32LitImpl = mulF32LitFallback
	divF32LitImpl = divF32LitFallback
	addF64VecImpl = addF64VecFallback
	subF64VecImpl = subF64VecFallback
	mulF64VecImpl = mulF64VecFallback
	divF64VecImpl = divF64VecFallback
	addF32VecImpl = addF32VecFallback
	subF32VecImpl = subF32VecFallback
	mulF32VecImpl = mulF32VecFallback
	divF32VecImpl = divF32VecFallback

	sqrtF64Impl = sqrtF64Fallback
	sqF64Impl = sqF64Fallback
	absF64Impl = absF64Fallback
	negF64Impl = negF64Fallback
	recipF64Impl = recipF64Fallback
	sqrtF32Impl = sqrtF32Fallback
	sqF32Impl = sqF32Fallback
	absF32Impl = absF32Fallback
	negF32Impl = negF32Fallback
	recipF32Impl = recipF32Fallback

	addI64LitImpl = addI64LitFallback
	subI64LitImpl = subI64LitFallback
	mulI64LitImpl = mulI64LitFallback
	divI64LitImpl = divI64LitFallback
	addI32LitImpl = addI32LitFallback
	subI32LitImpl = subI32LitFallback
	mulI32LitImpl = mulI32LitFallback
	divI32LitImpl = divI32LitFallback
	addI64VecImpl = addI64VecFallback
	subI64VecImpl = subI64VecFallback
	mulI64VecImpl = mulI64VecFallback
	divI64VecImpl = divI64VecFallback
	addI32VecImpl = addI32VecFallback
	subI32VecImpl = subI32VecFallback
	mulI32VecImpl = mulI32VecFallback
	divI32VecImpl = divI32VecFallback

	sqrtI64Impl = sqrtI64Fallback
	sqI64Impl = sqI64Fallback
	absI64Impl = absI64Fallback
	negI64Impl = negI64Fallback
	recipI64Impl = recipI64Fallback
	sqrtI32Impl = sqrtI32Fallback
	sqI32Impl = sqI32Fallback
	absI32Impl = absI32Fallback
	negI32Impl = negI32Fallback
	recipI32Impl = recipI32Fallback

	maxI64Impl = maxI64Fallback
	minI64Impl = minI64Fallback
	maxI32Impl = maxI32Fallback
	minI32Impl = minI32Fallback
	maxF64Impl = maxF64Fallback
	minF64Impl = minF64Fallback
	maxF32Impl = maxF32Fallback
	minF32Impl = minF32Fallback
	minMaxI64Impl = minMaxI64Fallback
	minMaxI32Impl = minMaxI32Fallback
	minMaxF64Impl = minMaxF64Fallback
	minMaxF32Impl = minMaxF32Fallback

	maxI64WithValidityImpl = maxI64WithValidityFallback
	minI64WithValidityImpl = minI64WithValidityFallback
	maxI32WithValidityImpl = maxI32WithValidityFallback
	minI32WithValidityImpl = minI32WithValidityFallback
	maxF64WithValidityImpl = maxF64WithValidityFallback
	minF64WithValidityImpl = minF64WithValidityFallback
	maxF32WithValidityImpl = maxF32WithValidityFallback
	minF32WithValidityImpl = minF32WithValidityFallback
	minMaxI64WithValidityImpl = minMaxI64WithValidityFallback
	minMaxI32WithValidityImpl = minMaxI32WithValidityFallback
	minMaxF64WithValidityImpl = minMaxF64WithValidityFallback
	minMaxF32WithValidityImpl = minMaxF32WithValidityFallback

	castI64ToF64Impl = castI64ToF64Fallback
	castI32ToF32Impl = castI32ToF32Fallback
	castF64ToI64Impl = castF64ToI64Fallback
	castF32ToI32Impl = castF32ToI32Fallback
	castF32ToF64Impl = castF32ToF64Fallback
	castI32ToI64Impl = castI32ToI64Fallback
	castI32ToF64Impl = castI32ToF64Fallback
	castF32ToI64Impl = castF32ToI64Fallback
	castI64ToF32Impl = castI64ToF32Fallback
	castF64ToI32Impl = castF64ToI32Fallback

	clipF64WithF64BoundsImpl = clipF64WithF64BoundsFallback
	clipF32WithF32BoundsImpl = clipF32WithF32BoundsFallback
	clipI32WithI32BoundsImpl = clipI32WithI32BoundsFallback
	clipI64WithI64BoundsImpl = clipI64WithI64BoundsFallback
	clipI64WithF64BoundsImpl = clipI64WithF64BoundsFallback
	clipI32WithF32BoundsImpl = clipI32WithF32BoundsFallback

	sumI64Impl = sumI64Fallback
	sumI32Impl = sumI32Fallback
	sumF64Impl = sumF64Fallback
	sumF32Impl = sumF32Fallback
	sumI64WithValidityImpl = sumI64WithValidityFallback
	sumI32WithValidityImpl = sumI32WithValidityFallback
	sumF64WithValidityImpl = sumF64WithValidityFallback
	sumF32WithValidityImpl = sumF32WithValidityFallback
}
