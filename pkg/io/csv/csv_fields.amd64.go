//go:build amd64 && goexperiment.simd

package csv

import "simd/archsimd"

func init() {
	if archsimd.X86.AVX2() {
		classifyCSV = classifyCSVAVX2
	}
}

func classifyCSVAVX2(src []byte, delimiter byte) csvMasks {
	separators := archsimd.BroadcastUint8x32(delimiter)
	quotes := archsimd.BroadcastUint8x32('"')
	newlines := archsimd.BroadcastUint8x32('\n')
	var masks csvMasks
	i := 0
	for ; i+32 <= len(src); i += 32 {
		chunk := archsimd.LoadUint8x32(src[i:])
		masks.delimiters |= uint64(chunk.Equal(separators).ToBits()) << i
		masks.quotes |= uint64(chunk.Equal(quotes).ToBits()) << i
		masks.newlines |= uint64(chunk.Equal(newlines).ToBits()) << i
	}
	if i < len(src) {
		tail := classifyCSVScalar(src[i:], delimiter)
		masks.delimiters |= tail.delimiters << i
		masks.quotes |= tail.quotes << i
		masks.newlines |= tail.newlines << i
	}
	return masks
}
