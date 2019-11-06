package winporep

import (
	"io"
	"os"
)

type Params struct {
	DRGParents int
	WindowSize int
	Stagger    int
}

var DefaultParams = Params{
	DRGParents: 2,
	WindowSize: 1 << 14,
	Stagger:    2,
}

func EncodeFull(seed []byte, dataSize int, Data io.ReadSeeker, Replica io.WriteSeeker) error {
	e := NewEncoder(DefaultParams, seed, dataSize, Data, Replica)
	return e.EncodeFull()
}

func EncodeFiles(seed []byte, Data, Replica string) error {

	df, err := os.Open(Data)
	if err != nil {
		return err
	}
	defer df.Close()

	rf, err := os.OpenFile(Replica, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer rf.Close()

	dfi, err := df.Stat()
	if err != nil {
		return err
	}

	dataSize := int(dfi.Size())

	return EncodeFull(seed, dataSize, df, rf)
}
