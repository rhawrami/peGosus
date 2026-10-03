package plan

import (
	"context"
	"os"

	"github.com/rhawrami/peGosus/pkg/mem"
	"github.com/rhawrami/peGosus/pkg/store"
)

func (c *scanCursor) nextSpill(ctx context.Context, a *mem.Allocator) (*store.Batch, bool, error) {
	budget := int64(256 << 20)
	if c.source.spillBudget > 0 {
		budget = c.source.spillBudget
	}
	output := packedRows{}
	defer output.release()
	for {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		if c.spill == nil {
			if c.index == len(c.source.spillPaths) {
				if output.length == 0 {
					return nil, true, nil
				}
				break
			}
			file, err := os.Open(c.source.spillPaths[c.index])
			if err != nil {
				return nil, false, err
			}
			c.spill = makeSpillStream(ctx, a, file, budget)
			if c.spill == nil {
				file.Close()
				return nil, false, errSpillBudget
			}
			c.index++
		}
		if !c.spill.recordReady {
			ready, err := c.spill.nextRecord(a, c.source.schema, budget)
			if err != nil {
				return nil, false, err
			}
			if !ready {
				c.spill.release()
				c.spill = nil
				continue
			}
		}
		charge := int64(c.source.schema.Len()*96) + int64(c.spill.recordLength)*4
		if output.length != 0 && (output.length >= spillBatchRows || output.bytes()*3+charge > budget/8) {
			break
		}
		if output.length == 0 && charge > budget/8 {
			batch, err := c.spill.takeRecord(a, c.source.schema)
			return batch, false, err
		}
		if !output.appendRecord(a, c.spill, c.source.schema, budget/8) {
			if output.length == 0 {
				output.release()
				batch, err := c.spill.takeRecord(a, c.source.schema)
				return batch, false, err
			}
			return nil, false, errSpillBudget
		}
		c.spill.recordReady = false
	}
	batch := output.makeBatch(a, c.source.schema)
	if batch == nil {
		return nil, false, errSpillBudget
	}
	return batch, false, nil
}
