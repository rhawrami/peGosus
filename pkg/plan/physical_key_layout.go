package plan

import (
	"math"
	"math/bits"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

type keyMode uint8

const (
	keyOrder keyMode = iota
	keyGroup
	keyJoin
)

type packedKeyStats struct {
	parts []keyPartStats
}

type keyPartStats struct {
	typeValue dtype.Type
	min       int64
	max       int64
	maxLen    int
	hasValue  bool
	hasNull   bool
}

type packedKeyPart struct {
	stats      keyPartStats
	valueBits  int
	lengthBits int
	nullBits   int
	descending bool
	nullsFirst bool
}

type packedKeyLayout struct {
	parts []packedKeyPart
	width int
	mode  keyMode
}

func makePackedKeyStats(types []dtype.Type) packedKeyStats {
	parts := make([]keyPartStats, len(types))
	for i, t := range types {
		parts[i].typeValue = t
	}
	return packedKeyStats{parts: parts}
}

func (s *packedKeyStats) observe(a *mem.Allocator, vectors []store.Vector, selection *store.RowSelection) bool {
	if len(vectors) != len(s.parts) {
		return false
	}
	length := -1
	for i := range vectors {
		if !vectors[i].Type().Equal(s.parts[i].typeValue) {
			return false
		}
		if length < 0 {
			length = vectors[i].Len()
		} else if vectors[i].Len() != length {
			return false
		}
	}
	if length < 0 {
		return true
	}
	if selection != nil && selection.Len() != length {
		return false
	}
	mask := selection.MakeBitMapTemp(a)
	if selection != nil && mask == nil {
		return false
	}
	if mask != nil {
		defer mask.Release()
	}
	for row := range length {
		if mask != nil && !mask.IsSet(row) {
			continue
		}
		for i := range vectors {
			vector := &vectors[i]
			part := &s.parts[i]
			if vector.Validity() != nil && !vector.Validity().IsSet(row) {
				part.hasNull = true
				continue
			}
			first := !part.hasValue
			part.hasValue = true
			switch vector.TypeID() {
			case dtype.INT32T, dtype.DATET:
				value := int64(vector.I32s()[row])
				if first || value < part.min {
					part.min = value
				}
				if first || value > part.max {
					part.max = value
				}
			case dtype.INT64T, dtype.TIMESTAMPTZT:
				value := vector.I64s()[row]
				if first || value < part.min {
					part.min = value
				}
				if first || value > part.max {
					part.max = value
				}
			case dtype.STRT:
				if n := vector.Strings()[row].Len(); n > part.maxLen {
					part.maxLen = n
				}
			case dtype.FLOAT32T, dtype.FLOAT64T, dtype.BOOLT:
			default:
				return false
			}
		}
	}
	return true
}

func (s packedKeyStats) layout(mode keyMode, descending, nullsFirst []bool) (packedKeyLayout, bool) {
	if len(s.parts) == 0 || mode == keyOrder && (len(descending) != len(s.parts) || len(nullsFirst) != len(s.parts)) {
		return packedKeyLayout{}, false
	}
	layout := packedKeyLayout{parts: make([]packedKeyPart, len(s.parts)), mode: mode}
	for i, stats := range s.parts {
		part := packedKeyPart{stats: stats}
		if mode == keyOrder {
			part.descending, part.nullsFirst = descending[i], nullsFirst[i]
		}
		switch stats.typeValue.ID() {
		case dtype.INT32T, dtype.INT64T, dtype.DATET, dtype.TIMESTAMPTZT:
			if stats.hasValue {
				part.valueBits = bits.Len64(uint64(stats.max) - uint64(stats.min))
			}
		case dtype.FLOAT32T:
			part.valueBits = 32
		case dtype.FLOAT64T:
			part.valueBits = 64
		case dtype.BOOLT:
			part.valueBits = 1
		case dtype.STRT:
			if stats.maxLen > 7 {
				return packedKeyLayout{}, false
			}
			part.lengthBits = bits.Len(uint(stats.maxLen))
			part.valueBits = stats.maxLen * 8
		default:
			return packedKeyLayout{}, false
		}
		if stats.hasNull && mode != keyJoin {
			part.nullBits = 1
		}
		width := part.valueBits + part.lengthBits + part.nullBits
		if width > 64-layout.width {
			return packedKeyLayout{}, false
		}
		layout.width += width
		layout.parts[i] = part
	}
	return layout, true
}

func (l packedKeyLayout) encode(vectors []store.Vector, row int) (uint64, bool) {
	if len(vectors) != len(l.parts) {
		return 0, false
	}
	var packed uint64
	for i, part := range l.parts {
		vector := &vectors[i]
		if !vector.Type().Equal(part.stats.typeValue) || row < 0 || row >= vector.Len() {
			return 0, false
		}
		isNull := vector.Validity() != nil && !vector.Validity().IsSet(row)
		if isNull && (!part.stats.hasNull || l.mode == keyJoin) {
			return 0, false
		}
		var component uint64
		if !isNull {
			switch vector.TypeID() {
			case dtype.INT32T, dtype.DATET, dtype.INT64T, dtype.TIMESTAMPTZT:
				value := int64(0)
				if vector.TypeID() == dtype.INT32T || vector.TypeID() == dtype.DATET {
					value = int64(vector.I32s()[row])
				} else {
					value = vector.I64s()[row]
				}
				if !part.stats.hasValue || value < part.stats.min || value > part.stats.max {
					return 0, false
				}
				component = uint64(value) - uint64(part.stats.min)
			case dtype.FLOAT32T:
				value := vector.F32s()[row]
				if l.mode == keyJoin && math.IsNaN(float64(value)) {
					return 0, false
				}
				bits32 := math.Float32bits(value)
				if value == 0 {
					bits32 = 0
				}
				if math.IsNaN(float64(value)) {
					component = math.MaxUint32
				} else if bits32&0x80000000 != 0 {
					component = uint64(^bits32)
				} else {
					component = uint64(bits32 ^ 0x80000000)
				}
			case dtype.FLOAT64T:
				value := vector.F64s()[row]
				if l.mode == keyJoin && math.IsNaN(value) {
					return 0, false
				}
				bits64 := math.Float64bits(value)
				if value == 0 {
					bits64 = 0
				}
				if math.IsNaN(value) {
					component = math.MaxUint64
				} else if bits64&0x8000000000000000 != 0 {
					component = ^bits64
				} else {
					component = bits64 ^ 0x8000000000000000
				}
			case dtype.BOOLT:
				component = uint64(vector.Bools()[row>>3] >> (row & 7) & 1)
			case dtype.STRT:
				value := vector.Strings()[row].View()
				if len(value) > part.stats.maxLen {
					return 0, false
				}
				for j := range len(value) {
					component = component<<8 | uint64(value[j])
				}
				component <<= uint(8 * (part.stats.maxLen - len(value)))
				component = component<<uint(part.lengthBits) | uint64(len(value))
			}
			if part.descending {
				component ^= keyBitMask(part.valueBits + part.lengthBits)
			}
		}
		if part.nullBits != 0 {
			nullTag := uint64(0)
			if l.mode == keyOrder && !part.nullsFirst {
				nullTag = 1
			}
			if isNull {
				component = nullTag << uint(part.valueBits+part.lengthBits)
			} else {
				component |= (nullTag ^ 1) << uint(part.valueBits+part.lengthBits)
			}
		}
		packed = packed<<uint(part.valueBits+part.lengthBits+part.nullBits) | component
	}
	return packed, true
}

func keyBitMask(width int) uint64 {
	if width == 64 {
		return math.MaxUint64
	}
	if width == 0 {
		return 0
	}
	return (uint64(1) << uint(width)) - 1
}
