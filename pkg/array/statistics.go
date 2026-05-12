package array

import "math"

// StatU64T represents the unsigned 64-bit integer value of any given statistic.
type StatU64T uint64

// I64ToStatU64T converts `x` to a StatU64T.
func I64ToStatU64T(x int64) StatU64T { return StatU64T(x) }

// I64ToStatU64T converts `x` to a StatU64T.
func I32ToStatU64T(x int32) StatU64T { return StatU64T(x) }

// I64ToStatU64T converts `x` to a StatU64T.
func F64ToStatU64T(x float64) StatU64T { return StatU64T(math.Float64bits(x)) }

// I64ToStatU64T converts `x` to a StatU64T.
func F32ToStatU64T(x float32) StatU64T { return StatU64T(math.Float32bits(x)) }

// ToI64T converts `x` to an int64 value.
func (x StatU64T) ToI64T() int64 { return int64(x) }

// ToI32T converts `x` to an int32 value.
func (x StatU64T) ToI32T() int32 { return int32(x) }

// ToF64T converts `x` to an float64 value.
func (x StatU64T) ToF64T() float64 { return math.Float64frombits(uint64(x)) }

// ToF32T converts `x` to an float32 value.
func (x StatU64T) ToF32T() float32 { return math.Float32frombits(uint32(x)) }

// CachedStatID represents the possible statistics tracked by
// an array's cached statistics.
type CachedStatID int

const (
	// maximum value; defined on numeric arrays;
	// int64 for integer arrays; float64 for floating-point arrays
	StatMAX CachedStatID = iota
	// minimum value; defined on numeric arrays;
	// int64 for integer arrays; float64 for floating-point arrays
	StatMIN
	// horizontal sum; defined on numeric arrays;
	// int64 for integer arrays; float64 for floating-point arrays
	StatSUM
	// arithmetic mean; defined on numeric arrays;
	// float64 for all numeric array types
	StatMEAN
	// standard deviation; defined on numeric arrays;
	// float64 for all numeric array types
	StatSTDDEV
	// number of unique values; defined on all arrays;
	StatUNIQ
	// maximum length; defined on string arrays
	StatMAXLEN
	// minimum length; defined on string arrays
	StatMINLEN
	// array only contains ASCII elements; defined on string arrays
	StatISASCII
	// array is sorted; defined on all arrays
	StatISSORTED
)

// CachedStatistics contain a handful of aggregate statistics
// and metadata of a given array and its qualities; given the various
// types that an array can be, CachedStatistics track data for all type arrays;
// this will mean that an Int64T array will track IsASCII in its statistics, even
// though it's meaningless.
type CachedStatistics struct {
	stats    [10]StatU64T // statistics, with offsets corresponding to ID values
	validity uint16       // bitmask representing validity of tracked statistics
}

// ClearAll clears the validity of all statistics.
func (cs *CachedStatistics) ClearAll() {
	cs.validity = 0
}

// ClearValidity clears the validity of a given statistic.
func (cs *CachedStatistics) ClearValidity(id CachedStatID) {
	cs.validity &= ^uint16(1 << id)
}

// Set sets the value of a given statistic to `v`, and its validity to true.
func (cs *CachedStatistics) Set(id CachedStatID, v StatU64T) {
	cs.stats[id] = v
	cs.SetValidity(id)
}

// SetValidity sets the validity of a given statistic.
func (cs *CachedStatistics) SetValidity(id CachedStatID) {
	cs.validity |= uint16(1 << id)
}

// CheckValidity checks the validity of a given statistic.
func (cs *CachedStatistics) CheckValidity(id CachedStatID) bool {
	return (cs.validity>>id)&1 == 1
}
