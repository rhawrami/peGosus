package parquet

import (
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"math"
	"unicode/utf8"

	"github.com/golang/snappy"
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

type parquetCursor struct {
	chunk                     parquetChunk
	offset                    int64
	input                     io.ReaderAt
	a                         *mem.Allocator
	options                   ParquetOptions
	column                    parquetColumn
	group, columnIndex, pages int
	rows                      int64
	pageRows, pagePos         int
	encoding                  int64
	data                      *mem.Segment
	decoded                   *mem.Segment
	levels                    *mem.Segment
	indices                   *mem.Segment
	dictionary                *mem.Segment
	dictOffsets               *mem.Segment
	dictRaw                   []byte
	dictCount                 int
	boolValue                 [1]byte
	values                    []byte
	valuePos, nonNull         int
}

func (c *parquetCursor) failure(code ParquetErrorCode, err error) *ParquetError {
	return &ParquetError{code: code, group: c.group, column: c.columnIndex, page: c.pages, cause: err}
}

func (c *parquetCursor) clearPage() {
	for _, seg := range []*mem.Segment{c.data, c.decoded, c.levels, c.indices} {
		if seg != nil {
			seg.Dec()
		}
	}
	c.data, c.decoded, c.levels, c.indices = nil, nil, nil, nil
	c.values = nil
}

func (c *parquetCursor) close() {
	c.clearPage()
	if c.dictionary != nil {
		c.dictionary.Dec()
		c.dictionary = nil
	}
	if c.dictOffsets != nil {
		c.dictOffsets.Dec()
		c.dictOffsets = nil
	}
	c.dictRaw = nil
	c.dictCount = 0
}

func (c *parquetCursor) advancePage(ctx context.Context) *ParquetError {
	for c.pagePos == c.pageRows {
		if c.pageRows > 0 && c.nonNull != 0 {
			return c.failure(ParquetInvalid, errors.New("page value count mismatch"))
		}
		if c.pageRows > 0 && c.encoding == 0 && (c.column.typ.ID() != dtype.BOOLT && c.valuePos != len(c.values) || c.column.typ.ID() == dtype.BOOLT && (c.valuePos+7)/8 != len(c.values)) {
			return c.failure(ParquetInvalid, errors.New("page value length mismatch"))
		}
		c.clearPage()
		if c.rows >= c.chunk.values {
			return c.failure(ParquetInvalid, errors.New("column ended early"))
		}
		if err := c.readPage(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *parquetCursor) next(ctx context.Context) ([]byte, bool, *ParquetError) {
	if err := c.advancePage(ctx); err != nil {
		return nil, false, err
	}
	row := c.pagePos
	c.pagePos++
	c.rows++
	if c.levels != nil && c.levels.AsBytes()[row] == 0 {
		return nil, false, nil
	}
	if c.nonNull == 0 {
		return nil, false, c.failure(ParquetInvalid, errors.New("too many values"))
	}
	c.nonNull--
	if c.encoding == 8 || c.encoding == 2 {
		index := binary.LittleEndian.Uint32(c.indices.AsBytes()[4*c.valuePos:])
		c.valuePos++
		if int(index) >= c.dictCount {
			return nil, false, c.failure(ParquetInvalid, errors.New("dictionary index out of range"))
		}
		switch c.column.typ.ID() {
		case dtype.BOOLT:
			c.boolValue[0] = c.dictRaw[index/8] >> (index & 7) & 1
			return c.boolValue[:], true, nil
		case dtype.STRT:
			position := uint64(c.dictOffsets.AsI64T()[index])
			return c.dictRaw[uint32(position):uint32(position>>32)], true, nil
		default:
			size := c.column.typ.SlotBits() / 8
			return c.dictRaw[int(index)*size : (int(index)+1)*size], true, nil
		}
	}
	if c.column.typ.ID() == dtype.BOOLT {
		index := c.valuePos
		if index/8 >= len(c.values) {
			return nil, false, c.failure(ParquetInvalid, errors.New("truncated boolean values"))
		}
		c.valuePos++
		c.boolValue[0] = c.values[index/8] >> (index & 7) & 1
		return c.boolValue[:], true, nil
	}
	v, n, ok := parquetPlainValue(c.values[c.valuePos:], c.column.typ.ID())
	if !ok {
		return nil, false, c.failure(ParquetInvalid, errors.New("truncated PLAIN value"))
	}
	c.valuePos += n
	if c.column.typ.ID() == dtype.STRT && !utf8.Valid(v) {
		return nil, false, c.failure(ParquetInvalid, errors.New("invalid UTF-8 value"))
	}
	return v, true, nil
}

func parquetPlainValue(buf []byte, typ dtype.TID) ([]byte, int, bool) {
	size := 0
	switch typ {
	case dtype.INT32T, dtype.DATET, dtype.FLOAT32T:
		size = 4
	case dtype.INT64T, dtype.TIMESTAMPTZT, dtype.FLOAT64T:
		size = 8
	case dtype.STRT:
		if len(buf) < 4 {
			return nil, 0, false
		}
		length := binary.LittleEndian.Uint32(buf)
		if uint64(length) > uint64(len(buf)-4) {
			return nil, 0, false
		}
		size = int(length)
		return buf[4 : 4+size], size + 4, true
	}
	if size == 0 || len(buf) < size {
		return nil, 0, false
	}
	return buf[:size], size, true
}

func (c *parquetCursor) readPage(ctx context.Context) *ParquetError {
	for {
		if err := ctx.Err(); err != nil {
			return c.failure(ParquetCancelled, err)
		}
		if c.offset >= c.chunk.end {
			return c.failure(ParquetInvalid, errors.New("missing data page"))
		}
		maxHeader := int(min(int64(parquetHeaderLimit), c.chunk.end-c.offset))
		var d compactDecoder
		var header compactValue
		var headerBytes *mem.Segment
		for hlen := min(256, maxHeader); ; hlen = min(hlen*2, maxHeader) {
			hseg := c.a.AllocSegTemp(hlen)
			if hseg == nil {
				return c.failure(ParquetResourceExhausted, errors.New("page header allocation failed"))
			}
			read, readErr := c.input.ReadAt(hseg.AsBytes(), c.offset)
			if readErr != nil || read != hlen {
				hseg.Dec()
				if readErr == nil {
					readErr = io.ErrUnexpectedEOF
				}
				return c.failure(ParquetReadFailure, readErr)
			}
			d = compactDecoder{buf: hseg.AsBytes()}
			var err error
			header, err = d.decode(12)
			if err == nil {
				headerBytes = hseg
				break
			}
			hseg.Dec()
			if hlen == maxHeader {
				return c.failure(ParquetInvalid, errors.New("invalid page header"))
			}
		}
		compressed, uncompressed := header.number(3), header.number(2)
		if header.field(1).kind == 0 || compressed < 0 || uncompressed < 0 || compressed > c.chunk.end-c.offset-int64(d.pos) {
			headerBytes.Dec()
			return c.failure(ParquetInvalid, errors.New("invalid page length"))
		}
		if compressed > int64(c.options.MaxPageBytes) || uncompressed > int64(c.options.MaxDecodedPageBytes) || compressed > math.MaxInt || uncompressed > math.MaxInt {
			headerBytes.Dec()
			return c.failure(ParquetResourceExhausted, errors.New("page exceeds configured size limit"))
		}
		c.offset += int64(d.pos)
		body := c.a.AllocSegTemp(int(compressed))
		if body == nil {
			headerBytes.Dec()
			return c.failure(ParquetResourceExhausted, errors.New("page allocation failed"))
		}
		cached := copy(body.AsBytes(), headerBytes.AsBytes()[d.pos:])
		headerBytes.Dec()
		remaining := int(compressed) - cached
		if remaining != 0 {
			if n, err := c.input.ReadAt(body.AsBytes()[cached:compressed], c.offset+int64(cached)); err != nil || n != remaining {
				body.Dec()
				if err == nil {
					err = io.ErrUnexpectedEOF
				}
				return c.failure(ParquetReadFailure, err)
			}
		}
		c.offset += compressed
		c.pages++
		if header.field(4).kind != 0 && uint32(header.number(4)) != crc32.ChecksumIEEE(body.AsBytes()[:compressed]) {
			body.Dec()
			return c.failure(ParquetInvalid, errors.New("page CRC mismatch"))
		}
		kind := header.number(1)
		if kind != 0 && kind != 2 && kind != 3 {
			body.Dec()
			return c.failure(ParquetUnsupported, errors.New("unsupported page type"))
		}
		if kind == 2 && c.dictionary != nil {
			body.Dec()
			return c.failure(ParquetInvalid, errors.New("duplicate dictionary page"))
		}
		if kind == 2 && c.rows > 0 {
			body.Dec()
			return c.failure(ParquetInvalid, errors.New("late dictionary page"))
		}
		var data []byte
		if kind == 3 {
			v2 := header.field(8)
			if v2.kind != 12 {
				body.Dec()
				return c.failure(ParquetInvalid, errors.New("missing v2 page header"))
			}
			rep, defs := v2.number(6), v2.number(5)
			if rep != 0 || defs < 0 || defs > compressed || defs > uncompressed {
				body.Dec()
				return c.failure(ParquetInvalid, errors.New("invalid v2 levels"))
			}
			compressedValues := v2.field(7).kind == 0 || v2.number(7) != 0
			if c.chunk.codec == 1 && compressedValues {
				if compressed == defs {
					if uncompressed != defs {
						body.Dec()
						return c.failure(ParquetInvalid, errors.New("missing compressed v2 values"))
					}
					c.data = body
					return c.startPage(kind, header, body.AsBytes()[:defs], nil)
				}
				decoded, err := c.decompress(body.AsBytes()[defs:compressed], int(uncompressed-defs))
				if err != nil {
					body.Dec()
					return err
				}
				c.decoded = decoded
				c.data = body
				return c.startPage(kind, header, c.data.AsBytes()[:defs], decoded.AsBytes()[:uncompressed-defs])
			}
			if compressed != uncompressed {
				body.Dec()
				return c.failure(ParquetInvalid, errors.New("v2 page size mismatch"))
			}
			data = body.AsBytes()[:compressed]
		} else if c.chunk.codec == 1 {
			decoded, err := c.decompress(body.AsBytes()[:compressed], int(uncompressed))
			if err != nil {
				body.Dec()
				return err
			}
			c.decoded = decoded
			data = decoded.AsBytes()[:uncompressed]
		} else {
			if compressed != uncompressed {
				body.Dec()
				return c.failure(ParquetInvalid, errors.New("page size mismatch"))
			}
			data = body.AsBytes()[:compressed]
		}
		if kind == 2 {
			dict := header.field(7)
			if dict.kind != 12 || dict.number(2) != 0 || dict.number(1) < 0 || dict.number(1) > int64(c.options.MaxDecodedPageBytes/8) || c.column.typ.ID() != dtype.BOOLT && dict.number(1) > int64(len(data)) || c.column.typ.ID() == dtype.BOOLT && dict.number(1) > int64(len(data))*8 {
				body.Dec()
				c.clearPage()
				return c.failure(ParquetUnsupported, errors.New("unsupported dictionary page"))
			}
			count := int(dict.number(1))
			c.dictCount = count
			c.dictRaw = data
			pos := 0
			if c.column.typ.ID() == dtype.BOOLT {
				pos = (count + 7) / 8
			} else if c.column.typ.ID() == dtype.STRT {
				c.dictOffsets = c.a.AllocSegTemp(count * 8)
				if c.dictOffsets == nil {
					body.Dec()
					c.clearPage()
					return c.failure(ParquetResourceExhausted, errors.New("dictionary offset allocation failed"))
				}
				for i := range count {
					v, n, ok := parquetPlainValue(data[pos:], c.column.typ.ID())
					if !ok || !utf8.Valid(v) {
						body.Dec()
						c.clearPage()
						return c.failure(ParquetInvalid, errors.New("invalid dictionary value"))
					}
					c.dictOffsets.AsI64T()[i] = int64(uint64(uint32(pos+n))<<32 | uint64(uint32(pos+4)))
					pos += n
				}
			} else {
				size := c.column.typ.SlotBits() / 8
				if count > len(data)/size {
					body.Dec()
					c.clearPage()
					return c.failure(ParquetInvalid, errors.New("truncated dictionary"))
				}
				pos = count * size
			}
			if pos != len(data) {
				body.Dec()
				c.clearPage()
				return c.failure(ParquetInvalid, errors.New("dictionary size mismatch"))
			}
			if c.decoded != nil {
				c.dictionary = c.decoded
				c.decoded = nil
				body.Dec()
			} else {
				c.dictionary = body
			}
			continue
		}
		c.data = body
		return c.startPage(kind, header, data, nil)
	}
}

func (c *parquetCursor) decompress(src []byte, n int) (*mem.Segment, *ParquetError) {
	if c.chunk.codec != 1 {
		return nil, c.failure(ParquetUnsupported, errors.New("unsupported codec"))
	}
	length, err := snappy.DecodedLen(src)
	if err != nil || length != n {
		return nil, c.failure(ParquetInvalid, errors.New("invalid Snappy size"))
	}
	seg := c.a.AllocSegTemp(n)
	if seg == nil {
		return nil, c.failure(ParquetResourceExhausted, errors.New("decompression allocation failed"))
	}
	if _, err := snappy.Decode(seg.AsBytes()[:n], src); err != nil {
		seg.Dec()
		return nil, c.failure(ParquetInvalid, err)
	}
	return seg, nil
}

func (c *parquetCursor) startPage(kind int64, header compactValue, data, v2values []byte) *ParquetError {
	info := header.field(5)
	if kind == 3 {
		info = header.field(8)
	}
	if info.kind != 12 {
		return c.failure(ParquetInvalid, errors.New("missing data page header"))
	}
	count := info.number(1)
	if count < 0 || kind == 0 && count == 0 || count > c.chunk.values-c.rows || count > int64(c.options.MaxDecodedPageBytes) {
		return c.failure(ParquetInvalid, errors.New("invalid page row count"))
	}
	c.pageRows, c.pagePos, c.nonNull, c.valuePos = int(count), 0, int(count), 0
	if kind == 3 {
		if info.number(3) != count || info.number(2) < 0 || info.number(2) > count || info.number(6) != 0 || (!c.column.optional && info.number(5) != 0) {
			return c.failure(ParquetInvalid, errors.New("invalid v2 row or level count"))
		}
		c.encoding = info.number(4)
		c.values = v2values
		if v2values == nil {
			c.values = data[info.number(5):]
		}
		data = data[:info.number(5)]
	} else {
		c.encoding = info.number(2)
		if info.number(4) != 3 || c.column.optional && info.number(3) != 3 {
			return c.failure(ParquetUnsupported, errors.New("unsupported level encoding"))
		}
	}
	if c.column.optional {
		var levels []byte
		if kind == 0 {
			if len(data) < 4 {
				return c.failure(ParquetInvalid, errors.New("missing level length"))
			}
			n := int(binary.LittleEndian.Uint32(data))
			if n > len(data)-4 {
				return c.failure(ParquetInvalid, errors.New("level length outside page"))
			}
			levels = data[4 : 4+n]
			data = data[4+n:]
		} else {
			levels = data
		}
		run, used := binary.Uvarint(levels)
		allValid := count > 0 && used > 0 && run&1 == 0 && run>>1 == uint64(count) && len(levels) == used+1 && levels[used] == 1
		if !allValid {
			c.levels = c.a.AllocSegTemp(int(count))
			if c.levels == nil {
				return c.failure(ParquetResourceExhausted, errors.New("level allocation failed"))
			}
			if err := decodeParquetRLE(levels, 1, c.levels.AsBytes()[:count]); err != nil {
				return c.failure(ParquetInvalid, err)
			}
			for _, v := range c.levels.AsBytes()[:count] {
				c.nonNull -= 1 - int(v)
			}
		}
	}
	if kind == 0 {
		c.values = data
	}
	if c.encoding != 0 && c.encoding != 2 && c.encoding != 8 {
		return c.failure(ParquetUnsupported, errors.New("unsupported value encoding"))
	}
	if c.encoding == 2 || c.encoding == 8 {
		if c.dictionary == nil || len(c.values) == 0 {
			return c.failure(ParquetInvalid, errors.New("missing dictionary or indices"))
		}
		width := c.values[0]
		if width > 32 {
			return c.failure(ParquetInvalid, errors.New("invalid index width"))
		}
		c.indices = c.a.AllocSegTemp(c.nonNull * 4)
		if c.indices == nil {
			return c.failure(ParquetResourceExhausted, errors.New("index allocation failed"))
		}
		if err := decodeParquetIndices(c.values[1:], width, c.indices.AsBytes()[:c.nonNull*4]); err != nil {
			return c.failure(ParquetInvalid, err)
		}
		c.values = nil
	}
	if kind == 3 && int64(c.pageRows-c.nonNull) != info.number(2) {
		return c.failure(ParquetInvalid, errors.New("v2 null count mismatch"))
	}
	return nil
}

func decodeParquetRLE(buf []byte, width byte, dst []byte) error {
	return decodeParquetRuns(buf, width, len(dst), func(i int, v uint32) { dst[i] = byte(v) })
}

func decodeParquetIndices(buf []byte, width byte, dst []byte) error {
	return decodeParquetRuns(buf, width, len(dst)/4, func(i int, v uint32) { binary.LittleEndian.PutUint32(dst[i*4:], v) })
}

func decodeParquetRuns(buf []byte, width byte, n int, emit func(int, uint32)) error {
	if width > 32 {
		return errors.New("invalid RLE width")
	}
	pos, out := 0, 0
	for out < n {
		header, used := binary.Uvarint(buf[pos:])
		if used <= 0 || header < 2 {
			return errors.New("invalid RLE run")
		}
		pos += used
		if header&1 == 0 {
			count := header >> 1
			bytes := int((width + 7) / 8)
			if count > uint64(n-out) || bytes > len(buf)-pos {
				return errors.New("RLE run exceeds input")
			}
			var value uint32
			for j := range bytes {
				value |= uint32(buf[pos+j]) << (8 * j)
			}
			if width < 32 && value >= uint32(1)<<width {
				return errors.New("RLE value out of range")
			}
			pos += bytes
			for range int(count) {
				emit(out, value)
				out++
			}
		} else {
			groups := header >> 1
			if groups == 0 {
				return errors.New("bit-packed run exceeds output")
			}
			if width != 0 && groups > uint64(len(buf)-pos)/uint64(width) {
				return errors.New("bit-packed run exceeds input")
			}
			bytes := groups * uint64(width)
			limit := n - out
			if groups <= uint64((n-out+7)/8) {
				limit = min(limit, int(groups*8))
			}
			mask := uint32(uint64(1)<<width - 1)
			packed := buf[pos : pos+int(bytes)]
			for j := range limit {
				bit := j * int(width)
				src := packed[bit/8:]
				var word uint64
				if len(src) >= 8 {
					word = binary.LittleEndian.Uint64(src)
				} else {
					for at, value := range src {
						word |= uint64(value) << (8 * at)
					}
				}
				emit(out, uint32(word>>(bit&7))&mask)
				out++
			}
			pos += int(bytes)
		}
	}
	if pos != len(buf) {
		return errors.New("trailing RLE bytes")
	}
	return nil
}
