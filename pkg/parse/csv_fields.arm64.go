//go:build arm64 && goexperiment.simd

package parse

import "simd/archsimd"

func init() { classifyCSV = classifyCSVNEON }

func classifyCSVNEON(src []byte, delimiter byte) csvMasks {
	separators := archsimd.BroadcastUint8x16(delimiter)
	quotes := archsimd.BroadcastUint8x16('"')
	newlines := archsimd.BroadcastUint8x16('\n')
	var masks csvMasks
	i := 0
	for ; i+16 <= len(src); i += 16 {
		chunk := archsimd.LoadUint8x16(src[i:])
		separatorWords := chunk.Equal(separators).ToInt8x16().ToBits().ReshapeToUint64s()
		quoteWords := chunk.Equal(quotes).ToInt8x16().ToBits().ReshapeToUint64s()
		newlineWords := chunk.Equal(newlines).ToInt8x16().ToBits().ReshapeToUint64s()
		masks.delimiters |= (csvNEONWordMask(separatorWords.GetElem(0)) | csvNEONWordMask(separatorWords.GetElem(1))<<8) << i
		masks.quotes |= (csvNEONWordMask(quoteWords.GetElem(0)) | csvNEONWordMask(quoteWords.GetElem(1))<<8) << i
		masks.newlines |= (csvNEONWordMask(newlineWords.GetElem(0)) | csvNEONWordMask(newlineWords.GetElem(1))<<8) << i
	}
	if i < len(src) {
		tail := classifyCSVScalar(src[i:], delimiter)
		masks.delimiters |= tail.delimiters << i
		masks.quotes |= tail.quotes << i
		masks.newlines |= tail.newlines << i
	}
	return masks
}

func csvNEONWordMask(word uint64) uint64 {
	return (word & 0x8080808080808080) * 0x0002040810204081 >> 56
}
