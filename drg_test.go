package winporep

import (
	"bytes"
	"testing"
)

var kSEED = []byte("abcdefghijklmnopqrstuvwxyz012345")

func TestDRG(t *testing.T) {
	l := 1024

	d1 := NewDRG(l, 5, kSEED)
	d2 := NewDRG(l, 5, kSEED)

	for i := 0; i < l; i++ {
		d1.Node(i) // gen fwd
	}
	d2.Node(l - 1) // gen back

	cmpDRG(t, d1, d2)

	t.Fail()
	for i := 0; i < l; i++ {
		t.Log(d1.NodeParents(i))
	}

}

func cmpDRG(t *testing.T, d1, d2 *DRG) bool {
	eq := true
	if d1.Size() != d2.Size() {
		t.Error("d1 and d2 differ in size", d1.Size(), d2.Size())
		eq = false
	}
	if d1.Parents() != d2.Parents() {
		t.Error("d1 and d2 differ in parents", d1.Parents(), d2.Parents())
		eq = false
	}

	for i := 0; i < d1.Size(); i++ {
		if !cmpNodes(d1, d2, i) {
			t.Error("d1 and d2 differ at", i)
			eq = false
		}
	}
	return eq
}

func cmpParents(p1, p2 []int) bool {

	if len(p1) != len(p2) {
		return false
	}

	for i, p := range p1 {
		if p2[i] != p {
			return false
		}
	}

	return true
}

func cmpNodes(d1, d2 *DRG, i int) bool {
	p1 := d1.NodeParents(i)
	p2 := d2.NodeParents(i)
	if !cmpParents(p1, p2) {
		return false
	}

	b1 := d1.Node(i)
	b2 := d2.Node(i)
	return bytes.Equal(b1, b2)
}
