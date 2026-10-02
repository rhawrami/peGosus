package parquet

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (c *parquetCursor) materializeFixed(ctx context.Context, vector *store.Vector) *ParquetError {
	out := 0
	for out < vector.Len() {
		if err := ctx.Err(); err != nil {
			return c.failure(ParquetCancelled, err)
		}
		if err := c.advancePage(ctx); err != nil {
			return err
		}
		count := min(vector.Len()-out, c.pageRows-c.pagePos, parquetBatchSize)
		end := c.pagePos + count
		var levels []byte
		if c.levels != nil {
			levels = c.levels.AsBytes()
		}
		for c.pagePos < end {
			if levels != nil && levels[c.pagePos] == 0 {
				vector.Validity().Clear(out)
				c.pagePos++
				c.rows++
				out++
				continue
			}
			length := end - c.pagePos
			if levels != nil {
				length = 1
				for c.pagePos+length < end && levels[c.pagePos+length] != 0 {
					length++
				}
			}
			if err := c.materializeValues(vector, out, length); err != nil {
				return err
			}
			c.pagePos += length
			c.rows += int64(length)
			c.nonNull -= length
			out += length
		}
	}
	return nil
}

func (c *parquetCursor) materializeValues(vector *store.Vector, out, count int) *ParquetError {
	if count > c.nonNull {
		return c.failure(ParquetInvalid, errors.New("too many values"))
	}
	boolean := c.column.typ.ID() == dtype.BOOLT
	size := c.column.typ.SlotBits() / 8
	if c.encoding == 2 || c.encoding == 8 {
		indices := c.indices.AsBytes()
		if c.valuePos > len(indices)/4-count {
			return c.failure(ParquetInvalid, errors.New("truncated dictionary indices"))
		}
		indices = indices[c.valuePos*4 : (c.valuePos+count)*4]
		dst := vector.Data().AsBytes()
		if !boolean {
			dst = dst[out*size:]
		}
		for i := range count {
			index := binary.LittleEndian.Uint32(indices[i*4:])
			if uint64(index) >= uint64(c.dictCount) {
				return c.failure(ParquetInvalid, errors.New("dictionary index out of range"))
			}
			if boolean {
				if c.dictRaw[index/8]>>(index&7)&1 != 0 {
					dst[(out+i)/8] |= 1 << ((out + i) & 7)
				}
			} else if size == 4 {
				binary.NativeEndian.PutUint32(dst[i*4:], binary.LittleEndian.Uint32(c.dictRaw[int(index)*4:]))
			} else {
				binary.NativeEndian.PutUint64(dst[i*8:], binary.LittleEndian.Uint64(c.dictRaw[int(index)*8:]))
			}
		}
		c.valuePos += count
		return nil
	}
	if boolean {
		if c.valuePos > len(c.values)*8-count {
			return c.failure(ParquetInvalid, errors.New("truncated boolean values"))
		}
		dst := vector.Bools()
		if c.valuePos&7 == 0 && out&7 == 0 {
			bytes := count / 8
			copy(dst[out/8:], c.values[c.valuePos/8:c.valuePos/8+bytes])
			out += bytes * 8
			c.valuePos += bytes * 8
			count -= bytes * 8
		}
		for i := range count {
			index := c.valuePos + i
			if c.values[index/8]>>(index&7)&1 != 0 {
				dst[(out+i)/8] |= 1 << ((out + i) & 7)
			}
		}
		c.valuePos += count
		return nil
	}
	if c.valuePos > len(c.values)-count*size {
		return c.failure(ParquetInvalid, errors.New("truncated PLAIN value"))
	}
	src := c.values[c.valuePos : c.valuePos+count*size]
	dst := vector.Data().AsBytes()[out*size : (out+count)*size]
	if binary.NativeEndian.Uint16([]byte{1, 0}) == 1 {
		copy(dst, src)
	} else if size == 4 {
		for i := range count {
			binary.NativeEndian.PutUint32(dst[i*4:], binary.LittleEndian.Uint32(src[i*4:]))
		}
	} else {
		for i := range count {
			binary.NativeEndian.PutUint64(dst[i*8:], binary.LittleEndian.Uint64(src[i*8:]))
		}
	}
	c.valuePos += count * size
	return nil
}
