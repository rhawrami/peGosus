package plan

import (
	"github.com/rhawrami/peGosus/pkg/dtype"
	"github.com/rhawrami/peGosus/pkg/mem"
)

func (r *packedRows) appendRecord(a *mem.Allocator, stream *spillStream, schema Schema, budget int64) bool {
	if !stream.recordReady || stream.recordColumns != schema.Len() {
		return false
	}
	if r.types == nil {
		r.types = make([]dtype.Type, schema.Len())
		for i, field := range schema.fields {
			r.types[i] = field.Type()
		}
	}
	if len(r.types) != schema.Len() {
		return false
	}
	additional := 0
	maxInt := int(^uint(0) >> 1)
	for col, field := range schema.fields {
		if !r.types[col].Equal(field.Type()) {
			return false
		}
		value := stream.recordValue(schema, col)
		if len(value.text) > maxInt-additional {
			return false
		}
		additional += len(value.text)
	}
	if len(r.types) == 0 {
		r.length++
		return true
	}
	if !r.reserve(a, additional, budget) {
		return false
	}
	slots := r.slots()[r.length*len(r.types) : (r.length+1)*len(r.types)]
	for col := range slots {
		value := stream.recordValue(schema, col)
		slots[col] = packedValue{bits: value.bits}
		if value.IsNull() {
			slots[col].null = 1
		} else if r.types[col].ID() == dtype.STRT {
			slots[col].offset = uint64(r.stringBytes)
			slots[col].length = uint32(len(value.text))
			if len(value.text) != 0 {
				copy(r.strings.AsBytes()[r.stringBytes:], value.text)
			}
			r.stringBytes += len(value.text)
		}
	}
	r.length++
	return true
}
