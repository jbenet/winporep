package winporep

import (
	"errors"
	"io"
	"log"
)

const NodeSize = 32

type Encoder struct {
	Params   Params
	Seed     []byte
	DataSize int
	Data     io.ReadSeeker
	Replica  io.WriteSeeker

	drgs []*WinDrg // lazily constructed
}

// WinDrg is a handle to a DRG that keeps a window number with it.
type WinDrg struct {
	win int
	drg *DRG
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

func (e *Encoder) EncodeFull() error {
	return e.Encode(0, e.NumNodes())
}

func (e *Encoder) Encode(start, end int) error {
	if start < 0 {
		return errors.New("invalid start")
	}
	if end > e.NumNodes() {
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
		log.Printf("Encode window idx: %d/%d/%d - win: %d/%d", start, next, end, w, windows)
		if next >= end {
			break
		}

		// this grabs the k drgs relevant to this window (staggered, usually just 2)
		// (may cause them to be constructed if they haven't been accessed before)
		drgs, err := e.WindowDRGs(w)
		if err != nil {
			return err
		}

		winEnd := (w + 1) * winsize
		if winEnd > end {
			// if window end is greater than whole data end, use data end
			winEnd = end
		}
		// main loop, advances the encoder
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

// Get the DRG for a given index. Most of the time it will be cached.
// On the rare case it is the first access, will construct the DRG object.
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

	seed := Hash(nil, e.Seed, wn)
	drgSize := e.Params.WindowSize * e.Params.DRGStagger
	drg := NewDRG(drgSize, e.Params.DRGParents, seed)

	drgh := &WinDrg{window, drg}
	e.drgs[window] = drgh
	return drgh, nil
}

func (e *Encoder) Window(index int) int {
	return index / e.Params.WindowSize
}

func (e *Encoder) DataNode(index int) ([]byte, error) {
	err := SeekNode(e.Data, index)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, NodeSize)
	_, err = e.Data.Read(buf)
	return buf, err
}

func (e *Encoder) WindowDRGs(window int) ([]*WinDrg, error) {
	var err error
	drgs := make([]*WinDrg, e.Params.DRGStagger) // usually just 2

	nw := e.NumWindows()
	for s := 0; s < e.Params.DRGStagger; s++ {
		di := (window - s + nw) % nw
		drgs[s], err = e.DRG(di)
		if err != nil {
			return nil, err
		}
	}
	return drgs, nil
}

func (e *Encoder) NumNodes() int {
	return e.DataSize / NodeSize
}

func (e *Encoder) NumWindows() int {
	nn := e.NumNodes()
	windows := nn / e.Params.WindowSize
	if nn > (windows * e.Params.WindowSize) {
		windows++ // partial last window
	}
	return windows
}

func encodeKeyNode(index, win int, seed []byte, Data io.ReadSeeker, Rep io.WriteSeeker) ([]byte, error) {
	log.Print("encodeKeyNode ", index, win)

	err := SeekNode(Data, index)
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
	// h := Hash(nil, d, seed, wb)
	h := d // for now, just use the keyNode as is. easier decoding

	err = SeekNode(Rep, index)
	if err != nil {
		return nil, err
	}
	_, err = Rep.Write(h)
	return h, err
}

func encodeDataNode(index, win, windows, winsize int, drgs []*WinDrg, Data io.ReadSeeker, Rep io.WriteSeeker) ([]byte, error) {
	if index%4096 == 0 {
		log.Print("encodeDataNode ", index, win, windows, winsize, len(drgs))
	}

	err := SeekNode(Data, index)
	if err != nil {
		return nil, err
	}

	d := make([]byte, NodeSize)
	_, err = io.ReadFull(Data, d)
	if err != nil {
		return nil, err
	}

	for _, drg := range drgs {
		didx := drgIndex(index, drg.win, windows, winsize)
		nd := drg.drg.Node(didx)
		// if index%4096 == 0 {
		// 	log.Printf("encodeDataNode drg %d %d %x", i, didx, nd)
		// }
		XOR(d, nd)
	}

	err = SeekNode(Rep, index)
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

func SeekNode(s io.Seeker, n int) error {
	_, err := s.Seek(int64(n)*NodeSize, io.SeekStart)
	return err
}
