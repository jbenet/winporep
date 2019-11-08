package winporep

import (
	"math/rand"
)

type Node [32]byte

const DRGNodeSize = 32

type DRG struct {
	size    int // number of nodes
	parents int
	seed    []byte
	data    []byte // lazily evaluated populated
	gen     []bool
}

func NewDRG(size int, parents int, seed []byte) *DRG {
	return &DRG{
		size:    size,
		parents: parents,
		seed:    seed,
		data:    make([]byte, size*DRGNodeSize),
		gen:     make([]bool, size),
	}
}

func (d *DRG) Size() int {
	return d.size
}

func (d *DRG) Parents() int {
	return d.parents
}

func (d *DRG) nodeSlice(index int) []byte {
	if index < 0 || index > d.size {
		panic("node slice out of bounds")
	}

	return d.data[index*DRGNodeSize : (index+1)*DRGNodeSize]
}

func (d *DRG) Node(index int) []byte {
	// get the node slice
	ns := d.nodeSlice(index)
	if !d.gen[index] {
		p := d.NodeParents(index)
		d.hashNodes(p, ns) // hash in place
		d.gen[index] = true
	}
	return ns
}

func (d *DRG) NodeParents(index int) []int {
	if index <= 0 {
		return nil
	}

	p := make([]int, d.parents)

	// first parent is always preceding node
	p[0] = index - 1

	// seed rand with index
	r := rand.New(rand.NewSource(int64(index)))

	// generate d.parents - 1 more parents
	for i := 1; i < d.parents; i++ {
		p[i] = r.Int() % index // parents must always be smaller
	}

	return p
}

func (d *DRG) hashNodes(nodes []int, buf []byte) {

	var vals [][]byte
	if len(nodes) < 1 {
		// if no parents, use the seed
		vals = append(vals, d.seed)
	} else {
		for _, i := range nodes {
			vals = append(vals, d.Node(i))
		}
	}

	// FIXME: `Hash()` does *not* work in-place, the underlying `sha256`
	//  implementation `append`s to the input byte stream (see
	//  `sha256.digest.Sum()`). Disturbingly, Go allows then to write to
	//  the underlying `DRG.data` memory beyond the `buf` limit (determined
	//  by `nodeSlice`). This is a *sub-optimal* workaround.
	tmp := make([]byte, 0, DRGNodeSize)
	Hash(tmp, vals...)
	copy(buf, tmp[:DRGNodeSize])
}
