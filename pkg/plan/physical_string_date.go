package plan

import (
	"math"
	"strings"
	"time"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/op/strop"
	"github.com/rhawrami/peGosus/pkg/store"
)

func evaluateDateMapping(operation exprOp, src, dst *store.Vector) bool {
	if src.TypeID() != dtype.DATET {
		return false
	}
	for i, days := range src.I32s() {
		date := time.Unix(int64(days)*86400, 0).UTC()
		switch operation {
		case exprOpExtractYear:
			dst.I32s()[i] = int32(date.Year())
		case exprOpExtractMonth:
			dst.I32s()[i] = int32(date.Month())
		case exprOpExtractDay:
			dst.I32s()[i] = int32(date.Day())
		case exprOpTruncateYear:
			truncated := time.Date(date.Year(), 1, 1, 0, 0, 0, 0, time.UTC).Unix() / 86400
			if truncated < math.MinInt32 || truncated > math.MaxInt32 {
				dst.Validity().Clear(i)
				continue
			}
			dst.I32s()[i] = int32(truncated)
		case exprOpTruncateMonth:
			truncated := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC).Unix() / 86400
			if truncated < math.MinInt32 || truncated > math.MaxInt32 {
				dst.Validity().Clear(i)
				continue
			}
			dst.I32s()[i] = int32(truncated)
		default:
			return false
		}
	}
	return true
}

