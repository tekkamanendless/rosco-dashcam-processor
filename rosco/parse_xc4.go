package rosco

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/tekkamanendless/rosco-dashcam-processor/hexline"
)

// HeaderSize is the size of the file header.
const HeaderSize int = 0x10000

// Metadata type constants.
const (
	MetadataTypeFloat64 int8 = 0x01
	MetadataTypeString  int8 = 0x02
	MetadataType3       int8 = 0x03 // 32-bit integer?
	MetadataType4       int8 = 0x04 // Sub-metadata
	MetadataType8       int8 = 0x08 // 8-bit integer
	MetadataTypeInt64   int8 = 0x09
	MetadataType10      int8 = 0x10 // 32-bit integer?
)

// ParseReaderXC4 parses a DVXC4 NVR file using an `io.Reader` instance.
func ParseReaderXC4(reader *bufio.Reader, headerOnly bool) (*FileInfo, error) {
	buffer := make([]byte, HeaderSize)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("could not read header: %v", err)
	}

	fileInfo, err := parseXC4FileHeader(bytes.NewReader(buffer))
	if err != nil {
		return nil, fmt.Errorf("could not parse header: %v", err)
	}

	if headerOnly {
		return fileInfo, nil
	}

	// This is the version at which we will assume that the audio format changed.
	// Note that I do not have any proof of this other than the following two
	// data points:
	//    v1.0.0: Audio is encoded as separate left and right channels; length is wrong.
	//    v1.6.5: Audio is encoded as a single mono channel; length is correct.
	Version1Point6 := version.Must(version.NewVersion("v1.6.0"))

	var fileVersion *version.Version
	if fileInfo.Metadata != nil {
		versionString := ""
		for _, entry := range fileInfo.Metadata.Entries {
			if entry.Name == "appVersion" {
				value, okay := entry.Value.(string)
				if !okay {
					logger.Warnf("appVersion is not a string: %v", entry.Value)
				} else {
					versionString = value
				}
			}
		}
		if versionString != "" {
			fileVersion, err = version.NewVersion(versionString)
			if err != nil {
				logger.Warnf("Could not parse version string %q: %v", versionString, err)
			}
		}
	}
	logger.Infof("File version: %v", fileVersion)

	for i := 0; ; i++ {
		buffer, err = reader.Peek(4)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("could not peek the chunk info for chunk %d: %v", i, err)
		}

		chunk := &Chunk{
			ID:   string(buffer[0:2]),
			Type: string(buffer[2:4]),
		}
		logger.Debugf("Chunk[%d]: %s / %s [%x]", i, chunk.ID, chunk.Type, []byte(chunk.ID+chunk.Type))

		switch chunk.Type {
		case "dc":
			// We already peeked at these, so read them for real.
			buffer = make([]byte, 4)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("could not actually read the first 4 bytes of chunk %d: %v", i, err)
			}

			chunk.Video = new(VideoChunk)

			buffer = make([]byte, 4)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("could not read the codec for chunk %d: %v", i, err)
			}
			chunk.Video.Codec = string(buffer)

			logger.Debugf("Codec: %s", chunk.Video.Codec)

			var mediaLength int32
			err = binary.Read(reader, binary.LittleEndian, &mediaLength)
			if err != nil {
				return nil, fmt.Errorf("could not read the media length for chunk %d: %v", i, err)
			}

			logger.Debugf("Media length: %d", mediaLength)

			var metadataLengthSmall int16
			err = binary.Read(reader, binary.LittleEndian, &metadataLengthSmall)
			if err != nil {
				return nil, fmt.Errorf("could not read the (small) metadata length for chunk %d: %v", i, err)
			}

			logger.Debugf("(Small) metadata length: %d", metadataLengthSmall)

			chunk.Video.Unknown1 = make([]byte, 2)
			err = binary.Read(reader, binary.LittleEndian, &chunk.Video.Unknown1)
			if err != nil {
				return nil, fmt.Errorf("could not read unknown1 for chunk %d: %v", i, err)
			}

			logger.Debugf("Unknown1: %d", chunk.Video.Unknown1)

			err = binary.Read(reader, binary.LittleEndian, &chunk.Video.Timestamp)
			if err != nil {
				return nil, fmt.Errorf("could not read timestamp for chunk %d: %v", i, err)
			}

			logger.Debugf("Timestamp: %d", chunk.Video.Timestamp)

			var metadataLength int32
			err = binary.Read(reader, binary.LittleEndian, &metadataLength)
			if err != nil {
				return nil, fmt.Errorf("could not read the (small) metadata length for chunk %d: %v", i, err)
			}

			logger.Debugf("Metadata length: %d", metadataLength)

			metadataLength -= 4

			buffer = make([]byte, metadataLength)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("could not read the metadata buffer: %v", err)
			}

			chunk.Video.Metadata, err = parseXC4Metadata(bytes.NewReader(buffer), false)
			if err != nil {
				return nil, fmt.Errorf("could not parse the metadata: %v", err)
			}

			originalMediaLength := mediaLength
			for mediaLength%8 != 0 {
				mediaLength++
			}

			buffer = make([]byte, mediaLength)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("could not read the media buffer: %v", err)
			}
			chunk.Video.Media = buffer[0:originalMediaLength]
		case "wb":
			// We already peeked at these, so read them for real.
			buffer = make([]byte, 4)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("could not actually read the first 4 bytes of chunk %d: %v", i, err)
			}

			chunk.Audio = new(AudioChunk)

			var audioChannelLength int16
			err = binary.Read(reader, binary.LittleEndian, &audioChannelLength)
			if err != nil {
				return nil, fmt.Errorf("could not read the audio channel length for chunk %d: %v", i, err)
			}
			logger.Debugf("Audio channel length: %d", audioChannelLength)

			var firstAudioChannelLength int16
			err = binary.Read(reader, binary.LittleEndian, &firstAudioChannelLength)
			if err != nil {
				return nil, fmt.Errorf("could not read the first audio channel length for chunk %d: %v", i, err)
			}
			logger.Debugf("First audio channel length: %d", firstAudioChannelLength)

			err = binary.Read(reader, binary.LittleEndian, &chunk.Audio.Timestamp)
			if err != nil {
				return nil, fmt.Errorf("could not read the timestamp for chunk %d: %v", i, err)
			}

			logger.Debugf("Timestamp: %d", chunk.Audio.Timestamp)

			buffer = make([]byte, audioChannelLength)
			_, err = io.ReadFull(reader, buffer)
			if err != nil {
				return nil, fmt.Errorf("could not read the media buffer: %v", err)
			}
			chunk.Audio.Media = buffer

			if fileVersion != nil && fileVersion.LessThan(Version1Point6) {
				logger.Debugf("Reading another %d bytes (second channel)", audioChannelLength)
				buffer = make([]byte, audioChannelLength)
				_, err = io.ReadFull(reader, buffer)
				if err != nil {
					return nil, fmt.Errorf("could not read the media buffer: %v", err)
				}
				chunk.Audio.ExtraMedia = buffer
			}
		case "\xff\xe0":
			img, err := jpeg.Decode(reader)
			if err != nil {
				return nil, fmt.Errorf("could not read the image: %v", err)
			}
			logger.Infof("Image bounds: %v", img.Bounds())
			f, err := os.Create(fmt.Sprintf("/tmp/image-%04d.png", i))
			if err != nil {
				logger.Warnf("Could not create image: %v", err)
			} else {
				png.Encode(f, img)
			}
		default:
			// Attempt to read more data to provide context.
			{
				buffer := make([]byte, 4000)
				readBytes, _ := io.ReadFull(reader, buffer)
				if readBytes > 0 {
					out := &bytes.Buffer{}
					hexline.Write(out, bytes.NewReader(buffer), int64(readBytes), 80)

					logger.Debugf("Next %d bytes:", readBytes)
					for _, line := range strings.Split(out.String(), "\n") {
						logger.Debugf("%s", line)
					}
				}
			}
			return nil, fmt.Errorf("unknown chunk type for chunk %d: %v", i, chunk.Type)
		}

		fileInfo.Chunks = append(fileInfo.Chunks, chunk)
	}

	return fileInfo, nil
}

