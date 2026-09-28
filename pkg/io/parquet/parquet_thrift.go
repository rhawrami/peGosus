package parquet

import (
	"encoding/binary"
	"errors"
	"math"
)

var errCompact = errors.New("invalid compact-Thrift data")

type compactValue struct {
	kind    byte
	integer int64
	text    string
	fields  map[int16]compactValue
	items   []compactValue
}

func (v compactValue) field(id int16) compactValue { return v.fields[id] }
func (v compactValue) number(id int16) int64       { return v.field(id).integer }
func (v compactValue) stringField(id int16) string { return v.field(id).text }

type compactDecoder struct {
	buf   []byte
	pos   int
	depth int
}

func (d *compactDecoder) byte() (byte, error) {
	if d.pos >= len(d.buf) {
		return 0, errCompact
	}
	b := d.buf[d.pos]
	d.pos++
	return b, nil
}

func (d *compactDecoder) varint() (uint64, error) {
	var value uint64
	for i := 0; i < 10; i++ {
		b, err := d.byte()
		if err != nil {
			return 0, err
		}
		if i == 9 && b > 1 {
			return 0, errCompact
		}
		value |= uint64(b&127) << (7 * i)
		if b&128 == 0 {
			return value, nil
		}
	}
	return 0, errCompact
}

func (d *compactDecoder) length() (int, error) {
	v, err := d.varint()
	if err != nil || v > uint64(len(d.buf)-d.pos) {
		return 0, errCompact
	}
	return int(v), nil
}

func (d *compactDecoder) decode(kind byte) (compactValue, error) {
	if d.depth >= 32 {
		return compactValue{}, errCompact
	}
	switch kind {
	case 1, 2:
		return compactValue{kind: kind, integer: int64(2 - kind)}, nil
	case 3:
		b, err := d.byte()
		return compactValue{kind: kind, integer: int64(int8(b))}, err
	case 4, 5, 6:
		n, err := d.varint()
		if err != nil {
			return compactValue{}, err
		}
		return compactValue{kind: kind, integer: int64(n>>1) ^ -int64(n&1)}, nil
	case 7:
		if len(d.buf)-d.pos < 8 {
			return compactValue{}, errCompact
		}
		n := binary.LittleEndian.Uint64(d.buf[d.pos:])
		d.pos += 8
		return compactValue{kind: kind, integer: int64(n)}, nil
	case 8:
		n, err := d.length()
		if err != nil {
			return compactValue{}, err
		}
		v := compactValue{kind: kind, text: string(d.buf[d.pos : d.pos+n])}
		d.pos += n
		return v, nil
	case 9, 10:
		h, err := d.byte()
		if err != nil {
			return compactValue{}, err
		}
		n := int(h >> 4)
		if n == 15 {
			x, e := d.varint()
			if e != nil || x > uint64(len(d.buf)-d.pos) || x > math.MaxInt32 {
				return compactValue{}, errCompact
			}
			n = int(x)
		}
		if n > len(d.buf)-d.pos {
			return compactValue{}, errCompact
		}
		d.depth++
		items := make([]compactValue, n)
		for i := range items {
			items[i], err = d.decode(h & 15)
			if err != nil {
				d.depth--
				return compactValue{}, err
			}
		}
		d.depth--
		return compactValue{kind: kind, items: items}, nil
	case 11:
		n, err := d.varint()
		if err != nil || n > uint64(len(d.buf)-d.pos) {
			return compactValue{}, errCompact
		}
		if n == 0 {
			return compactValue{kind: kind}, nil
		}
		h, err := d.byte()
		if err != nil {
			return compactValue{}, err
		}
		d.depth++
		for range n {
			if _, err = d.decode(h >> 4); err != nil {
				break
			}
			if _, err = d.decode(h & 15); err != nil {
				break
			}
		}
		d.depth--
		return compactValue{kind: kind}, err
	case 12:
		d.depth++
		fields := make(map[int16]compactValue)
		var previous int16
		for {
			h, err := d.byte()
			if err != nil {
				d.depth--
				return compactValue{}, err
			}
			if h == 0 {
				break
			}
			id := previous + int16(h>>4)
			if h>>4 == 0 {
				n, e := d.varint()
				if e != nil || n > math.MaxUint16 {
					d.depth--
					return compactValue{}, errCompact
				}
				id = int16(n>>1) ^ -int16(n&1)
			}
			_, exists := fields[id]
			if id <= 0 || exists {
				d.depth--
				return compactValue{}, errCompact
			}
			previous = id
			v, e := d.decode(h & 15)
			if e != nil {
				d.depth--
				return compactValue{}, e
			}
			fields[id] = v
		}
		d.depth--
		return compactValue{kind: kind, fields: fields}, nil
	default:
		return compactValue{}, errCompact
	}
}
