package rosco

import (
	"fmt"
	"io"
)

func ScanJPEG(reader io.Reader) ([]byte, error) {
	var result []byte

	for {
		buffer := make([]byte, 1)
		count, err := reader.Read(buffer)
		if err != nil {
			return nil, err
		}
		if count < 1 {
			return nil, fmt.Errorf("read %d bytes", count)
		}
		//logger.Debugf("Working on byte[%d]: %x", len(result), buffer[0])
		result = append(result, buffer...)
		if buffer[0] != 0xff {
			//logger.Debugf("Bytes so far: %x", result)
			return nil, fmt.Errorf("expected ff")
		}

		count, err = reader.Read(buffer)
		if err != nil {
			return nil, err
		}
		if count < 1 {
			return nil, fmt.Errorf("read %d bytes", count)
		}
		//logger.Debugf("Working on byte[%d]: %x", len(result), buffer[0])
		result = append(result, buffer...)
		if buffer[0] == 0 {
			continue
		}
		for buffer[0] == 0xff {
			count, err = reader.Read(buffer)
			if err != nil {
				return nil, err
			}
			if count < 1 {
				return nil, fmt.Errorf("read %d bytes", count)
			}
			//logger.Debugf("Working on byte[%d]: %x", len(result), buffer[0])
			result = append(result, buffer...)
		}
		switch buffer[0] {
		case 0xd9:
			// End of image; no payload.
			//logger.Debugf("End of image: %x", buffer[0])
			return result, nil
		case 0xd8, 0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7:
			// No payload.
			//logger.Debugf("No payload: %x", buffer[0])
		case 0xdd:
			// 4 bytes.
			//logger.Debugf("4 bytes: %x", buffer[0])
			buffer = make([]byte, 4)
			count, err = reader.Read(buffer)
			if err != nil {
				return nil, err
			}
			if count < 4 {
				return nil, fmt.Errorf("read %d bytes", count)
			}
			result = append(result, buffer...)
		case 0xc0, 0xc2, 0xc4, 0xdb, 0xe0, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xfe:
			// Variable payload.
			//logger.Debugf("Variable payload: %x", buffer[0])
			buffer = make([]byte, 2)
			count, err = reader.Read(buffer)
			if err != nil {
				return nil, err
			}
			if count < 2 {
				return nil, fmt.Errorf("read %d bytes", count)
			}
			result = append(result, buffer...)

			length := (int(buffer[0]) << 8) + int(buffer[1])
			//logger.Debugf("Length: %d", length)
			buffer = make([]byte, length-2)
			count, err = reader.Read(buffer)
			if err != nil {
				return nil, err
			}
			if count < length-2 {
				return nil, fmt.Errorf("read %d bytes", count)
			}
			result = append(result, buffer...)
		case 0xda:
			// Start of scan; variable payload, then data until 0xffd9
			//logger.Debugf("Variable payload: %x", buffer[0])
			buffer = make([]byte, 2)
			count, err = reader.Read(buffer)
			if err != nil {
				return nil, err
			}
			if count < 2 {
				return nil, fmt.Errorf("read %d bytes", count)
			}
			result = append(result, buffer...)

			length := (int(buffer[0]) << 8) + int(buffer[1])
			//logger.Debugf("Length: %d", length)
			buffer = make([]byte, length-2)
			count, err = reader.Read(buffer)
			if err != nil {
				return nil, err
			}
			if count < length-2 {
				return nil, fmt.Errorf("read %d bytes", count)
			}
			result = append(result, buffer...)

			buffer = make([]byte, 1)
			lastWasFF := false
			for {
				count, err = reader.Read(buffer)
				if err != nil {
					return nil, err
				}
				if count < 1 {
					return nil, fmt.Errorf("read %d bytes", count)
				}
				//logger.Debugf("Working on byte[%d]: %x", len(result), buffer[0])
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
		}
	}
}
