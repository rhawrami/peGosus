package peg

import "github.com/rhawrami/peGosus/pkg/mem"

// MemoryUsage returns synchronized allocator observations across all queries
// using the engine, including retained results and reusable slab capacity.
func (e *Engine) MemoryUsage() mem.AllocatorUsage {
	if e == nil {
		return mem.AllocatorUsage{}
	}
	return e.allocator.Usage()
}
