package rosco

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	XCHeaderPacketType    = 0x14
	XCUnknown00PacketType = 0x00
	XCUnknown01PacketType = 0x01
	XCGPSPacketType       = 0x02
	XCAudioPacketType     = 0x03
	XCVideoPacketType     = 0x80
	XCEndPacketType       = 0x06
)

type XCHeaderPacket struct {
	Unknown1  []byte // TODO: Figure this out.
	StartTime time.Time
	EndTime   time.Time
	Unknown2  []byte // TODO: Figure this out.
}

type XCUnknown00Packet struct {
	Unknown1  []byte // TODO: Figure this out.
	Timestamp time.Time
}

type XCUnknown01Packet struct {
	SequenceNumber uint32
}

type XCGPSPacket struct {
	SequenceNumber uint32
	Unknown1       []byte // TODO: Figure this out.
	Latitude       float64
	Longitude      float64
	Unknown2       []byte // TODO: Figure this out.
	Year           int32
	Month          int32
	Day            int32
	Hour           int32
	Minute         int32
	Second         int32
}

type XCAudioPacket struct {
	SequenceNumber uint32
	PayloadSize    int32
	Timestamp      time.Time
	Payload        []byte
}

type XCVideoPacket struct {
	Unknown1     []byte // TODO: Figure this out.
	StreamNumber int8
	Unknown2     []byte // TODO: Figure this out.
	StreamType   int8
	PayloadSize  int32
	Timestamp    time.Time
	Payload      []byte
}

type XCEndPacket struct {
	Number int32
}

// ParseReaderXC parses a DVXC ASD file using a `bufio.Reader` instance.
func ParseReaderXC(reader *bufio.Reader, headerOnly bool) (*FileInfo, error) {
	fileInfo := &FileInfo{
		Filename: "",
		Metadata: &Metadata{
			Entries: []MetadataEntry{
				MetadataEntry{
					Type:  MetadataTypeInt64,
					Name:  "_audioBitDepth",
					Value: int64(16),
				},
			},
		},
		Chunks: []*Chunk{},
	}

	packetType, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if packetType != XCHeaderPacketType {
		return nil, fmt.Errorf("Could not find the header packet")
	}

	headerPacket, err := parseXCHeaderPacket(reader)
	if err != nil {
		return nil, err
	}

	//spew.Dump(headerPacket)

	fileInfo.Filename = fmt.Sprintf("rec-%s-%s-%s.asd", headerPacket.StartTime.Format("20060102"), headerPacket.StartTime.Format("150405"), headerPacket.EndTime.Format("150405"))

	fileInfo.Metadata.Entries = append(fileInfo.Metadata.Entries, MetadataEntry{Type: MetadataTypeInt64, Name: "_duration", Value: headerPacket.EndTime.Unix() - headerPacket.StartTime.Unix()})

	if headerOnly {
		return fileInfo, nil
	}

	var firstTimestampInNanoseconds int64

	done := false
	for !done {
		packetType, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch packetType {
		case XCHeaderPacketType:
			return nil, fmt.Errorf("Unexpected second file header")
		case XCUnknown00PacketType:
			packet, err := parseXCUnknown00Packet(reader)
			if err != nil {
				return nil, fmt.Errorf("Could not parse XCUnknown00Packet: %v", err)
			}
			//spew.Dump(packet)
			logrus.Debugf("Unknown00 packet: %v", packet.Timestamp)
			if firstTimestampInNanoseconds == 0 {
				firstTimestampInNanoseconds = packet.Timestamp.UnixNano()
			}
		case XCUnknown01PacketType:
			packet, err := parseXCUnknown01Packet(reader)
			if err != nil {
				return nil, fmt.Errorf("Could not parse XCUnknown01Packet: %v", err)
			}
			//spew.Dump(packet)
			logrus.Debugf("Unknown01 packet: %v", packet.SequenceNumber)
		case XCGPSPacketType:
			packet, err := parseXCGPSPacket(reader)
			if err != nil {
				return nil, fmt.Errorf("Could not parse XCGPSPacket: %v", err)
			}
			//spew.Dump(packet)
			logrus.Debugf("GPS packet: (%f, %f)", packet.Latitude, packet.Longitude)
		case XCAudioPacketType:
			packet, err := parseXCAudioPacket(reader)
			if err != nil {
				return nil, fmt.Errorf("Could not parse XCAudioPacket: %v", err)
			}
			//spew.Dump(packet)
			logrus.Debugf("Audio packet: %d bytes", packet.PayloadSize)
			if firstTimestampInNanoseconds == 0 {
				firstTimestampInNanoseconds = packet.Timestamp.UnixNano()
			}

			chunk := &Chunk{
				ID:   "17",
				Type: "wb",
				Audio: &AudioChunk{
					Timestamp: uint32((packet.Timestamp.UnixNano() - firstTimestampInNanoseconds) / 1000),
					Media:     packet.Payload,
				},
			}
			fileInfo.Chunks = append(fileInfo.Chunks, chunk)
		case XCVideoPacketType:
			packet, err := parseXCVideoPacket(reader)
			if err != nil {
				return nil, fmt.Errorf("Could not parse XCVideoPacket: %v", err)
			}
			//spew.Dump(packet)
			logrus.Debugf("Video packet: %d / %d: %d bytes", packet.StreamNumber, packet.StreamType, packet.PayloadSize)
			if firstTimestampInNanoseconds == 0 {
				firstTimestampInNanoseconds = packet.Timestamp.UnixNano()
			}

			chunk := &Chunk{
				ID:   fmt.Sprintf("%d%d", packet.StreamNumber, packet.StreamType),
				Type: "dc",
				Video: &VideoChunk{
					Timestamp: uint32((packet.Timestamp.UnixNano() - firstTimestampInNanoseconds) / 1000),
					Media:     packet.Payload,
				},
			}
			fileInfo.Chunks = append(fileInfo.Chunks, chunk)
		case XCEndPacketType:
			packet, err := parseXCEndPacket(reader)
			if err != nil {
				return nil, fmt.Errorf("Could not parse XCEndPacket: %v", err)
			}
			//spew.Dump(packet)
			logrus.Debugf("End packet: %d", packet.Number)
			done = true
		default:
			return nil, fmt.Errorf("Unknown packet type: %x", packetType)
		}
	}

	remainingData, err := ioutil.ReadAll(reader)
	if err != nil && err != io.EOF {
		return nil, err
	}
	//spew.Dump(remainingData)
	logrus.Debugf("Remaining data: %d", len(remainingData))

	return fileInfo, nil
}

