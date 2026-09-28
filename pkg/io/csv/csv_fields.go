package csv

import "math/bits"

type csvMasks struct {
	delimiters uint64
	quotes     uint64
	newlines   uint64
}

type csvClassifier func([]byte, byte) csvMasks

var classifyCSV csvClassifier = classifyCSVScalar

func classifyCSVScalar(src []byte, delimiter byte) csvMasks {
	var masks csvMasks
	for i, value := range src {
		bit := uint64(1) << i
		if value == delimiter {
			masks.delimiters |= bit
		}
		if value == '"' {
			masks.quotes |= bit
		}
		if value == '\n' {
			masks.newlines |= bit
		}
	}
	return masks
}

func scanCSVChunk(src []byte, delimiter byte, offsets []int, base int, inQuote bool) ([]int, bool, bool, bool) {
	hasQuote, ended := false, false
	for i := 0; i < len(src); i += 64 {
		block := src[i:min(i+64, len(src))]
		masks := classifyCSV(block, delimiter)
		hasQuote = hasQuote || masks.quotes != 0
		inside := masks.quotes
		inside ^= inside << 1
		inside ^= inside << 2
		inside ^= inside << 4
		inside ^= inside << 8
		inside ^= inside << 16
		inside ^= inside << 32
		if inQuote {
			inside = ^inside
		}
		inQuote = inside&(uint64(1)<<(len(block)-1)) != 0
		if offsets != nil {
			separators := masks.delimiters &^ inside
			for separators != 0 {
				offsets = append(offsets, base+i+bits.TrailingZeros64(separators))
				separators &= separators - 1
			}
		}
		ended = ended || masks.newlines&^inside != 0
	}
	return offsets, hasQuote, inQuote, ended
}
