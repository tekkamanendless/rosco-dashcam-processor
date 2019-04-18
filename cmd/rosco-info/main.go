package main

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/tekkamanendless/rosco-dashcam-processor/rosco"
)

func main() {
	filename := os.Args[1]

	handle, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Could not open file '%s': %v\n", filename, err)
		os.Exit(1)
	}

	info, err := rosco.ParseReader(handle)
	if err != nil {
		fmt.Printf("Could not parse file: %v\n", err)
		os.Exit(1)
	}

	spew.Dump(info)
}
