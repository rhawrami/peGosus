package boolop

// And applies SQL three-valued AND to bit-packed values and validity. The
// destination bitmaps may exactly alias input bitmaps but not each other.
func And(src1, src1Validity, src2, src2Validity, dst, dstValidity []byte, n int) {
	byteLen := (n + 7) / 8
	for i := 0; i < byteLen; i++ {
		value1 := src1[i]
		valid1 := src1Validity[i]
		value2 := src2[i]
		valid2 := src2Validity[i]
		trueBits := value1 & valid1 & value2 & valid2
		falseBits := (valid1 &^ value1) | (valid2 &^ value2)
		dst[i] = trueBits
		dstValidity[i] = trueBits | falseBits
	}
	if n%8 != 0 {
		mask := byte(1<<(n%8) - 1)
		dst[byteLen-1] &= mask
		dstValidity[byteLen-1] &= mask
	}
}

// Or applies SQL three-valued OR to bit-packed values and validity. The
// destination bitmaps may exactly alias input bitmaps but not each other.
func Or(src1, src1Validity, src2, src2Validity, dst, dstValidity []byte, n int) {
	byteLen := (n + 7) / 8
	for i := 0; i < byteLen; i++ {
		value1 := src1[i]
		valid1 := src1Validity[i]
		value2 := src2[i]
		valid2 := src2Validity[i]
		trueBits := (value1 & valid1) | (value2 & valid2)
		falseBits := valid1 &^ value1 & valid2 &^ value2
		dst[i] = trueBits
		dstValidity[i] = trueBits | falseBits
	}
	if n%8 != 0 {
		mask := byte(1<<(n%8) - 1)
		dst[byteLen-1] &= mask
		dstValidity[byteLen-1] &= mask
	}
}

// Not applies SQL three-valued NOT to bit-packed values and validity. The
// destination bitmaps may exactly alias input bitmaps but not each other.
func Not(src, srcValidity, dst, dstValidity []byte, n int) {
	byteLen := (n + 7) / 8
	for i := 0; i < byteLen; i++ {
		value := src[i]
		validity := srcValidity[i]
		dst[i] = ^value & validity
		dstValidity[i] = validity
	}
	if n%8 != 0 {
		mask := byte(1<<(n%8) - 1)
		dst[byteLen-1] &= mask
		dstValidity[byteLen-1] &= mask
	}
}
