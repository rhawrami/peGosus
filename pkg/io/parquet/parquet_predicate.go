package parquet

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

// ScanPredicate describes an exact conjunctive comparison against a literal.
// Column is a file column index. Integer is used for signed integers, dates,
// timestamps and booleans (0 or 1); Text is used for strings. Nulls never match.
type ScanPredicate struct {
	Column  int
	Op      PruningOp
	Integer int64
	Text    string
}

type parquetPredicateState struct {
	predicates []ScanPredicate
	matches    *mem.Segment
}

// SetScanPredicates installs exact reader-side comparisons before scanning.
// Unsupported predicates reject the whole installation without changing it.
func (r *ParquetReader) SetScanPredicates(predicates []ScanPredicate) bool {
	if r == nil || r.closed || r.group != 0 || r.row != 0 || r.cursors != nil {
		return false
	}
	for _, p := range predicates {
		if p.Column < 0 || p.Column >= len(r.columns) || p.Op < PruneEqual || p.Op > PruneGreaterEqual {
			return false
		}
		switch r.columns[p.Column].typ.ID() {
		case dtype.INT32T, dtype.INT64T, dtype.DATET, dtype.TIMESTAMPTZT, dtype.STRT:
		case dtype.BOOLT:
			if p.Integer < 0 || p.Integer > 1 || p.Op != PruneEqual && p.Op != PruneNotEqual {
				return false
			}
		default:
			return false
		}
	}
	r.predicates = append([]ScanPredicate(nil), predicates...)
	return true
}

func (r *ParquetReader) predicateAll(p ScanPredicate, group parquetGroup) bool {
	chunk := group.chunks[p.Column]
	if !chunk.noNull || !chunk.hasMinMax {
		return false
	}
	switch r.columns[p.Column].typ.ID() {
	case dtype.INT32T, dtype.INT64T, dtype.DATET, dtype.TIMESTAMPTZT:
	default:
		return false
	}
	switch p.Op {
	case PruneEqual:
		return chunk.min == p.Integer && chunk.max == p.Integer
	case PruneNotEqual:
		return p.Integer < chunk.min || p.Integer > chunk.max
	case PruneLess:
		return chunk.max < p.Integer
	case PruneLessEqual:
		return chunk.max <= p.Integer
	case PruneGreater:
		return chunk.min > p.Integer
	case PruneGreaterEqual:
		return chunk.min >= p.Integer
	}
	return false
}

func (r *ParquetReader) releaseScanStates() {
	for i := range r.scanStates {
		if r.scanStates[i].matches != nil {
			r.scanStates[i].matches.Dec()
		}
	}
	r.scanStates, r.scanColumns = nil, nil
}

func (r *ParquetReader) startScanGroup() {
	r.scanColumns = append([]int(nil), r.selected...)
	r.scanStates = make([]parquetPredicateState, len(r.selected))
	for _, p := range r.predicates {
		if r.predicateAll(p, r.groups[r.group]) {
			continue
		}
		at := -1
		for i, col := range r.scanColumns {
			if col == p.Column {
				at = i
				break
			}
		}
		if at < 0 {
			at = len(r.scanColumns)
			r.scanColumns = append(r.scanColumns, p.Column)
			r.scanStates = append(r.scanStates, parquetPredicateState{})
		}
		r.scanStates[at].predicates = append(r.scanStates[at].predicates, p)
	}
	r.cursors = make([]parquetCursor, len(r.scanColumns))
	for i, col := range r.scanColumns {
		chunk := r.groups[r.group].chunks[col]
		r.cursors[i] = parquetCursor{chunk: chunk, offset: chunk.start, column: r.columns[col], input: r.input, a: r.allocator, options: r.options, group: r.groupBase + r.group, columnIndex: col}
	}
}

func parquetComparison(op PruningOp, comparison int) bool {
	switch op {
	case PruneEqual:
		return comparison == 0
	case PruneNotEqual:
		return comparison != 0
	case PruneLess:
		return comparison < 0
	case PruneLessEqual:
		return comparison <= 0
	case PruneGreater:
		return comparison > 0
	case PruneGreaterEqual:
		return comparison >= 0
	}
	return false
}

func (p ScanPredicate) match(typ dtype.TID, value []byte) bool {
	if typ == dtype.STRT {
		return parquetComparison(p.Op, bytes.Compare(value, []byte(p.Text)))
	}
	var v int64
	switch typ {
	case dtype.INT32T, dtype.DATET:
		v = int64(int32(binary.LittleEndian.Uint32(value)))
	case dtype.INT64T, dtype.TIMESTAMPTZT:
		v = int64(binary.LittleEndian.Uint64(value))
	case dtype.BOOLT:
		v = int64(value[0])
	}
	comparison := 0
	if v < p.Integer {
		comparison = -1
	} else if v > p.Integer {
		comparison = 1
	}
	return parquetComparison(p.Op, comparison)
}

