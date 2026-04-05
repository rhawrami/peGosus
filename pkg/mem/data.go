package mem

import (
	"sync/atomic"
)

// Block is a set of zero or more Data objects.
type Block struct {
	chunks    []*Data
	totLength uint64
}

func (b *Block) Chunks() []*Data { return b.chunks }
func (b *Block) Length() uint64  { return b.totLength }

// Duplicate returns a copy of `b` with new underlying data (and buffers);
// think of the returned block as a deep copy of b, with uninitialized bytes.
func (b *Block) Duplicate() *Block {
	nChunks := len(b.chunks)
	c := make([]*Data, nChunks)

	for i := 0; i < nChunks; i++ {
		c[i] = b.chunks[i].pool.Get(b.chunks[i].length)
	}

	return &Block{chunks: c, totLength: b.totLength}
}

// LengthProfile returns the set of logical lengths per chunk. Note that this
// is (often) not the byte length per chunk.
func (b *Block) LengthProfile() []uint64 {
	nChunks := len(b.chunks)
	p := make([]uint64, nChunks)

	for i := 0; i < nChunks; i++ {
		p[i] = b.chunks[i].length
	}

	return p
}

// ByteLengthProfile returns the set of byte lengths per chunk.
// In other words, returned is the set of byte lengths that a Data object's
// respective pool is set to.
func (b *Block) ByteLengthProfile() []uint64 {
	nChunks := len(b.chunks)
	p := make([]uint64, nChunks)

	for i := 0; i < nChunks; i++ {
		p[i] = b.chunks[i].pool.size
	}

	return p
}

// Data represents an untyped block of data.
type Data struct {
	buff   []byte        // buffer
	length uint64        // logical length
	ref    *atomic.Int32 // reference count
	pool   *childPool    // data pool
}

func (d *Data) RefCount() int32 { return d.ref.Load() }
func (d *Data) Length() uint64  { return d.length }

// Increment increments the Data's reference count by 1.
func (d *Data) Increment() {
	d.ref.Add(1)
}

// Decrement decrements the Data's reference count by 1;
// returns true if `d's` reference count becomes 0.
func (d *Data) Decrement() bool {
	if d.ref.Add(-1) == 1 {
		d.pool.Put(d)
		return true
	}

	return false
}

// Put returns `d` back to its respective pool.
func (d *Data) Put() {
	d.pool.Put(d)
}

// Constrict returns the underlying buffer, restricted to
// the Data's length, multiplied by a scale factor.
func (d *Data) Constrict(scale int) []byte {
	return d.buff[:(d.length * uint64(scale))]
}

// Slice returns a slice of the underlying buffer, buff[start:stop].
func (d *Data) Slice(start, stop int) []byte {
	return d.buff[start:stop]
}

// AsI64T returns `buff` as an int64 slice (returns nil if length is zero).
func (d *Data) AsI64T() []int64 {
	return bToI64(d.buff, int(d.length))
}

// AsI32T returns `buff` as an int32 slice (returns nil if length is zero).
func (d *Data) AsI32T() []int32 {
	return bToI32(d.buff, int(d.length))
}

// AsF64T returns `buff` as an float64 slice (returns nil if length is zero).
func (d *Data) AsF64T() []float64 {
	return bToF64(d.buff, int(d.length))
}

// AsF32T returns `buff` as an float32 slice (returns nil if length is zero).
func (d *Data) AsF32T() []float32 {
	return bToF32(d.buff, int(d.length))
}
