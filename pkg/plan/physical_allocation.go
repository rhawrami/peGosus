package plan

import "github.com/rhawrami/peGosus/pkg/mem"

func allocOperatorSegment(a *mem.Allocator, scope *mem.AllocationScope, length int) (*mem.Segment, bool) {
	if scope != nil {
		return scope.AllocSeg(length)
	}
	return a.AllocSeg(length), true
}
