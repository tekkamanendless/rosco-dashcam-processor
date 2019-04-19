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
	MetadataTypeFloat64 int8 = 0x01
	MetadataTypeString       = 0x02
	MetadataType3            = 0x03 // 32-bit integer?
	MetadataType4            = 0x04 // Sub-metadata
	MetadataType8            = 0x08 // 16-bit integer?  8-bit integer?  Seems different in file header and chunk metadata...
	MetadataTypeInt64        = 0x09
	MetadataType10           = 0x10 // 32-bit integer?
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

	for i := 0; ; i++ {
		buffer = make([]byte, 2)
		_, err = io.ReadFull(reader, buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Could not read the stream ID for chunk %d: %v", i, err)
		}

		chunk := &Chunk{
			ID: string(buffer),
		}

		buffer = make([]byte, 2)
		_, err = io.ReadFull(reader, buffer)
		if err != nil {
			return nil, fmt.Errorf("Could not read the stream type for chunk %d: %v", i, err)
		}
		chunk.Type = string(buffer)

		fmt.Printf("Chunk: %s / %s\n", chunk.ID, chunk.Type)

		switch chunk.Type {
		case "dc":
			chunk.Video = new(VideoChunk)

			buffer = make([]byte, 4)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("Could not read the codec for chunk %d: %v", i, err)
			}
			chunk.Video.Codec = string(buffer)

			fmt.Printf("Codec: %s\n", chunk.Video.Codec)

			var mediaLength int32
			err = binary.Read(reader, binary.LittleEndian, &mediaLength)
			if err != nil {
				return nil, fmt.Errorf("Could not read the media length for chunk %d: %v", i, err)
			}

			fmt.Printf("Media length: %d\n", mediaLength)

			var metadataLengthSmall int16
			err = binary.Read(reader, binary.LittleEndian, &metadataLengthSmall)
			if err != nil {
				return nil, fmt.Errorf("Could not read the (small) metadata length for chunk %d: %v", i, err)
			}

			fmt.Printf("(Small) metadata length: %d\n", metadataLengthSmall)

			err = binary.Read(reader, binary.LittleEndian, &chunk.Video.Unknown1)
			if err != nil {
				return nil, fmt.Errorf("Could not read unknown1 for chunk %d: %v", i, err)
			}

			fmt.Printf("Unknown1: %d\n", chunk.Video.Unknown1)

			err = binary.Read(reader, binary.LittleEndian, &chunk.Video.Timestamp)
			if err != nil {
				return nil, fmt.Errorf("Could not read timestamp for chunk %d: %v", i, err)
			}

			fmt.Printf("Timestamp: %d\n", chunk.Video.Timestamp)

			err = binary.Read(reader, binary.LittleEndian, &chunk.Video.Unknown2)
			if err != nil {
				return nil, fmt.Errorf("Could not read unknown2 for chunk %d: %v", i, err)
			}

			fmt.Printf("Unknown2: %d\n", chunk.Video.Unknown2)

			var metadataLength int32
			err = binary.Read(reader, binary.LittleEndian, &metadataLength)
			if err != nil {
				return nil, fmt.Errorf("Could not read the (small) metadata length for chunk %d: %v", i, err)
			}

			fmt.Printf("Metadata length: %d\n", metadataLength)

			//metadataLength -= 4

			buffer = make([]byte, metadataLength)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("Could not read the metadata buffer: %v", err)
			}
			fmt.Printf("len(buffer): %d\n", len(buffer))

			chunk.Video.Metadata, err = parseMetadata(bytes.NewReader(buffer), false)
			if err != nil {
				return nil, fmt.Errorf("Could not parse the metadata: %v", err)
			}

			buffer = make([]byte, mediaLength)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("Could not read the media buffer: %v", err)
			}
			chunk.Video.Media = buffer
		case "wb":
			chunk.Audio = new(AudioChunk)

			var audioChannelLength int16
			err = binary.Read(reader, binary.LittleEndian, &audioChannelLength)
			if err != nil {
				return nil, fmt.Errorf("Could not read the audio channel length for chunk %d: %v", i, err)
			}

			var firstAudioChannelLength int16
			err = binary.Read(reader, binary.LittleEndian, &firstAudioChannelLength)
			if err != nil {
				return nil, fmt.Errorf("Could not read the first audio channel length for chunk %d: %v", i, err)
			}

			err = binary.Read(reader, binary.LittleEndian, &chunk.Audio.Timestamp)
			if err != nil {
				return nil, fmt.Errorf("Could not read the timestamp for chunk %d: %v", i, err)
			}

			err = binary.Read(reader, binary.LittleEndian, &chunk.Audio.Unknown1)
			if err != nil {
				return nil, fmt.Errorf("Could not read unknown1 for chunk %d: %v", i, err)
			}

			remainingLength := audioChannelLength + (firstAudioChannelLength - (4 + 4))
			if remainingLength != audioChannelLength*2 {
				return nil, fmt.Errorf("Could not figure out the proper remaining media length for chunk %d: got %d, expected %d", i, remainingLength, audioChannelLength*2)
			}

			buffer = make([]byte, audioChannelLength)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("Could not read the media buffer: %v", err)
			}
			chunk.Audio.Channels = append(chunk.Audio.Channels, buffer)

			buffer = make([]byte, audioChannelLength)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("Could not read the media buffer: %v", err)
			}
			chunk.Audio.Channels = append(chunk.Audio.Channels, buffer)
		default:
			return nil, fmt.Errorf("Unknown chunk type for chunk %d: %v", i, chunk.Type)
		}

		fileInfo.Chunks = append(fileInfo.Chunks, chunk)
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
		fileInfo.Unknown1 = buffer

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

		fileInfo.Metadata, err = parseMetadata(bytes.NewReader(buffer), true)
		if err != nil {
			return nil, fmt.Errorf("Could not parse the metadata: %v", err)
		}

		return fileInfo, nil
	default:
		return nil, fmt.Errorf("Unhandled file type: %s", string(buffer))
	}
}

func parseMetadata(reader io.Reader, inFileHeader bool) (*Metadata, error) {
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

		//fmt.Printf("Entry %d: Name: %s\n", i, entry.Name)

		switch entryType {
		case MetadataTypeFloat64:
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
			subMetadata, err = parseMetadata(bytes.NewReader(buffer), inFileHeader)
			if err != nil {
				return nil, fmt.Errorf("Could not read the metadata value on entry %d: %v", i, err)
			}
			entry.Value = subMetadata
		case MetadataType8:
			if inFileHeader {
				var value int16
				err = binary.Read(reader, binary.LittleEndian, &value)
				if err != nil {
					return nil, fmt.Errorf("Could not read the value on entry %d: %v", i, err)
				}
				entry.Value = value
			} else {
				var value int8
				err = binary.Read(reader, binary.LittleEndian, &value)
				if err != nil {
					return nil, fmt.Errorf("Could not read the value on entry %d: %v", i, err)
				}
				entry.Value = value
			}
		case MetadataTypeInt64:
			var value int64
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

		fmt.Printf("Entry %d: [%s] = %v\n", i, entry.Name, entry.Value)

		metadata.Entries = append(metadata.Entries, entry)
	}
	return metadata, nil
}