func parseXCTimestamp(reader *bufio.Reader) (time.Time, error) {
	var timeSeconds uint32
	err := binary.Read(reader, binary.LittleEndian, &timeSeconds)
	if err != nil {
		return time.Time{}, fmt.Errorf("Could not parse timestamp seconds: %v", err)
	}

	var timeMicroseconds uint32
	err = binary.Read(reader, binary.LittleEndian, &timeMicroseconds)
	if err != nil {
		return time.Time{}, fmt.Errorf("Could not parse timestamp microseconds: %v", err)
	}

	return time.Unix(int64(timeSeconds), int64(timeMicroseconds)*1000), nil
}

func parseXCHeaderPacket(reader *bufio.Reader) (*XCHeaderPacket, error) {
	packet := &XCHeaderPacket{}
	packetSize := 0x52 - 1

	buffer := make([]byte, packetSize)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, err
	}

	bufferReader := bufio.NewReader(bytes.NewReader(buffer))

	packet.Unknown1 = make([]byte, 11)
	_, err = io.ReadFull(bufferReader, packet.Unknown1)
	if err != nil {
		return nil, err
	}

	packet.StartTime, err = parseXCTimestamp(bufferReader)
	if err != nil {
		return nil, err
	}

	packet.EndTime, err = parseXCTimestamp(bufferReader)
	if err != nil {
		return nil, err
	}

	packet.Unknown2, err = ioutil.ReadAll(bufferReader)
	if err != nil {
		return nil, err
	}

	return packet, nil
}

