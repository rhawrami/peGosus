package mem

import (
	"sync"
	"sync/atomic"
)

const (
	TinyDef   int = 5_000     // default for "tiny" pool
	SmallDef  int = 50_000    // default for "small" pool
	MediumDef int = 500_000   // default for "medium" pool
	LargeDef  int = 1_000_000 // default for "large" pool
)

// poolProfile determines the byte length for slices
// allocated from each pool.
type PoolProfile struct {
	tiny   int
	small  int
	medium int
	large  int
}

// NewPoolProfile returns a new custom PoolProfile
func NewPoolProfile(tiny, small, medium, large int) *PoolProfile {
	return &PoolProfile{
		tiny:   tiny,
		small:  small,
		medium: medium,
		large:  large,
	}
}

// NewPoolWithCustomProfile returns a new Pool,
// using custom byte length size parameters.
func NewPoolWithCustomProfile(pp *PoolProfile) *Pool {
	t := newChildPool(pp.tiny)
	s := newChildPool(pp.small)
	m := newChildPool(pp.medium)
	l := newChildPool(pp.large)

	pools := []*childPool{t, s, m, l}

	return &Pool{pools: pools, profile: pp}
}

// NewPool returns a new Pool, using default byte length
// size parameters.
func NewPool() *Pool {
	t := newChildPool(TinyDef)
	s := newChildPool(SmallDef)
	m := newChildPool(MediumDef)
	l := newChildPool(LargeDef)

	pools := []*childPool{t, s, m, l}
	profile := PoolProfile{
		tiny:   TinyDef,
		small:  SmallDef,
		medium: MediumDef,
		large:  LargeDef,
	}

	return &Pool{pools: pools, profile: &profile}
}

// Pool is a thread-safe pool, managing Data objects.
type Pool struct {
	pools   []*childPool
	profile *PoolProfile
}

// GetBlock returns a Block, based on a set of lengths,
// and a constant scale factor.
func (p *Pool) GetBlock(lenProf []uint64, scale int) *Block {
	c := make([]*Data, len(lenProf))

	var tl uint64
	for i := 0; i < len(c); i++ {
		c[i] = p.Get(int(lenProf[i]), int(scale))
		tl += lenProf[i]
	}

	return &Block{chunks: c, totLength: tl}
}

// GetBlockWithByteLengths returns a Block, based on a set of byte
// length profiles; note that this is equivalent to calling GetBlock with
// the scale factor being 1.
func (p *Pool) GetBlockWithByteLengths(bLenProf []uint64) *Block {
	c := make([]*Data, len(bLenProf))

	var tl uint64
	for i := 0; i < len(c); i++ {
		c[i] = p.Get(int(bLenProf[i]), 1)
		tl += bLenProf[i]
	}

	return &Block{chunks: c, totLength: tl}
}

// Get returns a Data object, given a logical length, and
// a scale factor.
//
// e.g., For data for 100 elements of int64 type, Get returns
// a Data object with a buffer length of AT LEAST 800 bytes.
func (p *Pool) Get(length, scale int) *Data {
	bl := length * scale

	if bl <= p.profile.tiny {
		return p.pools[0].Get(uint64(length))
	}

	if bl <= p.profile.small {
		return p.pools[1].Get(uint64(length))
	}

	if bl <= p.profile.medium {
		return p.pools[2].Get(uint64(length))
	}

	if bl <= p.profile.large {
		return p.pools[3].Get(uint64(length))
	}

	// if bl > largest pool, make on-demand Data,
	// belonging to largest pool.
	r := &atomic.Int32{}
	r.Store(1)

	return &Data{
		buff:   make([]byte, bl),
		length: uint64(length),
		ref:    r,
		pool:   p.pools[3],
	}
}

// childPool manages Data objects, all with lengths
// of AT LEAST `size` bytes.
type childPool struct {
	pool *sync.Pool
	size uint64
}

// Size returns the minimum buffer size for Data returned
// by the childPool.
func (p *childPool) Size() uint64 {
	return p.size
}

// Get returns a Data object with logical length `l`.
func (p *childPool) Get(l uint64) *Data {
	d := p.pool.Get().(*Data)
	d.ref.Store(1)

	d.length = l
	d.pool = p

	return d
}

// Put returns `d` back to its pool.
func (p *childPool) Put(d *Data) {
	d.buff = d.buff[:cap(d.buff)]
	p.pool.Put(d)
}

// newChildPool returns a new childPool, which
// allocates Data objects with buffer lengths of
// at least `l bytes.`
func newChildPool(l int) *childPool {
	p := sync.Pool{
		New: func() any {
			return &Data{
				buff: make([]byte, l),
			}
		},
	}
	return &childPool{pool: &p, size: uint64(l)}
}
