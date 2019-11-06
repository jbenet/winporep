package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	winporep "github.com/jbenet/winporep"
)

type ArgOpts struct {
	Seed     string
	SeedHash []byte
	InFile   string
	OutFile  string
}

func parseArgs() (ArgOpts, error) {
	a := ArgOpts{}

	flag.StringVar(&a.Seed, "seed", "", "random seed to initialize file")

	flag.Parse()

	if a.Seed == "" {
		a.Seed = "WinPoRepFTW!"
	}
	a.SeedHash = winporep.Hash([]byte(a.Seed))

	if len(os.Args) < 3 {
		return a, errors.New("error: must pass in two file paths")
	}

	a.InFile = os.Args[1]
	a.OutFile = os.Args[2]
	return a, nil
}

func run() error {
	a, err := parseArgs()
	if err != nil {
		return err
	}

	return winporep.EncodeFiles(a.SeedHash, a.InFile, a.OutFile)
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
}
