package rosco

import (
	"bufio"
	"fmt"
	"io"
)

// ParseReader parses a file using an `io.Reader` instance.
func ParseReader(reader io.Reader, headerOnly bool) (*FileInfo, error) {
	bufferedReader := bufio.NewReader(reader)

	buffer, err := bufferedReader.Peek(4)
	if err != nil {
		return nil, fmt.Errorf("Could not read the first 4 bytes: %v", err)
	}

	if string(buffer) == "SAYS" {
		return ParseReaderXC4(bufferedReader, headerOnly)
	}
	if buffer[0] == 0x14 {
		return ParseReaderXC(bufferedReader, headerOnly)
	}
	return ParseReaderXC4(bufferedReader, headerOnly)
}
