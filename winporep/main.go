package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	winporep "github.com/jbenet/winporep"
)

type ArgOpts struct {
	Verbose   bool
	Seed      string
	SeedHash  []byte
	InFile    string
	OutFile   string
	WinParams winporep.Params
}

func parseArgs() (ArgOpts, error) {
	a := ArgOpts{}
	a.WinParams = winporep.DefaultParams

	flag.BoolVar(&a.Verbose, "v", false, "show logging output")
	flag.StringVar(&a.Seed, "seed", "", "random seed to initialize file")

	flag.IntVar(&a.WinParams.WindowSize, "winsize", a.WinParams.WindowSize, "window size")
	flag.IntVar(&a.WinParams.DRGParents, "parents", a.WinParams.DRGParents, "DRG Parents")
	flag.IntVar(&a.WinParams.DRGStagger, "stagger", a.WinParams.DRGStagger, "DRG Stagger")

	flag.Parse()

	if a.Seed == "" {
		a.Seed = "WinPoRepFTW!"
	}
	a.SeedHash = winporep.Hash([]byte(a.Seed))

	if !a.Verbose {
		log.SetOutput(ioutil.Discard)
	}

	// args
	args := flag.Args()
	if len(args) < 2 {
		return a, errors.New("error: must pass in two file paths")
	}

	a.InFile = args[0]
	a.OutFile = args[1]
	return a, nil
}

func prettyPrint(i interface{}) string {
	s, err := json.MarshalIndent(i, "", "\t")
	if err != nil {
		panic(err)
	}
	return string(s)
}

func stringStats(enc *winporep.Encoder, tdiff time.Duration) string {
	w := bytes.NewBuffer(nil)
	fmt.Fprintln(w, "data size:", enc.DataSize)
	fmt.Fprintf(w, "seed: %x\n", enc.Seed)
	fmt.Fprintln(w, "nodes:", enc.NumNodes())
	fmt.Fprintln(w, "windows:", enc.NumWindows())
	fmt.Fprintln(w, "window size:", enc.Params.WindowSize)
	fmt.Fprintln(w, "DRG Parents:", enc.Params.DRGParents)
	fmt.Fprintln(w, "DRG Stagger:", enc.Params.DRGStagger)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "time elapsed:", tdiff)
	fmt.Fprintln(w, "time per window:", time.Duration(int(tdiff)/enc.NumWindows()))
	fmt.Fprintln(w, "")

	hpw := enc.Params.WindowSize * enc.Params.DRGParents * enc.Params.DRGStagger
	hta := 30 * time.Nanosecond
	fmt.Fprintln(w, "target hashes per win:", hpw)
	fmt.Fprintln(w, "target hashes total: ", hpw*enc.NumWindows())
	fmt.Fprintln(w, "actual hashes total: ", winporep.HashCounter)
	fmt.Fprintln(w, "hash time assumption: ", hta)
	fmt.Fprintln(w, "target time per window: ", time.Duration(int(hta)*hpw))
	fmt.Fprintln(w, "target time total: ", time.Duration(hpw*enc.NumWindows()*int(hta)))

	return string(w.Bytes())
}

func run() error {
	a, err := parseArgs()
	if err != nil {
		return err
	}

	fmt.Println("winporep encoding", a.InFile, a.OutFile)

	t1 := time.Now()
	fmt.Println(t1)

	enc, err := winporep.EncodeFilesRet(a.SeedHash, a.WinParams, a.InFile, a.OutFile)

	tdiff := time.Since(t1)
	fmt.Println("winporep done:", stringStats(enc, tdiff))

	return err
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