func parseXC4FileHeader(reader io.Reader) (*FileInfo, error) {
	buffer := make([]byte, 4)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("could not read file type: %v", err)
	}

	switch string(buffer) {
	case "SAYS":
		fileInfo := &FileInfo{}

		buffer = make([]byte, 32)
		_, err = io.ReadFull(reader, buffer)
		if err != nil {
			return nil, fmt.Errorf("could not read the unknown data: %v", err)
		}
		fileInfo.Unknown1 = buffer

		buffer = make([]byte, 128)
		_, err = io.ReadFull(reader, buffer)
		if err != nil {
			return nil, fmt.Errorf("could not read the filename: %v", err)
		}

		fileInfo.Filename = strings.Trim(string(buffer), "\x00")

		var metadataLength int32
		err = binary.Read(reader, binary.LittleEndian, &metadataLength)
		if err != nil {
			return nil, fmt.Errorf("could not read the metadata length: %v", err)
		}

		logger.Debugf("Metadata length: %v", metadataLength)

		buffer = make([]byte, metadataLength)
		_, err = io.ReadFull(reader, buffer)
		if err != nil {
			return nil, fmt.Errorf("could not read the metadata buffer: %v", err)
		}

		fileInfo.Metadata, err = parseXC4Metadata(bytes.NewReader(buffer), true)
		if err != nil {
			return nil, fmt.Errorf("could not parse the metadata: %v", err)
		}

		return fileInfo, nil
	default:
		return nil, fmt.Errorf("unhandled file type: %s", string(buffer))
	}
}

