package rosco

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// HeaderSize is the size of the file header.
const HeaderSize int = 0x10000

// Metadata type constants.
const (
	MetadataType1      int8 = 0x01 // Double?
	MetadataTypeString      = 0x02
	MetadataType3           = 0x03 // 32-bit integer?
	MetadataType4           = 0x04 // Sub-metadata
	MetadataType8           = 0x08 // 16-bit integer?
	MetadataType10          = 0x10 // 32-bit integer?
)

// ParseReader parses an NVR file using an `io.Reader` instance.
func ParseReader(reader io.Reader) (*FileInfo, error) {
	buffer := make([]byte, HeaderSize)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("Could not read header: %v", err)
	}

	fileInfo, err := parseFileHeader(bytes.NewReader(buffer))
	if err != nil {
		return nil, fmt.Errorf("Could not parse header: %v", err)
	}

	return fileInfo, nil
}

func parseFileHeader(reader io.Reader) (*FileInfo, error) {
	buffer := make([]byte, 4)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("Could not read file type: %v", err)
	}

	switch string(buffer) {
	case "SAYS":
		fileInfo := &FileInfo{}

		buffer = make([]byte, 32)
		_, err = io.ReadFull(reader, buffer)
		if err != nil {
			return nil, fmt.Errorf("Could not read the unknown data: %v", err)
		}

		buffer = make([]byte, 128)
		_, err = io.ReadFull(reader, buffer)
		if err != nil {
			return nil, fmt.Errorf("Could not read the filename: %v", err)
		}

		fileInfo.Filename = strings.Trim(string(buffer), "\x00")

		var metadataLength int32
		err = binary.Read(reader, binary.LittleEndian, &metadataLength)
		if err != nil {
			return nil, fmt.Errorf("Could not read the metadata length: %v", err)
		}

		fmt.Printf("Metadata length: %v\n", metadataLength)

		buffer = make([]byte, metadataLength)
		_, err = io.ReadFull(reader, buffer)
		if err != nil {
			return nil, fmt.Errorf("Could not read the metadata buffer: %v", err)
		}

		fileInfo.Metadata, err = parseMetadata(bytes.NewReader(buffer))
		if err != nil {
			return nil, fmt.Errorf("Could not parse the metadata: %v", err)
		}

		return fileInfo, nil
	default:
		return nil, fmt.Errorf("Unhandled file type: %s", string(buffer))
	}
}

func parseMetadata(reader io.Reader) (*Metadata, error) {
	metadata := &Metadata{}
	for i := 0; ; i++ {
		var entryType int8
		err := binary.Read(reader, binary.LittleEndian, &entryType)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Could not read the type on entry %d: %v", i, err)
		}

		fmt.Printf("Entry %d: Type: %d\n", i, entryType)

		if entryType == 0 {
			continue
		}

		entry := MetadataEntry{
			Type: entryType,
		}

		for {
			buffer := make([]byte, 1)
			_, err = reader.Read(buffer)
			if err != nil {
				return nil, fmt.Errorf("Could not read the name on entry %d: %v", i, err)
			}
			if buffer[0] == '\x00' {
				break
			}
			entry.Name += string(buffer[0])
		}

		fmt.Printf("Entry %d: Name: %s\n", i, entry.Name)

		switch entryType {
		case MetadataType1:
			var value float64
			err = binary.Read(reader, binary.LittleEndian, &value)
			if err != nil {
				return nil, fmt.Errorf("Could not read the value on entry %d: %v", i, err)
			}
			entry.Value = value
		case MetadataTypeString:
			var length int32
			err = binary.Read(reader, binary.LittleEndian, &length)
			if err != nil {
				return nil, fmt.Errorf("Could not read the value length on entry %d: %v", i, err)
			}

			buffer := make([]byte, length)
			_, err = reader.Read(buffer)
			if err != nil {
				return nil, fmt.Errorf("Could not read the string value on entry %d: %v", i, err)
			}
			entry.Value = strings.Trim(string(buffer), "\x00")
		case MetadataType3:
			var value int32
			err = binary.Read(reader, binary.LittleEndian, &value)
			if err != nil {
				return nil, fmt.Errorf("Could not read the value on entry %d: %v", i, err)
			}
			entry.Value = value
		case MetadataType4:
			var length int32
			err = binary.Read(reader, binary.LittleEndian, &length)
			if err != nil {
				return nil, fmt.Errorf("Could not read the value length on entry %d: %v", i, err)
			}

			buffer := make([]byte, length-4)
			_, err = reader.Read(buffer)
			if err != nil {
				return nil, fmt.Errorf("Could not read the buffer value on entry %d: %v", i, err)
			}
			var subMetadata *Metadata
			subMetadata, err = parseMetadata(bytes.NewReader(buffer))
			if err != nil {
				return nil, fmt.Errorf("Could not read the metadata value on entry %d: %v", i, err)
			}
			entry.Value = subMetadata
		case MetadataType8:
			var value int16
			err = binary.Read(reader, binary.LittleEndian, &value)
			if err != nil {
				return nil, fmt.Errorf("Could not read the value on entry %d: %v", i, err)
			}
			entry.Value = value
		case MetadataType10:
			var value int32
			err := binary.Read(reader, binary.LittleEndian, &value)
			if err != nil {
				return nil, fmt.Errorf("Could not read the value on entry %d: %v", i, err)
			}
			entry.Value = value
		default:
			return nil, fmt.Errorf("Unknown metadata type on entry %d: %v", i, entryType)
		}

		fmt.Printf("Entry %d: Value: %v\n", i, entry.Value)

		metadata.Entries = append(metadata.Entries, entry)
	}
	return metadata, nil
}
