package riff

import (
	"bytes"
	"encoding/binary"
)

// AVIFile is a RIFF AVI file.
type AVIFile struct {
	Header  AVIHeader
	Streams []Stream
}

// AVIHeader is the AVI header.
type AVIHeader struct {
	MicroSecPerFrame    int32
	MaxBytesPerSec      int32
	PaddingGranularity  int32
	Flags               int32
	TotalFrames         int32
	InitialFrames       int32
	Streams             int32
	SuggestedBufferSize int32
	Width               int32
	Height              int32
	Scale               int32
	Rate                int32
	Start               int32
	Length              int32
}

// These are the AVI flags.
const (
	AVIFlagHasIndex       int32 = 0x00000010 // Index at end of file?
	AVIFlagMustUseIndex         = 0x00000020
	AVIFlagIsInterleaved        = 0x00000100
	AVIFlagTrustCKType          = 0x00000800 // Use CKType to find key frames
	AVIFlagWasCaptureFile       = 0x00010000
	AVIFlagCopyrighted          = 0x00020000
)

// Bytes returns the encoded version of the header.
func (h *AVIHeader) Bytes() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.LittleEndian, *h)
	return buffer.Bytes()
}

// Stream represents a stream.
type Stream struct {
	Header      AVIStreamHeader
	AudioFormat AVIStreamAudioFormat // Used when the type is "auds".
	VideoFormat AVIStreamVideoFormat // Used when the type is "vids".
	Chunks      []Chunk
}

// AVIStreamHeader is the AVI stream header.
type AVIStreamHeader struct {
	Type                [4]byte
	Handler             [4]byte
	Flags               int32
	Priority            int16
	Language            int16
	InitialFrames       int32
	Scale               int32
	Rate                int32 /* dwRate / dwScale == samples/second */
	Start               int32
	Length              int32 /* In units above... */
	SuggestedBufferSize int32
	Quality             int32
	SampleSize          int32
	Width               int16
	Height              int16
}

// Bytes returns the encoded version of the header.
func (h *AVIStreamHeader) Bytes() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.LittleEndian, *h)
	return buffer.Bytes()
}

// AVIStreamAudioFormat is the AVI stream format.
type AVIStreamAudioFormat struct {
	FormatTag      int16
	Channels       int16
	SamplesPerSec  int32
	AvgBytesPerSec int32
	BlockAlign     int16
	BitsPerSample  int16
	Size           int16
}

// Bytes returns the encoded version of the format.
func (h *AVIStreamAudioFormat) Bytes() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.LittleEndian, *h)
	return buffer.Bytes()
}

// AVIStreamVideoFormat is the AVI stream format.
type AVIStreamVideoFormat struct {
	Size          int32
	Width         int32
	Height        int32
	Planes        int16
	BitCount      int16
	Compression   [4]byte
	SizeImage     int32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       int32
	ClrImportant  int32
}

// Bytes returns the encoded version of the format.
func (h *AVIStreamVideoFormat) Bytes() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.LittleEndian, *h)
	return buffer.Bytes()
}

// Chunk represents a chunk.
type Chunk struct {
	ID         string
	Data       []byte
	IsKeyframe bool   // Custom; this is true if we want to index this chunk.
	Timestamp  uint64 // Custom; this helps us order our chunks.
}

// AVIChunkIndex describes a chunk index.
type AVIChunkIndex struct {
	ID          string
	Flags       int32
	ChunkOffset int32
	ChunkLength int32
}

// These are the AVI chunk index flags.
const (
	AVIChunkIndexList     int32 = 0x00000001
	AVIChunkIndexKeyframe       = 0x00000010
	AVIChunkIndexNoTime         = 0x00000100
)
