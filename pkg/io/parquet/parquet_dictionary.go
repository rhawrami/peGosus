package parquet

import (
	"context"
	"errors"
	"math"
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/store"
)

// SetDictionaryColumns enables owned dictionary string vectors for selected file
// columns before scanning. PLAIN pages still return flat vectors; batches may
// end at page boundaries. The default reader always materializes flat vectors.
func (r *ParquetReader) SetDictionaryColumns(columns []int) bool {
	if r == nil || r.closed || r.group != 0 || r.row != 0 || r.cursors != nil {
		return false
	}
	encoded := make([]bool, len(r.selected))
	for _, column := range columns {
		if column < 0 || column >= len(r.columns) || r.columns[column].typ.ID() != dtype.STRT {
			return false
		}
		found := false
		for i, selected := range r.selected {
			if selected == column {
				encoded[i], found = true, true
			}
		}
		if !found {
			return false
		}
	}
	r.dictionaryColumns = encoded
	return true
}

func (c *parquetCursor) materializeDictionaryStrings(ctx context.Context, n int) (store.Vector, *ParquetError) {
	if n < 0 || n > math.MaxInt/4 || n > c.pageRows-c.pagePos {
		return store.Vector{}, c.failure(ParquetInvalid, errors.New("invalid dictionary batch length"))
	}
	if c.dictStrings.Kind() == store.VectorInvalid {
		if c.dictCount > math.MaxInt/dtype.StringSize {
			return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("dictionary descriptor overflow"))
		}
		data := c.a.AllocSeg(c.dictCount * dtype.StringSize)
		if data == nil {
			return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("dictionary descriptor allocation failed"))
		}
		descriptors := unsafe.Slice((*dtype.String)(unsafe.Pointer(unsafe.SliceData(data.AsBytes()))), c.dictCount)
		for i, position := range c.dictOffsets.AsI64T() {
			if i&4095 == 0 {
				if err := ctx.Err(); err != nil {
					data.Dec()
					return store.Vector{}, c.failure(ParquetCancelled, err)
				}
			}
			descriptors[i] = dtype.MakeString(c.dictRaw[uint32(position):uint32(uint64(position)>>32)])
		}
		c.dictionary.Inc()
		c.dictStrings = store.MakeVectorFromOwnedSegments(dtype.StringT(), c.dictCount, data, nil, c.dictionary)
		if c.dictStrings.Kind() == store.VectorInvalid {
			return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("dictionary vector allocation failed"))
		}
	}
	indices := c.a.AllocSeg(n * 4)
	if indices == nil {
		return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("dictionary row allocation failed"))
	}
	var validity *store.BitMap
	if c.column.optional {
		validity = store.MakeBitMap(c.a, n)
		if validity == nil {
			indices.Dec()
			return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("dictionary validity allocation failed"))
		}
	}
	source := c.indices.AsU32T()
	destination := indices.AsU32T()
	for row := range n {
		if row&4095 == 0 {
			if err := ctx.Err(); err != nil {
				indices.Dec()
				validity.Release()
				return store.Vector{}, c.failure(ParquetCancelled, err)
			}
		}
		destination[row] = 0
		if c.levels != nil && c.levels.AsBytes()[c.pagePos+row] == 0 {
			continue
		}
		if c.nonNull == 0 || c.valuePos >= len(source) {
			indices.Dec()
			validity.Release()
			return store.Vector{}, c.failure(ParquetInvalid, errors.New("too many dictionary values"))
		}
		destination[row] = source[c.valuePos]
		c.valuePos++
		c.nonNull--
		if validity != nil {
			validity.Set(row)
		}
	}
	v := store.MakeDictionaryStringVectorFromOwnedSegments(c.dictStrings.Retain(), n, indices, validity)
	if v.Kind() == store.VectorInvalid {
		return v, c.failure(ParquetInvalid, errors.New("dictionary index out of range"))
	}
	c.pagePos += n
	c.rows += int64(n)
	return v, nil
}

func (r *ParquetReader) dictionaryBatchLength(ctx context.Context, n int) (int, *ParquetError) {
	for i, enabled := range r.dictionaryColumns {
		if enabled {
			if failure := r.cursors[i].advancePage(ctx); failure != nil {
				return 0, failure
			}
			n = min(n, r.cursors[i].pageRows-r.cursors[i].pagePos)
		}
	}
	return n, nil
}
