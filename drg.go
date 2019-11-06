package winporep

import (
	"crypto/sha256"
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
	h := sha256.New()

	if len(nodes) < 1 {
		// if no parents, use the seed
		h.Write(d.seed)
	} else {
		for _, i := range nodes {
			h.Write(d.Node(i))
		}
	}

	h.Sum(buf)
}