func (s *parquetPredicateState) scan(ctx context.Context, c *parquetCursor, n int, mask *store.BitMap) *ParquetError {
	on := 0
	for on < n {
		if err := ctx.Err(); err != nil {
			return c.failure(ParquetCancelled, err)
		}
		if failure := c.advancePage(ctx); failure != nil {
			return failure
		}
		stop := min(n-on, c.pageRows-c.pagePos, parquetBatchSize)
		if c.encoding == 2 || c.encoding == 8 {
			if s.matches == nil && c.dictCount != 0 {
				s.matches = c.a.AllocSegTemp(c.dictCount)
				if s.matches == nil {
					return c.failure(ParquetResourceExhausted, errors.New("dictionary predicate allocation failed"))
				}
				matched := s.matches.AsBytes()[:c.dictCount]
				for index := range matched {
					if index&4095 == 0 {
						if err := ctx.Err(); err != nil {
							return c.failure(ParquetCancelled, err)
						}
					}
					var value []byte
					switch c.column.typ.ID() {
					case dtype.STRT:
						position := uint64(c.dictOffsets.AsI64T()[index])
						value = c.dictRaw[uint32(position):uint32(position>>32)]
					case dtype.BOOLT:
						c.boolValue[0] = c.dictRaw[index/8] >> (index & 7) & 1
						value = c.boolValue[:]
					default:
						width := c.column.typ.SlotBits() / 8
						value = c.dictRaw[index*width : (index+1)*width]
					}
					matched[index] = 1
					for _, p := range s.predicates {
						if !p.match(c.column.typ.ID(), value) {
							matched[index] = 0
							break
						}
					}
				}
			}
			var matched []byte
			if s.matches != nil {
				matched = s.matches.AsBytes()[:c.dictCount]
			}
			for j := range stop {
				row := c.pagePos + j
				if c.levels != nil && c.levels.AsBytes()[row] == 0 {
					mask.Clear(on + j)
					continue
				}
				if c.nonNull == 0 || c.valuePos >= c.indices.Len()/4 {
					return c.failure(ParquetInvalid, errors.New("too many dictionary values"))
				}
				index := binary.LittleEndian.Uint32(c.indices.AsBytes()[4*c.valuePos:])
				if uint64(index) >= uint64(c.dictCount) {
					return c.failure(ParquetInvalid, errors.New("dictionary index out of range"))
				}
				c.valuePos++
				c.nonNull--
				if matched[index] == 0 {
					mask.Clear(on + j)
				}
			}
			c.pagePos += stop
			c.rows += int64(stop)
		} else if c.column.typ.ID() != dtype.STRT && c.levels == nil {
			if stop > c.nonNull {
				return c.failure(ParquetInvalid, errors.New("too many predicate values"))
			}
			boolean := c.column.typ.ID() == dtype.BOOLT
			width := c.column.typ.SlotBits() / 8
			if boolean {
				if c.valuePos > len(c.values)*8-stop {
					return c.failure(ParquetInvalid, errors.New("truncated boolean values"))
				}
			} else if stop > len(c.values)/width || c.valuePos > len(c.values)-stop*width {
				return c.failure(ParquetInvalid, errors.New("truncated predicate values"))
			}
			start := 0
			if boolean && on&7 == 0 && c.valuePos&7 == 0 {
				zero, one := true, true
				for _, p := range s.predicates {
					zero = zero && p.match(dtype.BOOLT, []byte{0})
					one = one && p.match(dtype.BOOLT, []byte{1})
				}
				length := stop / 8
				dst, src := mask.Bytes()[on/8:on/8+length], c.values[c.valuePos/8:c.valuePos/8+length]
				for i := range dst {
					switch {
					case !zero && !one:
						dst[i] = 0
					case !zero:
						dst[i] &= src[i]
					case !one:
						dst[i] &^= src[i]
					}
				}
				mask.RecalcNiN()
				start = length * 8
			}
			for j := start; j < stop; j++ {
				var value []byte
				if boolean {
					at := c.valuePos + j
					c.boolValue[0] = c.values[at/8] >> (at & 7) & 1
					value = c.boolValue[:]
				} else {
					at := c.valuePos + j*width
					value = c.values[at : at+width]
				}
				for _, p := range s.predicates {
					if !p.match(c.column.typ.ID(), value) {
						mask.Clear(on + j)
						break
					}
				}
			}
			if boolean {
				c.valuePos += stop
			} else {
				c.valuePos += stop * width
			}
			c.pagePos += stop
			c.rows += int64(stop)
			c.nonNull -= stop
		} else {
			for j := range stop {
				value, valid, failure := c.next(ctx)
				if failure != nil {
					return failure
				}
				if !valid {
					mask.Clear(on + j)
					continue
				}
				for _, p := range s.predicates {
					if !p.match(c.column.typ.ID(), value) {
						mask.Clear(on + j)
						break
					}
				}
			}
		}
		on += stop
	}
	return nil
}

func (s *parquetPredicateState) filterVector(vector *store.Vector, mask *store.BitMap) {
	for row := range vector.Len() {
		if !mask.IsSet(row) {
			continue
		}
		if vector.Validity() != nil && !vector.Validity().IsSet(row) {
			mask.Clear(row)
			continue
		}
		for _, p := range s.predicates {
			var comparison int
			if vector.Type().ID() == dtype.STRT {
				comparison = strings.Compare(vector.StringAt(row).View(), p.Text)
			} else {
				var value int64
				switch vector.Type().ID() {
				case dtype.BOOLT:
					value = int64(vector.Bools()[row/8] >> (row & 7) & 1)
				case dtype.INT32T, dtype.DATET:
					value = int64(vector.I32s()[row])
				case dtype.INT64T, dtype.TIMESTAMPTZT:
					value = vector.I64s()[row]
				}
				if value < p.Integer {
					comparison = -1
				} else if value > p.Integer {
					comparison = 1
				}
			}
			if !parquetComparison(p.Op, comparison) {
				mask.Clear(row)
				break
			}
		}
	}
}