func parseXC4Metadata(reader *bytes.Reader, inFileHeader bool) (*Metadata, error) {
	metadata := &Metadata{}
	for i := 0; ; i++ {
		var entryType int8
		err := binary.Read(reader, binary.LittleEndian, &entryType)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("could not read the type on entry %d: %v", i, err)
		}

		logger.Debugf("Entry %d: Type: %d", i, entryType)

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
				return nil, fmt.Errorf("could not read the name on entry %d: %v", i, err)
			}
			if buffer[0] == '\x00' {
				break
			}
			entry.Name += string(buffer[0])
		}

		//logger.Debugf("Entry %d: Name: %s", i, entry.Name)

		switch entryType {
		case MetadataTypeFloat64:
			var value float64
			err = binary.Read(reader, binary.LittleEndian, &value)
			if err != nil {
				return nil, fmt.Errorf("could not read the value on entry %d: %v", i, err)
			}
			entry.Value = value
		case MetadataTypeString:
			var length int32
			err = binary.Read(reader, binary.LittleEndian, &length)
			if err != nil {
				return nil, fmt.Errorf("could not read the value length on entry %d: %v", i, err)
			}

			buffer := make([]byte, length)
			_, err = reader.Read(buffer)
			if err != nil {
				return nil, fmt.Errorf("could not read the string value on entry %d: %v", i, err)
			}
			entry.Value = strings.Trim(string(buffer), "\x00")
		case MetadataType3:
			var value int32
			err = binary.Read(reader, binary.LittleEndian, &value)
			if err != nil {
				return nil, fmt.Errorf("could not read the value on entry %d: %v", i, err)
			}
			entry.Value = value
		case MetadataType4:
			var length int32
			err = binary.Read(reader, binary.LittleEndian, &length)
			if err != nil {
				return nil, fmt.Errorf("could not read the value length on entry %d: %v", i, err)
			}

			buffer := make([]byte, length-4)
			_, err = reader.Read(buffer)
			if err != nil {
				return nil, fmt.Errorf("could not read the buffer value on entry %d: %v", i, err)
			}
			var subMetadata *Metadata
			subMetadata, err = parseXC4Metadata(bytes.NewReader(buffer), inFileHeader)
			if err != nil {
				return nil, fmt.Errorf("could not read the metadata value on entry %d: %v", i, err)
			}
			entry.Value = subMetadata
		case MetadataType8:
			var value int8
			err = binary.Read(reader, binary.LittleEndian, &value)
			if err != nil {
				return nil, fmt.Errorf("could not read the value on entry %d: %v", i, err)
			}
			entry.Value = value
		case MetadataTypeInt64:
			var value int64
			err = binary.Read(reader, binary.LittleEndian, &value)
			if err != nil {
				return nil, fmt.Errorf("could not read the value on entry %d: %v", i, err)
			}
			entry.Value = value
		case MetadataType10:
			var value int32
			err := binary.Read(reader, binary.LittleEndian, &value)
			if err != nil {
				return nil, fmt.Errorf("could not read the value on entry %d: %v", i, err)
			}
			entry.Value = value
		default:
			return nil, fmt.Errorf("unknown metadata type on entry %d: %v", i, entryType)
		}

		logger.Debugf("Entry %d: [%s] = %v", i, entry.Name, entry.Value)

		metadata.Entries = append(metadata.Entries, entry)
	}
	return metadata, nil
}
