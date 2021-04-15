package hexline

import (
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
)

func Print(contents io.ReadSeeker, byteLimit int64, width int) error {
	return Write(os.Stdout, contents, byteLimit, width)
}

func Write(out io.Writer, contents io.ReadSeeker, byteLimit int64, width int) error {
	bufferSize := 1024 * 1024
	if width > 0 {
		bufferSize = width
	}

	totalBytesRead := int64(0)
	eof := false
	for !eof {
		start := totalBytesRead
		for line := 0; line < 2; line++ {
			contents.Seek(start, 0)
			buffer := make([]byte, bufferSize)

			out.Write([]byte(fmt.Sprintf("0x%06x: ", start)))

			lineBytesRead := 0
			done := false
			for !done {
				bytesRead, err := contents.Read(buffer)
				if err == io.EOF {
					eof = true
					break
				} else if err != nil {
					log.Errorf("Could not read file: %v", err)
					return err
				}
				for i := 0; i < bytesRead; i++ {
					currentByte := buffer[i]
					switch line {
					case 0:
						if currentByte < ' ' || currentByte > '~' {
							out.Write([]byte(".."))
						} else {
							out.Write([]byte(fmt.Sprintf(" %c", currentByte)))
						}
					case 1:
						out.Write([]byte(fmt.Sprintf("%02x", currentByte)))
					}

					lineBytesRead++
					if line == 0 {
						totalBytesRead++
					}
					if width > 0 && lineBytesRead >= width {
						done = true
						break
					}
					if byteLimit > 0 && totalBytesRead >= byteLimit {
						done = true
						break
					}
				}
			}
			out.Write([]byte("\n"))
		}

		if byteLimit > 0 && totalBytesRead >= byteLimit {
			log.Debugf("Reached the byte limit of %d; ending early.", byteLimit)
			break
		}
	}

	return nil
}
