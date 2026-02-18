package engine

import "sync"

// dataSize denotes the possible sizes for byte slices,
// with the sizes below assuming 64-bit values by default
type dataSize int

const (
	DATASMALL  dataSize = (1 << 11) * 8
	DATAMEDIUM          = (1 << 15) * 8
	DATALARGE           = (1 << 17) * 8
)

// NewTieredDataPool returns a new TieredDataPool
func NewTieredDataPool() *TieredDataPool {
	pSmall := NewDataPool(DATASMALL)
	pMedium := NewDataPool(DATAMEDIUM)
	pLarge := NewDataPool(DATALARGE)
	return &TieredDataPool{
		ps: [3]*DataPool{pSmall, pMedium, pLarge},
	}
}

// TieredDataPool contains three DataPools, from size small to large
type TieredDataPool struct {
	ps [3]*DataPool
}

// Get returns a byte slice of size small, medium or large
func (tadp *TieredDataPool) Get(d dataSize) []byte {
	switch d {
	case DATASMALL:
		return tadp.ps[0].Get()
	case DATAMEDIUM:
		return tadp.ps[1].Get()
	case DATALARGE:
		return tadp.ps[2].Get()
	default:
		return tadp.ps[1].Get()
	}
}

// Put returns a byte slice back to its corresponding pool
func (tadp *TieredDataPool) Put(ad []byte) {
	ad = ad[:cap(ad)]
	switch cap(ad) {
	case int(DATASMALL):
		tadp.ps[0].Put(ad)
	case int(DATAMEDIUM):
		tadp.ps[1].Put(ad)
	case int(DATALARGE):
		tadp.ps[2].Put(ad)
	default:
		tadp.ps[2].Put(ad)
	}
}

// NewDataPool returns a new DataPool, given a dataSize
func NewDataPool(dSize dataSize) *DataPool {
	return &DataPool{
		p: &sync.Pool{
			New: func() any {
				return make([]byte, dSize)
			},
		},
	}
}

// DataPool contains a sync.Pool of byte slices
type DataPool struct {
	p *sync.Pool
}

// Get returns a slice from the pool if available, or allocates a new slice
// if private/shared/victim pools are empty
func (adp *DataPool) Get() []byte {
	return adp.p.Get().([]byte)
}

// Put returns a byte slice back to the pool
func (adp *DataPool) Put(data []byte) {
	adp.p.Put(data)
}
