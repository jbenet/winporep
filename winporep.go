package winporep

import (
	"io"
	"os"
)

type Params struct {
	WindowSize int
	DRGParents int
	DRGStagger int
}

var DefaultParams = Params{
	WindowSize: 1 << 14,
	DRGParents: 2,
	DRGStagger: 2,
}

func EncodeFull(seed []byte, dataSize int, Data io.ReadSeeker, Replica io.WriteSeeker) error {
	e := NewEncoder(DefaultParams, seed, dataSize, Data, Replica)
	return e.EncodeFull()
}

func EncodeFiles(seed []byte, Data, Replica string) error {
	_, err := EncodeFilesRet(seed, DefaultParams, Data, Replica)
	return err
}

func EncodeFilesRet(seed []byte, p Params, Data, Replica string) (*Encoder, error) {

	df, err := os.Open(Data)
	if err != nil {
		return nil, err
	}
	defer df.Close()

	rf, err := os.OpenFile(Replica, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, err
	}
	defer rf.Close()

	dfi, err := df.Stat()
	if err != nil {
		return nil, err
	}

	dataSize := int(dfi.Size())

	e := NewEncoder(p, seed, dataSize, df, rf)
	err = e.EncodeFull()
	return e, err
}
