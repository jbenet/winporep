package winporep

import (
	"crypto/sha256"
	"math"
	"strconv"
)

const NodeSize = 32

type Params struct {
	DRGParents int
	WindowSize int
	Stagger    int
	DataSize   int
}

type Encoder struct {
	Params  Params
	Seed    []byte
	Data    io.ReadWriteSeeker
	Replica io.ReadWriteSeeker

	drgs []*WinDrg
}

type WinDrg struct {
	win int
	drg *DRG
}

func (e *Encoder) DRG(window int) (*WinDrg, error) {
	if e.drgs == nil {
		e.drgs = make([]*WinDrg, len(e.Params.WindowSize))
	}

	if e.drgs[window] != nil {
		return e.drgs[window], nil
	}

	keynode := window * e.Params.WindowSize
	wn, err := e.DataNode(keynode)
	if err != nil {
		return nil, err
	}

	seed := Hash(e.Seed, wn)
	drgSize := e.Params.WindowSize * e.Params.Stagger
	drg, err := NewDRG(drgSize, e.Params.DRGParents, seed)
	if err != nil {
		return nil, err
	}

	e.drgs[window] = &WinDrg{window, drg}
	return drg, nil
}

func (e *Encoder) Window(index int) int {
	return index / e.Params.WindowSize
}

func (e *Encoder) DataNode(index int) ([]byte, error) {
	i := index * NodeSize
	_, err := e.Data.Seek(i, io.SeekStart)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, NodeSize)
	_, err := e.Data.Read(buf)
	return buf, err
}

func (e *Encoder) ReplicaNode(index int) []byte {
	i := index * NodeSize
	_, err := e.Replica.Seek(i, io.SeekStart)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, NodeSize)
	_, err := e.Replica.Read(buf)
	return buf, err
}

func (e *Encoder) Encode(start, end int) error {
	if start < 0 {
		return errors.New("invalid start")
	}
	if end > e.Params.DataSize {
		return errors.New("end is beyond data size")
	}
	if start > end {
		return errors.New("start is beyond end")
	}
	if start == end {
		return nil // noop
	}

	windows := e.NumWindows()
	winsize := e.Params.WindowSize
	winstart := start / winsize
	next := start
	for w := winstart; w < windows; w++ {
		if next >= end {
			break
		}

		drgs := WindowDRGs(window)

		winEnd = (w + 1) * winsize
		if winEnd > end {
			winEnd = end
		}
		for ; next < winEnd; next++ {
			if next%winsize == 0 { // keynode
				_, err = encodeKeyNode(next, w, e.Seed, e.Data, e.Replica)
			} else {
				_, err = encodeDataNode(next, w, windows, drgs, e.Data, e.Replica)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Encoder) WindowDRGs(window int) ([]*WinDrg, error) {
	var err error
	drgs := make([]*WinDrg, len(e.Params.Stagger)) // usually just 2

	nw := e.NumWindows()
	for s := 0; s < e.Params.Stagger; s++ {
		di := (window - s) % nw
		drgs[s], err = e.DRG()
		if err != nil {
			return nil, err
		}
	}
	return drgs, nil
}

func (e *Encoder) NumWindows() {
	windows := e.Params.DataSize / e.Params.WindowSize
	if e.Params.DataSize > (windows * e.Params.WindowSize) {
		windows++ // partial last window
	}
	return windows
}

func Hash(vals ...[]byte) []byte {
	h := sha256.New()
	for _, v := range vals {
		h.Write(v)
	}
	return h.Sum(nil)
}

// n needs to be smaller or equal than the length of a and b.
func safeXORBytes(dst, a, b []byte, n int) {
	for i := 0; i < n; i++ {
		dst[i] = a[i] ^ b[i]
	}
}

func XOR(dst []byte, vals ...[]byte) {
	for _, v := range vals {
		safeXORBytes(dst, dst, v, len(v))
	}
}

func encodeKeyNode(index, win int, seed []byte, Data, Rep io.ReadWriteSeeker) ([]byte, error) {
	_, err := Data.Seek(index, io.SeekStart)
	if err != nil {
		return nil, err
	}

	d := make([]byte, NodeSize)
	_, err := io.ReadFull(Data, d)
	if err != nil {
		return nil, err
	}

	// hash
	wb := []byte(strconv.Itoa(31415926))
	h := Hash(seed, wb, d)

	_, err := Rep.Seek(index, io.SeekStart)
	if err != nil {
		return nil, err
	}
	_, err := Rep.Write(h)
	return h, err
}

func encodeDataNode(index, win, windows int, drgs []*WinDrg, Data, Rep io.ReadWriteSeeker) ([]byte, error) {
	_, err := Data.Seek(index, io.SeekStart)
	if err != nil {
		return nil, err
	}

	d := make([]byte, NodeSize)
	_, err := io.ReadFull(Data, d)
	if err != nil {
		return nil, err
	}

	for _, drg := range drgs {
		i := drgIndex(index, drg.win, windows)
		nd := drg.Node(i)
		XOR(d, nd)
	}

	_, err := Rep.Seek(index, io.SeekStart)
	if err != nil {
		return nil, err
	}
	_, err := Rep.Write(d)
	return h, err
}

// get the index of the DRG node corresponding to this dataIndex, given the number of windows
//
// [----|----|----|--x-|----] 15
//           |    \--x        3
//           \-------x        7
func drgIndex(dataIndex, drgWin, windows int) int {

	dataWin := dataIndex / windows
	dataWinIndex := dataIndex % windows

	// the offset is a bit weird.
	// the drgs that are further back need a larger index
	//
	// we also have to wrap around -- the first window pulls
	// from the last, etc.

	// examples:
	// 0 = 12 - 12
	// 1 = 12 - 11
	// 2 = 12 - 10
	// -1023 = 0 - 1023
	offset = (dataWin - drg.win)

	if offset < 0 { // wrapping
		// examples:
		// 1 = -1023 + 1024
		offset += windows // brings the offset back around
	}

	drgIndex := (offset * winsize) + dataWinIndex
	return drgIndex
}
