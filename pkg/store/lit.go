package store

import (
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
)

// LitVec represents a vector of constants.
type LitVec struct {
	dType dtype.Type     // data type
	val   unsafe.Pointer // pointer to value
}

// GetLitVecVal returns the value held by the literal vector.
func GetLitVecVal[T any](v *LitVec) T { return *(*T)(v.val) }
