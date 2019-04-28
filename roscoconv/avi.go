package roscoconv

import (
	"sort"
	"strings"

	"github.com/nareix/joy4/codec/h264parser"
	"github.com/tekkamanendless/rosco-dashcam-processor/riff"
	"github.com/tekkamanendless/rosco-dashcam-processor/rosco"
)

// MakeAVI creates a `riff.AVIFile` instance based on this `rosco.FileInfo` one.
//
// Stream ID is the ID of the stream to export.
func MakeAVI(info *rosco.FileInfo, streamID string) (*riff.AVIFile, error) {
	streamIDs := []string{}
	for _, id := range info.StreamIDs() {
		if len(streamID) == 1 {
			if strings.HasPrefix(id, streamID) {
				streamIDs = append(streamIDs, id)
			}
		} else if len(streamID) == 2 {
			if id == streamID {
				streamIDs = append(streamIDs, id)
			}
		}
	}

	chunks := []*rosco.Chunk{}
	for _, id := range streamIDs {
		substreamChunks := info.ChunksForStreamID(id)
		for _, chunk := range substreamChunks {
			if chunk.Video == nil {
				continue
			}
			chunks = append(chunks, chunk)
		}
	}
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Video.Timestamp < chunks[j].Video.Timestamp
	})

	// Figure out the video information.
	var videoWidth int32
	var videoHeight int32
	for _, chunk := range chunks {
		// Only use the key frames.
		if !strings.HasSuffix(chunk.ID, "0") {
			continue
		}
		nalus, _ := h264parser.SplitNALUs(chunk.Video.Media)
		if len(nalus) == 0 {
			continue
		}
		for _, nalu := range nalus {
			spsInfo, err := h264parser.ParseSPS(nalu)
			if err != nil {
				continue
			}
			videoWidth = int32(spsInfo.Width)
			videoHeight = int32(spsInfo.Height)
		}
	}

	// Strip out any frames before the first keyframe.  We can't do anything
	// without a keyframe.
	firstKeyframeIndex := 0
	for chunkIndex, chunk := range chunks {
		// Only use the key frames.
		if !strings.HasSuffix(chunk.ID, "0") {
			continue
		}
		firstKeyframeIndex = chunkIndex
		break
	}
	chunks = chunks[firstKeyframeIndex:]

	stream := riff.Stream{
		Header: riff.AVIStreamHeader{
			Type:                [4]byte{'v', 'i', 'd', 's'},
			Handler:             [4]byte{'H', '2', '6', '4'}, // TODO: Pull this from the chunks.
			Scale:               1,
			Rate:                30, // TODO: Pull the frame rate from the chunks.
			SuggestedBufferSize: 65536,
			Width:               int16(videoWidth),
			Height:              int16(videoHeight),
		},
		VideoFormat: riff.AVIStreamVideoFormat{
			Size:        int32(len(new(riff.AVIStreamVideoFormat).Bytes())),
			Width:       videoWidth,
			Height:      videoHeight,
			Planes:      1,
			BitCount:    24,                          // TODO: Pull this from the metadata.
			Compression: [4]byte{'H', '2', '6', '4'}, // TODO: Pull this from the chunks.
		},
	}
	stream.VideoFormat.SizeImage = stream.VideoFormat.Width * stream.VideoFormat.Height * int32(stream.VideoFormat.BitCount) / 8
	for _, chunk := range chunks {
		if chunk.Video == nil {
			continue
		}
		streamChunk := riff.Chunk{
			ID:         "00dc",
			Data:       chunk.Video.Media,
			IsKeyframe: strings.HasSuffix(chunk.ID, "0"),
		}
		stream.Chunks = append(stream.Chunks, streamChunk)
	}
	stream.Header.Length = int32(len(stream.Chunks))
	file := &riff.AVIFile{
		Header: riff.AVIHeader{
			MicroSecPerFrame:    33333, // TODO: Figure this out somehow.
			MaxBytesPerSec:      0,
			PaddingGranularity:  0,
			Flags:               riff.AVIFlagIsInterleaved | riff.AVIFlagTrustCKType | riff.AVIFlagHasIndex,
			TotalFrames:         int32(len(stream.Chunks)),
			InitialFrames:       0,
			Streams:             1,
			SuggestedBufferSize: 65536,
			Width:               videoWidth,
			Height:              videoHeight,
			Scale:               0,
			Rate:                0,
			Start:               0,
			Length:              0,
		},
	}
	file.Streams = append(file.Streams, stream)

	return file, nil
}
