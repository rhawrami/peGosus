package parquet

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/store"
)

// SetSelectedMaterialization enables copying only active flat string payloads.
// Install it before scanning. Batches keep their physical row domain and source validity;
// inactive string values are unspecified. The default reader preserves every value.
func (r *ParquetReader) SetSelectedMaterialization(enabled bool) bool {
	if r == nil || r.closed || r.group != 0 || r.row != 0 || r.cursors != nil {
		return false
	}
	r.selectedMaterialization = enabled
	return true
}

func (r *ParquetReader) materializeColumn(ctx context.Context, i, n int, selection *store.BitMap) (store.Vector, *ParquetError) {
	c := &r.cursors[i]
	if c.chunk.codec != 0 && c.chunk.codec != 1 {
		return store.Vector{}, c.failure(ParquetUnsupported, errors.New("unsupported codec"))
	}
	if c.column.typ.ID() == dtype.STRT {
		if i < len(r.dictionaryColumns) && r.dictionaryColumns[i] && (c.encoding == 8 || c.encoding == 2) && c.dictCount <= max(4096, n*4) {
			return c.materializeDictionaryStrings(ctx, n)
		}
		return makeParquetSelectedStringVector(ctx, c, n, selection)
	}
	vector := store.MakeVector(r.allocator, n, c.column.typ, c.column.optional)
	if vector.Kind() == store.VectorInvalid {
		return store.Vector{}, c.failure(ParquetResourceExhausted, errors.New("vector allocation failed"))
	}
	if failure := c.materializeFixed(ctx, &vector); failure != nil {
		vector.Release()
		return store.Vector{}, failure
	}
	return vector, nil
}

func (c *parquetCursor) skip(ctx context.Context, n int) *ParquetError {
	for n > 0 {
		if err := ctx.Err(); err != nil {
			return c.failure(ParquetCancelled, err)
		}
		if failure := c.advancePage(ctx); failure != nil {
			return failure
		}
		count := min(n, c.pageRows-c.pagePos, parquetBatchSize)
		if c.column.typ.ID() == dtype.STRT && c.encoding == 0 {
			for range count {
				if _, _, failure := c.next(ctx); failure != nil {
					return failure
				}
			}
			n -= count
			continue
		}
		values := count
		if c.levels != nil {
			values = 0
			for _, valid := range c.levels.AsBytes()[c.pagePos : c.pagePos+count] {
				if valid != 0 {
					values++
				}
			}
		}
		if values > c.nonNull {
			return c.failure(ParquetInvalid, errors.New("too many skipped values"))
		}
		switch {
		case c.encoding == 2 || c.encoding == 8:
			indices := c.indices.AsBytes()
			if c.valuePos > len(indices)/4-values {
				return c.failure(ParquetInvalid, errors.New("truncated skipped dictionary indices"))
			}
			for i := range values {
				index := binary.LittleEndian.Uint32(indices[(c.valuePos+i)*4:])
				if uint64(index) >= uint64(c.dictCount) {
					return c.failure(ParquetInvalid, errors.New("skipped dictionary index out of range"))
				}
			}
			c.valuePos += values
		case c.column.typ.ID() == dtype.BOOLT:
			if c.valuePos > len(c.values)*8-values {
				return c.failure(ParquetInvalid, errors.New("truncated skipped boolean values"))
			}
			c.valuePos += values
		default:
			width := c.column.typ.SlotBits() / 8
			if c.valuePos > len(c.values)-values*width {
				return c.failure(ParquetInvalid, errors.New("truncated skipped PLAIN values"))
			}
			c.valuePos += values * width
		}
		c.pagePos += count
		c.rows += int64(count)
		c.nonNull -= values
		n -= count
	}
	return nil
}
