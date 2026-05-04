package mem

// AllocLocation represents where a segment belongs to.
type AllocLocation int

const (
	FromSlabs AllocLocation = iota
	FromPawner
	FromScratch
)

// MakeAllocConfig returns an AllocConfig object.
func MakeAllocConfig(slabProfile []uint64, pawnerSize, scratchSize uint64) AllocConfig {
	return AllocConfig{
		slabProfile: slabProfile,
		pawnerSize:  pawnerSize,
		scratchSize: scratchSize,
	}
}

// AllocConfig configures an initialized Allocator; if pawnerSize (scratchSize)
// is 0, then the underlying Pawner (Scratch) pointer will be nil.
type AllocConfig struct {
	slabProfile []uint64 // byte capacity per slab
	pawnerSize  uint64   // pawner byte capacity
	scratchSize uint64   // scratch byte capacity
}

// MakeAllocatorWithConfig returns an Allocator based on `cfg`.
func MakeAllocatorWithConfig(cfg AllocConfig) *Allocator {
	var pawner *Pawner
	var scratch *Scratch
	if cfg.pawnerSize == 0 {
		pawner = nil
	} else {
		pawner = MakePawner(int(cfg.pawnerSize))
	}
	if cfg.scratchSize == 0 {
		scratch = nil
	} else {
		scratch = MakeScratch(int(cfg.scratchSize))
	}

	slabs := make([]*Slab, len(cfg.slabProfile))
	var capacity uint64
	for i := 0; i < len(cfg.slabProfile); i++ {
		s := MakeSlab(int(cfg.slabProfile[i]))
		capacity += s.capacity
		slabs[i] = s
	}

	return &Allocator{
		on:       0,
		used:     0,
		capacity: capacity,
		slabs:    slabs,
		pawner:   pawner,
		scratch:  scratch,
	}
}

// Allocator allocates and manages memory.
type Allocator struct {
	on       uint64   // offset into slabs slice
	used     uint64   // bytes currently used FOR SLABS
	capacity uint64   // maximum byte capacity FOR SLABS
	slabs    []*Slab  // slabs
	pawner   *Pawner  // slab with fixed 128-byte segments
	scratch  *Scratch // slab for temporary objects
}

// Grow adds another slab with at least `l` bytes to the allocator.
func (a *Allocator) Grow(l int) {
	s := MakeSlab(l)
	a.slabs = append(a.slabs, s)
	a.capacity += s.capacity
}

// Update updates `a`'s metadata; returns true if the metadata changed.
func (a *Allocator) Update() bool {
	var u0, c0, u1, c1 uint64
	u0 = a.used
	c0 = a.capacity
	for _, v := range a.slabs {
		u1 += v.used
		c1 += v.capacity
	}
	a.used = u1
	a.capacity = c1
	return (u0 != u1) || (c0 != c1)
}

// PawnerIsValid returns true if `a`'s Pawner object is non-nil.
func (a *Allocator) PawnerIsValid() bool { return a.pawner != nil }

// ScratchIsValid returns true if `a`'s Scratch object is non-nil.
func (a *Allocator) ScratchIsValid() bool { return a.scratch != nil }

// AllocTiny returns a 128-byte Data object, and where it was allocated from.
// TODO
func (a *Allocator) AllocTiny() (*Data, AllocLocation)

// AllocTemp allocates a Data object that should be temporary,
// and where it was allocated from.
// TODO
func (a *Allocator) AllocTemp(l int) (*Data, AllocLocation)

// Alloc returns a Data object with a single segment of
// at least `l` bytes.
// TODO
func (a *Allocator) Alloc(l int) (*Data, bool)

// AllocWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity.
// TODO
func (a *Allocator) AllocWithProfile(s SizeProfile) (*Data, bool)

// AllocContiguous returns a Data object with segments all coming from
// the same slab, and all contiguous to each other. This allows for
// easier coalescing later on.
// TODO
func (a *Allocator) AllocContiguous(s SizeProfile) (*Data, bool)
