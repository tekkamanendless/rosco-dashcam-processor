package rosco

import (
	"fmt"
	"io"
)

// ScanJPEG scans through an entire JFIF stream from start-of-image to end-of-image
// and returns the bytes that it read through, never reading more than necessary.
//
// This function exists because the built-in `image/jpeg` parser reads 4KB chunks
// at a time, which means that it'll read past the image, which isn't good when there
// is additional data in the stream after the JPEG.
func ScanJPEG(reader io.Reader) ([]byte, error) {
	// This is the list of bytes that we read.
	result := make([]byte, 0, 4096) // Assume that we'll need 4KB; this will grow as needed.
	debug := false                  // Set this to true to log a whole lot.

	// Read each segment.
	for {
		// Each segment starts with a marker that consists of 2 bytes: 0xff and some other byte.

		// Read the 0xff.
		buffer := make([]byte, 1)
		count, err := io.ReadFull(reader, buffer)
		if err != nil {
			return nil, err
		}
		if count < 1 {
			return nil, fmt.Errorf("only read %d of %d bytes", count, len(buffer))
		}
		if debug {
			logger.Debugf("Working on byte[%d]: %x", len(result), buffer[0])
		}
		result = append(result, buffer...)
		if buffer[0] != 0xff {
			if debug {
				logger.Debugf("Bytes so far: %x", result)
			}
			return nil, fmt.Errorf("expected ff")
		}

		// Read the next byte.
		count, err = io.ReadFull(reader, buffer)
		if err != nil {
			return nil, err
		}
		if count < 1 {
			return nil, fmt.Errorf("only read %d of %d bytes", count, len(buffer))
		}
		if debug {
			logger.Debugf("Working on byte[%d]: %x", len(result), buffer[0])
		}
		result = append(result, buffer...)
		if buffer[0] == 0 {
			continue
		}

		// Apparently you a marker can start with an arbitrary padding of 0xff, so
		// keep reading unitl we get a byte that's not 0xff.
		for buffer[0] == 0xff {
			count, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, err
			}
			if count < 1 {
				return nil, fmt.Errorf("only read %d of %d bytes", count, len(buffer))
			}
			if debug {
				logger.Debugf("Working on byte[%d]: %x", len(result), buffer[0])
			}
			result = append(result, buffer...)
		}

		// Handle the segment based on the second marker byte.
		switch buffer[0] {
		case 0xd9:
			// End of image; no payload.
			if debug {
				logger.Debugf("End of image: %x", buffer[0])
			}
			return result, nil
		case 0xd8, 0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7:
			// No payload.
			if debug {
				logger.Debugf("No payload: %x", buffer[0])
			}
		case 0xc0, 0xc2, 0xc4, 0xdb, 0xdd, 0xe0, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xfe:
			// Variable payload.
			if debug {
				logger.Debugf("Variable payload: %x", buffer[0])
			}
			buffer = make([]byte, 2)
			count, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, err
			}
			if count < 2 {
				return nil, fmt.Errorf("only read %d of %d bytes", count, len(buffer))
			}
			result = append(result, buffer...)

			length := (int(buffer[0]) << 8) + int(buffer[1])
			if debug {
				logger.Debugf("Length: %d (0x%x)", length, buffer)
			}
			buffer = make([]byte, length-2)
			count, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, err
			}
			if count < length-2 {
				return nil, fmt.Errorf("only read %d of %d bytes", count, len(buffer))
			}
			result = append(result, buffer...)
		case 0xda:
			// Start of scan; variable payload, then data until 0xffd9
			if debug {
				logger.Debugf("Variable payload: %x", buffer[0])
			}
			buffer = make([]byte, 2)
			count, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, err
			}
			if count < 2 {
				return nil, fmt.Errorf("only read %d of %d bytes", count, len(buffer))
			}
			result = append(result, buffer...)

			length := (int(buffer[0]) << 8) + int(buffer[1])
			if debug {
				logger.Debugf("Length: %d (0x%x)", length, buffer)
			}
			buffer = make([]byte, length-2)
			count, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, err
			}
			if count < length-2 {
				return nil, fmt.Errorf("only read %d of %d bytes", count, len(buffer))
			}
			result = append(result, buffer...)

			buffer = make([]byte, 1)
			lastWasFF := false
			for {
				count, err = io.ReadFull(reader, buffer)
				if err != nil {
					return nil, err
				}
				if count < 1 {
					return nil, fmt.Errorf("only read %d of %d bytes", count, len(buffer))
				}
				if debug {
					logger.Debugf("Working on byte[%d]: %x", len(result), buffer[0])
				}
				result = append(result, buffer...)

				if lastWasFF && buffer[0] == 0xd9 {
					return result, nil
				}
				if buffer[0] == 0xff {
					lastWasFF = true
				} else {
					lastWasFF = false
				}
			}
		default:
			return nil, fmt.Errorf("unexpected marker byte: %x", buffer[0])
		}
	}
}
