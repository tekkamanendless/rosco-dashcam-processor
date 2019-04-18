package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	filename := os.Args[1]

	handle, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Could not open file '%s': %v\n", filename, err)
		os.Exit(1)
	}

	bufferSize := 1024 * 1024

	for line := 0; line < 2; line++ {
		handle.Seek(0, 0)
		buffer := make([]byte, bufferSize)
		for {
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
			}
		}
		fmt.Printf("\n")
	}
	handle.Close()
}