func evaluateStringMapping(a *mem.Allocator, node physicalExprNode, nodes []physicalExprNode, vectors []store.Vector, general bool) store.Vector {
	first := &vectors[node.children[0]]
	var second, third *store.Vector
	var secondScalar, thirdScalar *Scalar
	if node.childCount >= 2 {
		if nodes[node.children[1]].kind == exprLiteral {
			secondScalar = &nodes[node.children[1]].literal
		} else {
			second = &vectors[node.children[1]]
		}
	}
	if node.childCount >= 3 {
		if nodes[node.children[2]].kind == exprLiteral {
			thirdScalar = &nodes[node.children[2]].literal
		} else {
			third = &vectors[node.children[2]]
		}
	}
	lengths := make([]int, first.Len())
	var valid []bool
	if node.nullable {
		valid = make([]bool, first.Len())
	}
	total := 0
	for row := range first.Len() {
		if first.Validity() != nil && !first.Validity().IsSet(row) || second != nil && second.Validity() != nil && !second.Validity().IsSet(row) || third != nil && third.Validity() != nil && !third.Validity().IsSet(row) || secondScalar != nil && secondScalar.IsNull() || thirdScalar != nil && thirdScalar.IsNull() {
			continue
		}
		if valid != nil {
			valid[row] = true
		}
		x := first.StringAt(row).View()
		length := uint64(len(x))
		switch node.operation {
		case exprOpConcat:
			if secondScalar != nil {
				length += uint64(len(secondScalar.stringValue()))
			} else {
				length += uint64(len(second.StringAt(row).View()))
			}
		case exprOpReplace:
			var old, replacement string
			if secondScalar != nil {
				old = secondScalar.stringValue()
			} else {
				old = second.StringAt(row).View()
			}
			if thirdScalar != nil {
				replacement = thirdScalar.stringValue()
			} else {
				replacement = third.StringAt(row).View()
			}
			occurrences := uint64(len(x)) + 1
			if old != "" {
				occurrences = uint64(strings.Count(x, old))
			}
			if len(replacement) >= len(old) {
				delta := uint64(len(replacement) - len(old))
				if delta != 0 && occurrences > (math.MaxUint32-length)/delta {
					return store.Vector{}
				}
				length += occurrences * delta
			} else {
				decrease := occurrences * uint64(len(old)-len(replacement))
				if decrease > length {
					return store.Vector{}
				}
				length -= decrease
			}
		case exprOpSlice:
			var startAt, stopAt int64
			if secondScalar != nil {
				startAt = secondScalar.i64()
			} else {
				startAt = second.I64s()[row]
			}
			if thirdScalar != nil {
				stopAt = thirdScalar.i64()
			} else {
				stopAt = third.I64s()[row]
			}
			start, stop := stringSliceBounds(len(x), startAt, stopAt)
			length = uint64(stop - start)
		}
		if length > math.MaxUint32 || length > uint64(int(^uint(0)>>1)-total) {
			return store.Vector{}
		}
		lengths[row] = int(length)
		total += int(length)
	}
	data := a.AllocSegTemp(total)
	if data == nil {
		return store.Vector{}
	}
	defer data.Dec()
	result := make([][]byte, first.Len())
	bytes := data.AsBytes()
	on := 0
	for row, length := range lengths {
		if valid != nil && !valid[row] {
			continue
		}
		out := bytes[on : on+length]
		x := first.StringAt(row).View()
		switch node.operation {
		case exprOpUpper:
			if length != 0 {
				strop.ToUpperASCII(borrowedStringBytes(x), out)
			}
		case exprOpLower:
			if length != 0 {
				strop.ToLowerASCII(borrowedStringBytes(x), out)
			}
		case exprOpConcat:
			n := copy(out, x)
			if secondScalar != nil {
				copy(out[n:], secondScalar.stringValue())
			} else {
				copy(out[n:], second.StringAt(row).View())
			}
		case exprOpReplace:
			var old, replacement string
			if secondScalar != nil {
				old = secondScalar.stringValue()
			} else {
				old = second.StringAt(row).View()
			}
			if thirdScalar != nil {
				replacement = thirdScalar.stringValue()
			} else {
				replacement = third.StringAt(row).View()
			}
			if old == "" {
				pos := copy(out, replacement)
				for i := range len(x) {
					out[pos] = x[i]
					pos++
					pos += copy(out[pos:], replacement)
				}
			} else {
				pos := 0
				for {
					at := strings.Index(x, old)
					if at < 0 {
						copy(out[pos:], x)
						break
					}
					pos += copy(out[pos:], x[:at])
					pos += copy(out[pos:], replacement)
					x = x[at+len(old):]
				}
			}
		case exprOpSlice:
			var startAt, stopAt int64
			if secondScalar != nil {
				startAt = secondScalar.i64()
			} else {
				startAt = second.I64s()[row]
			}
			if thirdScalar != nil {
				stopAt = thirdScalar.i64()
			} else {
				stopAt = third.I64s()[row]
			}
			start, stop := stringSliceBounds(len(x), startAt, stopAt)
			copy(out, x[start:stop])
		}
		result[row] = out
		on += length
	}
	if general {
		return store.MakeStringVector(a, result, valid)
	}
	return store.MakeStringVectorTemp(a, result, valid)
}

func stringSliceBounds(length int, start, stop int64) (int, int) {
	if start < 0 {
		start = 0
	}
	if stop < 0 {
		stop = 0
	}
	if start > int64(length) {
		start = int64(length)
	}
	if stop > int64(length) {
		stop = int64(length)
	}
	if stop < start {
		stop = start
	}
	return int(start), int(stop)
}

func likeMatches(value, pattern string) bool {
	i, j, star, retry := 0, 0, -1, 0
	for i < len(value) {
		if j < len(pattern) {
			p := pattern[j]
			if p == '%' {
				star = j
				retry = i
				j++
				continue
			}
			if p == '_' {
				i++
				j++
				continue
			}
			if p == '\\' && j+1 < len(pattern) && (pattern[j+1] == '%' || pattern[j+1] == '_' || pattern[j+1] == '\\') {
				j++
				p = pattern[j]
			}
			if value[i] == p {
				i++
				j++
				continue
			}
		}
		if star < 0 {
			return false
		}
		retry++
		i, j = retry, star+1
	}
	for j < len(pattern) {
		if pattern[j] != '%' {
			return false
		}
		j++
	}
	return true
}
