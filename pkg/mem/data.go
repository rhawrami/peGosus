package mem

type Buffer struct {
	buffers []*buffer
	totLen  int
}

type buffer struct {
	data     []byte
	validity []byte
	length   int
	pool     *TieredDataPool
}
