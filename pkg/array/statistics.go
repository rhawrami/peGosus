package array

// CachedStatID represents the possible statistics tracked by
// an array's cached statistics.
type CachedStatID int

const (
	// maximum value; defined on numeric arrays;
	// int64 for integer arrays; float64 for floating-point arrays
	MAX CachedStatID = iota
	// minimum value; defined on numeric arrays;
	// int64 for integer arrays; float64 for floating-point arrays
	MIN
	// horizontal sum; defined on numeric arrays;
	// int64 for integer arrays; float64 for floating-point arrays
	SUM
	// arithmetic mean; defined on numeric arrays;
	// float64 for all numeric array types
	MEAN
	// standard deviation; defined on numeric arrays;
	// float64 for all numeric array types
	STDDEV
	// number of unique values; defined on all arrays;
	UNIQ
	// maximum length; defined on string arrays
	MAXLEN
	// minimum length; defined on string arrays
	MINLEN
	// array only contains ASCII elements; defined on string arrays
	ISASCII
	// array is sorted; defined on all arrays
	ISSORTED
)

// CachedStatistics contain a handful of aggregate statistics
// and metadata of a given array and its qualities; given the various
// types that an array can be, CachedStatistics track data for all type arrays;
// this will mean that an Int64T array will track IsASCII in its statistics, even
// though it's meaningless.
type CachedStatistics struct {
	MaxI64   int64   // bit 0 (defined on integer arrays)
	MaxF64   float64 // bit 0 (defined on floating-point arrays)
	MinI64   int64   // bit 1 (defined on integer arrays)
	MinF64   float64 // bit 1 (defined on floating-point arrays)
	SumI64   int64   // bit 2 (defined on integer arrays)
	SumF64   float64 // bit 2 (defined on floating-point arrays)
	Mean     float64 // bit 3 (defined on numeric arrays)
	StdDev   float64 // bit 4 (defined on numeric arrays)
	Uniq     uint64  // bit 5 (defined on all arrays)
	MaxLen   uint64  // bit 6 (defined on string arrays)
	MinLen   uint64  // bit 7 (defined on string arrays)
	IsASCII  bool    // bit 8 (defined on string arrays)
	IsSorted bool    // bit 9 (defined on all arrays)
	Validity uint16  // bitmask representing validity of tracked statistics
}

// ClearAll clears the validity of all statistics.
func (cs *CachedStatistics) ClearAll() {
	cs.Validity = 0
}

// Clear clears the validity of a given statistic.
func (cs *CachedStatistics) Clear(id CachedStatID) {
	switch id {
	case MAX:
		cs.Validity &= ^uint16(1)
	case MIN:
		cs.Validity &= ^uint16(1 << 1)
	case SUM:
		cs.Validity &= ^uint16(1 << 2)
	case MEAN:
		cs.Validity &= ^uint16(1 << 3)
	case STDDEV:
		cs.Validity &= ^uint16(1 << 4)
	case UNIQ:
		cs.Validity &= ^uint16(1 << 5)
	case MAXLEN:
		cs.Validity &= ^uint16(1 << 6)
	case MINLEN:
		cs.Validity &= ^uint16(1 << 7)
	case ISASCII:
		cs.Validity &= ^uint16(1 << 8)
	case ISSORTED:
		cs.Validity &= ^uint16(1 << 9)
	default:
		//
	}
}

// Set sets the validity of a given statistic.
func (cs *CachedStatistics) Set(id CachedStatID) {
	switch id {
	case MAX:
		cs.Validity |= uint16(1)
	case MIN:
		cs.Validity |= uint16(1 << 1)
	case SUM:
		cs.Validity |= uint16(1 << 2)
	case MEAN:
		cs.Validity |= uint16(1 << 3)
	case STDDEV:
		cs.Validity |= uint16(1 << 4)
	case UNIQ:
		cs.Validity |= uint16(1 << 5)
	case MAXLEN:
		cs.Validity |= uint16(1 << 6)
	case MINLEN:
		cs.Validity |= uint16(1 << 7)
	case ISASCII:
		cs.Validity |= uint16(1 << 8)
	case ISSORTED:
		cs.Validity |= uint16(1 << 9)
	default:
		//
	}
}

// Check checks the validity of a given statistic.
func (cs *CachedStatistics) Check(id CachedStatID) bool {
	switch id {
	case MAX:
		return cs.Validity&1 == 1
	case MIN:
		return (cs.Validity>>1)&1 == 1
	case SUM:
		return (cs.Validity>>2)&1 == 1
	case MEAN:
		return (cs.Validity>>3)&1 == 1
	case STDDEV:
		return (cs.Validity>>4)&1 == 1
	case UNIQ:
		return (cs.Validity>>5)&1 == 1
	case MAXLEN:
		return (cs.Validity>>6)&1 == 1
	case MINLEN:
		return (cs.Validity>>7)&1 == 1
	case ISASCII:
		return (cs.Validity>>8)&1 == 1
	case ISSORTED:
		return (cs.Validity>>9)&1 == 1
	default:
		return false
	}
}
