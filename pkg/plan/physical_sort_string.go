package plan

import (
	"encoding/binary"

	"github.com/rhawrami/peGosus/pkg/mem"
)

const compactStringRadixMinRows = 256

func (s *compactSortState) stringPrefixes(a *mem.Allocator, key *compactSortKey, order physicalOrderKey) *mem.Segment {
	if s.length < compactStringRadixMinRows || s.length > int(^uint(0)>>1)/16 {
		return nil
	}
	var prefixes *mem.Segment
	if s.scope != nil {
		prefixes, _ = s.scope.TryAllocSeg(s.length * 16)
	} else {
		prefixes = a.AllocSegTemp(s.length * 16)
	}
	if prefixes == nil {
		return nil
	}
	values := prefixes.AsU64T()
	for i, id := range s.rows.AsU64T()[s.offset : s.offset+s.length] {
		if i&65535 == 0 && s.cancelled() {
			prefixes.Dec()
			return nil
		}
		vector, row := &key.vectors[id>>32], int(uint32(id))
		null := vector.Validity() != nil && !vector.Validity().IsSet(row)
		var normalized [16]byte
		if null != order.nullsFirst {
			normalized[0] = 1
		}
		if !null {
			copy(normalized[1:], vector.StringAt(row).View())
		}
		first, second := binary.BigEndian.Uint64(normalized[:8]), binary.BigEndian.Uint64(normalized[8:])
		if !null && order.descending {
			first ^= 0x00ffffffffffffff
			second = ^second
		}
		values[i*2], values[i*2+1] = first, second
	}
	return prefixes
}

func (s *compactSortState) sortStrings(a *mem.Allocator, key *compactSortKey, order physicalOrderKey, source, destination []uint64) ([]uint64, []uint64, bool) {
	rows := s.rows.AsU64T()[s.offset:]
	prefixes := s.stringPrefixes(a, key, order)
	var values []uint64
	if prefixes != nil {
		defer prefixes.Dec()
		values = prefixes.AsU64T()
		var vary [2]uint64
		first := source[0] * 2
		for i, id := range source {
			if i&65535 == 0 && s.cancelled() {
				return nil, nil, false
			}
			vary[0] |= values[id*2] ^ values[first]
			vary[1] |= values[id*2+1] ^ values[first+1]
		}
		for word := 1; word >= 0; word-- {
			for shift := uint(0); shift < 64; shift += 8 {
				if byte(vary[word]>>shift) == 0 {
					continue
				}
				var counts [256]int
				for i, id := range source {
					if i&65535 == 0 && s.cancelled() {
						return nil, nil, false
					}
					counts[byte(values[id*2+uint64(word)]>>shift)]++
				}
				position := 0
				for i, n := range counts {
					counts[i] = position
					position += n
				}
				for i, id := range source {
					if i&65535 == 0 && s.cancelled() {
						return nil, nil, false
					}
					bucket := byte(values[id*2+uint64(word)] >> shift)
					destination[counts[bucket]] = id
					counts[bucket]++
				}
				source, destination = destination, source
			}
		}
	}
	less := func(left, right uint64) bool {
		l, r := rows[left], rows[right]
		lv, rv := &key.vectors[l>>32], &key.vectors[r>>32]
		li, ri := int(uint32(l)), int(uint32(r))
		if values == nil {
			ln, rn := lv.Validity() != nil && !lv.Validity().IsSet(li), rv.Validity() != nil && !rv.Validity().IsSet(ri)
			if ln != rn {
				return ln == order.nullsFirst
			}
			if ln {
				return false
			}
		}
		ltext, rtext := lv.StringAt(li).View(), rv.StringAt(ri).View()
		if order.descending {
			return ltext > rtext
		}
		return ltext < rtext
	}
	for start := 0; start < len(source); {
		if s.cancelled() {
			return nil, nil, false
		}
		end := len(source)
		if values != nil {
			first := source[start] * 2
			end = start + 1
			for end < len(source) && values[source[end]*2] == values[first] && values[source[end]*2+1] == values[first+1] {
				if end&65535 == 0 && s.cancelled() {
					return nil, nil, false
				}
				end++
			}
			nullClass := uint64(1)
			if order.nullsFirst {
				nullClass = 0
			}
			if values[first]>>56 == nullClass || end-start == 1 {
				start = end
				continue
			}
			// Equal lengths make complete cached prefixes exact, including embedded NULs.
			row := rows[source[start]]
			length := key.vectors[row>>32].StringAt(int(uint32(row))).Len()
			if length <= 15 {
				equal := true
				for i := start + 1; i < end; i++ {
					if i&65535 == 0 && s.cancelled() {
						return nil, nil, false
					}
					row = rows[source[i]]
					if key.vectors[row>>32].StringAt(int(uint32(row))).Len() != length {
						equal = false
						break
					}
				}
				if equal {
					start = end
					continue
				}
			}
		}
		// Zero padding is only a prefix: embedded NULs and suffixes need exact comparison.
		src, dst := source[start:end], destination[start:end]
		for width := 1; width < len(src); width *= 2 {
			for offset := 0; offset < len(src); offset += 2 * width {
				if s.cancelled() {
					return nil, nil, false
				}
				middle, limit := min(offset+width, len(src)), min(offset+2*width, len(src))
				l, r := offset, middle
				for on := offset; on < limit; on++ {
					if on&65535 == 0 && s.cancelled() {
						return nil, nil, false
					}
					if l < middle && (r == limit || !less(src[r], src[l])) {
						dst[on] = src[l]
						l++
					} else {
						dst[on] = src[r]
						r++
					}
				}
			}
			src, dst = dst, src
		}
		if start == 0 && end == len(source) {
			return src, dst, true
		}
		if &src[0] != &source[start] {
			copy(source[start:end], src)
		}
		start = end
	}
	return source, destination, !s.cancelled()
}
