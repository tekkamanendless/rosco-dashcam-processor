package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	byteLimit := flag.Int("byte-limit", 0, "The number of bytes to read.  If this is 0, then the whole file will be read.")

	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Printf("Missing filename.\n")
		os.Exit(1)
	}
	if len(flag.Args()) > 1 {
		fmt.Printf("Too many arguments.\n")
		os.Exit(1)
	}
	filename := flag.Args()[0]

	log.Infof("Byte limit: %d", *byteLimit)
	log.Infof("Filename: %s", filename)

	handle, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Could not open file '%s': %v\n", filename, err)
		os.Exit(1)
	}

	bufferSize := 1024 * 1024

	for line := 0; line < 2; line++ {
		handle.Seek(0, 0)
		buffer := make([]byte, bufferSize)
		totalBytesRead := 0
		done := false
		for !done {
			bytesRead, err := handle.Read(buffer)
			if err == io.EOF {
				break
			} else if err != nil {
				fmt.Printf("Could not read file: %v\n", err)
				os.Exit(1)
			}
			for i := 0; i < bytesRead; i++ {
				currentByte := buffer[i]
				switch line {
				case 0:
					if currentByte < ' ' || currentByte > '~' {
						fmt.Printf("..")
					} else {
						fmt.Printf(" %c", currentByte)
					}
				case 1:
					fmt.Printf("%02x", currentByte)
				}

				totalBytesRead++
				if *byteLimit > 0 && totalBytesRead >= *byteLimit {
					log.Infof("Reached the byte limit of %d; ending early.", *byteLimit)
					done = true
					break
				}
			}
		}
		fmt.Printf("\n")
	}
	handle.Close()
}
