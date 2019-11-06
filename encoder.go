package winporep

import (
	"crypto/sha256"
	"errors"
	"io"
)

const NodeSize = 32

type Encoder struct {
	Params   Params
	Seed     []byte
	DataSize int
	Data     io.ReadSeeker
	Replica  io.WriteSeeker

	drgs []*WinDrg
}

func NewEncoder(p Params, seed []byte, datasize int, Data io.ReadSeeker, Replica io.WriteSeeker) *Encoder {
	return &Encoder{
		Params:   p,
		Seed:     seed,
		DataSize: datasize,
		Data:     Data,
		Replica:  Replica,
	}
}

type WinDrg struct {
	win int
	drg *DRG
}

func (e *Encoder) DRG(window int) (*WinDrg, error) {
	if e.drgs == nil {
		e.drgs = make([]*WinDrg, e.NumWindows())
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
	drg := NewDRG(drgSize, e.Params.DRGParents, seed)

	drgh := &WinDrg{window, drg}
	e.drgs[window] = drgh
	return drgh, nil
}

func (e *Encoder) Window(index int) int {
	return index / e.Params.WindowSize
}

func (e *Encoder) DataNode(index int) ([]byte, error) {
	i := index * NodeSize
	_, err := e.Data.Seek(int64(i), io.SeekStart)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, NodeSize)
	_, err = e.Data.Read(buf)
	return buf, err
}

func (e *Encoder) EncodeFull() error {
	return e.Encode(0, e.DataSize)
}

func (e *Encoder) Encode(start, end int) error {
	if start < 0 {
		return errors.New("invalid start")
	}
	if end > e.DataSize {
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

		drgs, err := e.WindowDRGs(w)
		if err != nil {
			return err
		}

		winEnd := (w + 1) * winsize
		if winEnd > end {
			winEnd = end
		}
		for ; next < winEnd; next++ {
			if next%winsize == 0 { // keynode
				_, err = encodeKeyNode(next, w, e.Seed, e.Data, e.Replica)
			} else {
				_, err = encodeDataNode(next, w, windows, winsize, drgs, e.Data, e.Replica)
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
	drgs := make([]*WinDrg, e.Params.Stagger) // usually just 2

	nw := e.NumWindows()
	for s := 0; s < e.Params.Stagger; s++ {
		di := (window - s) % nw
		drgs[s], err = e.DRG(di)
		if err != nil {
			return nil, err
		}
	}
	return drgs, nil
}

func (e *Encoder) NumWindows() int {
	windows := e.DataSize / e.Params.WindowSize
	if e.DataSize > (windows * e.Params.WindowSize) {
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

func encodeKeyNode(index, win int, seed []byte, Data io.ReadSeeker, Rep io.WriteSeeker) ([]byte, error) {
	_, err := Data.Seek(int64(index), io.SeekStart)
	if err != nil {
		return nil, err
	}

	d := make([]byte, NodeSize)
	_, err = io.ReadFull(Data, d)
	if err != nil {
		return nil, err
	}

	// // hash
	// wb := []byte(strconv.Itoa(31415926))
	// h := Hash(seed, wb, d)
	h := d // for now, just use the keyNode as is. easier decoding

	_, err = Rep.Seek(int64(index), io.SeekStart)
	if err != nil {
		return nil, err
	}
	_, err = Rep.Write(h)
	return h, err
}

func encodeDataNode(index, win, windows, winsize int, drgs []*WinDrg, Data io.ReadSeeker, Rep io.WriteSeeker) ([]byte, error) {
	_, err := Data.Seek(int64(index), io.SeekStart)
	if err != nil {
		return nil, err
	}

	d := make([]byte, NodeSize)
	_, err = io.ReadFull(Data, d)
	if err != nil {
		return nil, err
	}

	for _, drg := range drgs {
		i := drgIndex(index, drg.win, windows, winsize)
		nd := drg.drg.Node(i)
		XOR(d, nd)
	}

	_, err = Rep.Seek(int64(index), io.SeekStart)
	if err != nil {
		return nil, err
	}
	_, err = Rep.Write(d)
	return d, err
}

// get the index of the DRG node corresponding to this dataIndex, given the number of windows
//
// [----|----|----|--x-|----] 15
//           |    \--x        3
//           \-------x        7
func drgIndex(dataIndex, drgWin, windows, winsize int) int {

	dataWin := dataIndex / winsize
	dataWinIndex := dataIndex % winsize

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
	offset := (dataWin - drgWin)

	if offset < 0 { // wrapping
		// examples:
		// 1 = -1023 + 1024
		offset += windows // brings the offset back around
	}

	drgIndex := (offset * winsize) + dataWinIndex
	return drgIndex
}
