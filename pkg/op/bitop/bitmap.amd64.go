//go:build amd64

package bitop

//go:noescape
func bitWiseANDRetPopCount(src1, src2, dst []byte) uint64

//go:noescape
func bitWiseORRetPopCount(src1, src2, dst []byte) uint64

//go:noescape
func bitWiseXorWithPopCount(src1, src2, dst []byte) uint64

//go:noescape
func bitWiseAndNWithPopCount(src1, src2, dst []byte) uint64

//go:noescape
func bitWisePopCount(src []byte) uint64