func parseXCUnknown00Packet(reader *bufio.Reader) (*XCUnknown00Packet, error) {
	packet := &XCUnknown00Packet{}
	packetSize := 0x16 - 1

	buffer := make([]byte, packetSize)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("Could not read packet contents: %v", err)
	}

	bufferReader := bufio.NewReader(bytes.NewReader(buffer))

	// Read the 0xff byte.
	firstByte, err := bufferReader.ReadByte()
	if err != nil {
		return nil, err
	}
	if firstByte != 0xff {
		return nil, fmt.Errorf("Incorrect first byte: %x", firstByte)
	}

	packet.Unknown1 = make([]byte, 12)
	_, err = io.ReadFull(bufferReader, packet.Unknown1)
	if err != nil {
		return nil, fmt.Errorf("Could not read unknown1 content: %v", err)
	}

	packet.Timestamp, err = parseXCTimestamp(bufferReader)
	if err != nil {
		return nil, fmt.Errorf("Could not parse timestamp: %v", err)
	}

	remainder, err := ioutil.ReadAll(bufferReader)
	if err != nil {
		return nil, err
	}
	if len(remainder) > 0 {
		return nil, fmt.Errorf("Too many extra bytes: %d", len(remainder))
	}

	return packet, nil
}

func parseXCUnknown01Packet(reader *bufio.Reader) (*XCUnknown01Packet, error) {
	packet := &XCUnknown01Packet{}
	packetSize := 0x6 - 1

	buffer := make([]byte, packetSize)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("Could not read packet contents: %v", err)
	}

	bufferReader := bufio.NewReader(bytes.NewReader(buffer))

	// Read the 0xff byte.
	firstByte, err := bufferReader.ReadByte()
	if err != nil {
		return nil, err
	}
	if firstByte != 0xff {
		return nil, fmt.Errorf("Incorrect first byte: %x", firstByte)
	}

	err = binary.Read(bufferReader, binary.LittleEndian, &packet.SequenceNumber)
	if err != nil {
		return nil, err
	}

	return packet, nil
}

func parseXCGPSPacket(reader *bufio.Reader) (*XCGPSPacket, error) {
	packet := &XCGPSPacket{}
	packetSize := 0x5e - 1

	buffer := make([]byte, packetSize)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("Could not read packet contents: %v", err)
	}

	bufferReader := bufio.NewReader(bytes.NewReader(buffer))

	// Read the 0xff byte.
	firstByte, err := bufferReader.ReadByte()
	if err != nil {
		return nil, err
	}
	if firstByte != 0xff {
		return nil, fmt.Errorf("Incorrect first byte: %x", firstByte)
	}

	err = binary.Read(bufferReader, binary.LittleEndian, &packet.SequenceNumber)
	if err != nil {
		return nil, err
	}

	packet.Unknown1 = make([]byte, 23)
	_, err = io.ReadFull(bufferReader, packet.Unknown1)
	if err != nil {
		return nil, err
	}

	stringBuffer := make([]byte, 15)
	_, err = io.ReadFull(bufferReader, stringBuffer)
	if err != nil {
		return nil, err
	}
	packet.Latitude, err = strconv.ParseFloat(strings.Trim(string(stringBuffer), "\x00"), 64)
	if err != nil {
		return nil, err
	}

	_, err = io.ReadFull(bufferReader, stringBuffer)
	if err != nil {
		return nil, err
	}
	packet.Longitude, err = strconv.ParseFloat(strings.Trim(string(stringBuffer), "\x00"), 64)
	if err != nil {
		return nil, err
	}

	packet.Unknown2 = make([]byte, 11)
	_, err = io.ReadFull(bufferReader, packet.Unknown2)
	if err != nil {
		return nil, err
	}

	err = binary.Read(bufferReader, binary.LittleEndian, &packet.Year)
	if err != nil {
		return nil, err
	}
	err = binary.Read(bufferReader, binary.LittleEndian, &packet.Month)
	if err != nil {
		return nil, err
	}
	err = binary.Read(bufferReader, binary.LittleEndian, &packet.Day)
	if err != nil {
		return nil, err
	}
	err = binary.Read(bufferReader, binary.LittleEndian, &packet.Hour)
	if err != nil {
		return nil, err
	}
	err = binary.Read(bufferReader, binary.LittleEndian, &packet.Minute)
	if err != nil {
		return nil, err
	}
	err = binary.Read(bufferReader, binary.LittleEndian, &packet.Second)
	if err != nil {
		return nil, err
	}

	return packet, nil
}

func parseXCAudioPacket(reader *bufio.Reader) (*XCAudioPacket, error) {
	packet := &XCAudioPacket{}
	packetSize := 0x12 - 1

	buffer := make([]byte, packetSize)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("Could not read packet contents: %v", err)
	}

	bufferReader := bufio.NewReader(bytes.NewReader(buffer))

	// Read the 0xff byte.
	firstByte, err := bufferReader.ReadByte()
	if err != nil {
		return nil, err
	}
	if firstByte != 0xff {
		return nil, fmt.Errorf("Incorrect first byte: %x", firstByte)
	}

	err = binary.Read(bufferReader, binary.LittleEndian, &packet.SequenceNumber)
	if err != nil {
		return nil, err
	}

	packet.Timestamp, err = parseXCTimestamp(bufferReader)
	if err != nil {
		return nil, err
	}

	err = binary.Read(bufferReader, binary.LittleEndian, &packet.PayloadSize)
	if err != nil {
		return nil, err
	}

	remainder, err := ioutil.ReadAll(bufferReader)
	if err != nil {
		return nil, err
	}
	if len(remainder) > 0 {
		return nil, fmt.Errorf("Too many extra bytes: %d", len(remainder))
	}

	buffer = make([]byte, packet.PayloadSize)
	_, err = io.ReadFull(reader, buffer)
	if err != nil {
		return nil, err
	}

	packet.Payload = buffer

	return packet, nil
}

func parseXCVideoPacket(reader *bufio.Reader) (*XCVideoPacket, error) {
	packet := &XCVideoPacket{}
	packetSize := 0x14 - 1

	buffer := make([]byte, packetSize)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("Could not read packet contents: %v", err)
	}

	bufferReader := bufio.NewReader(bytes.NewReader(buffer))

	packet.Unknown1 = make([]byte, 3)
	_, err = io.ReadFull(bufferReader, packet.Unknown1)
	if err != nil {
		return nil, err
	}

	err = binary.Read(bufferReader, binary.LittleEndian, &packet.StreamNumber)
	if err != nil {
		return nil, err
	}

	packet.Unknown2 = make([]byte, 2)
	_, err = io.ReadFull(bufferReader, packet.Unknown2)
	if err != nil {
		return nil, err
	}

	err = binary.Read(bufferReader, binary.LittleEndian, &packet.StreamType)
	if err != nil {
		return nil, err
	}

	err = binary.Read(bufferReader, binary.LittleEndian, &packet.PayloadSize)
	if err != nil {
		return nil, err
	}

	packet.Timestamp, err = parseXCTimestamp(bufferReader)
	if err != nil {
		return nil, err
	}

	remainder, err := ioutil.ReadAll(bufferReader)
	if err != nil {
		return nil, err
	}
	if len(remainder) > 0 {
		return nil, fmt.Errorf("Too many extra bytes: %d", len(remainder))
	}

	buffer = make([]byte, packet.PayloadSize)
	_, err = io.ReadFull(reader, buffer)
	if err != nil {
		return nil, err
	}

	packet.Payload = buffer

	return packet, nil
}

func parseXCEndPacket(reader *bufio.Reader) (*XCEndPacket, error) {
	packet := &XCEndPacket{}
	packetSize := 0x6 - 1

	buffer := make([]byte, packetSize)
	_, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, fmt.Errorf("Could not read packet contents: %v", err)
	}

	bufferReader := bufio.NewReader(bytes.NewReader(buffer))

	// Read the 0xff byte.
	firstByte, err := bufferReader.ReadByte()
	if err != nil {
		return nil, err
	}
	if firstByte != 0xff {
		return nil, fmt.Errorf("Incorrect first byte: %x", firstByte)
	}

	err = binary.Read(bufferReader, binary.LittleEndian, &packet.Number)
	if err != nil {
		return nil, err
	}

	return packet, nil
}