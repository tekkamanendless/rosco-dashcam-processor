package roscoconv

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-audio/audio"
	"github.com/hraban/opus"
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

	audioStreamID := streamID
	if len(audioStreamID) == 1 {
		for _, id := range info.StreamIDs() {
			if strings.HasPrefix(id, streamID) {
				audioPresent := false
				for _, chunk := range info.ChunksForStreamID(id) {
					if chunk.Audio != nil {
						audioPresent = true
						break
					}

				}
				if audioPresent {
					audioStreamID = id
					break
				}
			}
		}
	}

	videoChunks := []*rosco.Chunk{}
	for _, id := range streamIDs {
		substreamChunks := info.ChunksForStreamID(id)
		for _, chunk := range substreamChunks {
			if chunk.Video != nil {
				videoChunks = append(videoChunks, chunk)
			}
		}
	}
	sort.Slice(videoChunks, func(i, j int) bool {
		return videoChunks[i].Video.Timestamp < videoChunks[j].Video.Timestamp
	})

	// Figure out the video information.
	var videoWidth int32
	var videoHeight int32
	for _, chunk := range videoChunks {
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
	for chunkIndex, chunk := range videoChunks {
		// Only use the key frames.
		if !strings.HasSuffix(chunk.ID, "0") {
			continue
		}
		firstKeyframeIndex = chunkIndex
		break
	}
	videoChunks = videoChunks[firstKeyframeIndex:]

	videoStream := riff.Stream{
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
	videoStream.VideoFormat.SizeImage = videoStream.VideoFormat.Width * videoStream.VideoFormat.Height * int32(videoStream.VideoFormat.BitCount) / 8
	for _, chunk := range videoChunks {
		if chunk.Video == nil {
			continue
		}
		streamChunk := riff.Chunk{
			ID:         "00dc",
			Data:       chunk.Video.Media,
			IsKeyframe: strings.HasSuffix(chunk.ID, "0"),
			Timestamp:  chunk.Video.Timestamp,
		}
		videoStream.Chunks = append(videoStream.Chunks, streamChunk)
	}
	videoStream.Header.Length = int32(len(videoStream.Chunks))
	file := &riff.AVIFile{
		Header: riff.AVIHeader{
			MicroSecPerFrame:    33333, // TODO: Figure this out somehow.
			MaxBytesPerSec:      0,
			PaddingGranularity:  0,
			Flags:               riff.AVIFlagIsInterleaved | riff.AVIFlagTrustCKType | riff.AVIFlagHasIndex,
			TotalFrames:         int32(len(videoStream.Chunks)),
			InitialFrames:       0,
			Streams:             0,
			SuggestedBufferSize: 65536,
			Width:               videoWidth,
			Height:              videoHeight,
			Scale:               0,
			Rate:                0,
			Start:               0,
			Length:              0,
		},
	}
	file.Streams = append(file.Streams, videoStream)
	file.Header.Streams++

	fmt.Printf("Audio stream ID: %s\n", audioStreamID)

	if strings.HasSuffix(audioStreamID, "7") {
		audioData, err := MakePCM(info, audioStreamID)
		if err != nil {
			return nil, err
		}

		audioStream := riff.Stream{
			Header: riff.AVIStreamHeader{
				Type:                [4]byte{'a', 'u', 'd', 's'},
				Handler:             [4]byte{' ', ' ', ' ', ' '},
				Scale:               1,
				Rate:                int32(audioData.Format.SampleRate),
				SuggestedBufferSize: 65536,
			},
			AudioFormat: riff.AVIStreamAudioFormat{
				FormatTag:      0x0007, // mu-law
				Channels:       int16(audioData.Format.NumChannels),
				SamplesPerSec:  int32(audioData.Format.SampleRate),
				AvgBytesPerSec: int32(audioData.Format.SampleRate * audioData.Format.NumChannels / (audioData.SourceBitDepth / 8)),
				BlockAlign:     int16(audioData.SourceBitDepth / 8 * audioData.Format.NumChannels),
				BitsPerSample:  int16(audioData.SourceBitDepth * audioData.Format.NumChannels),
			},
		}

		var rawBytes []byte
		rawBytes, err = MakeRawAudio(audioData)
		if err != nil {
			return nil, err
		}

		// Break up the audio into smaller chunks.
		// Increments for 1-second chunks.
		timestampIncrement := uint32(1000000)
		offsetIncrement := audioData.Format.SampleRate * audioData.Format.NumChannels * (audioData.SourceBitDepth / 8)
		// Now, break up those increments into smaller increments.
		// The smaller the increment, the better the AVI file ends up working out.
		// From my experiments, 1-second intervals are too large.
		timestampIncrement /= 8
		offsetIncrement /= 8
		// Start the audio with the video using the video's first timestamp.
		currentTimestamp := videoStream.Chunks[0].Timestamp
		for offset := 0; offset < len(rawBytes); offset += offsetIncrement {
			endOffset := offset + offsetIncrement
			if endOffset > len(rawBytes) {
				endOffset = len(rawBytes)
			}
			streamChunk := riff.Chunk{
				ID:        "01wb",
				Data:      rawBytes[offset:endOffset],
				Timestamp: currentTimestamp,
			}
			audioStream.Chunks = append(audioStream.Chunks, streamChunk)
			currentTimestamp += timestampIncrement
		}

		file.Streams = append(file.Streams, audioStream)
		file.Header.Streams++
	} else if strings.HasSuffix(audioStreamID, "9") {
		sampleRate := 8000
		channelCount := 1
		sourceBitDepth := 8

		audioStream := riff.Stream{
			Header: riff.AVIStreamHeader{
				Type:                [4]byte{'a', 'u', 'd', 's'},
				Handler:             [4]byte{' ', ' ', ' ', ' '},
				Scale:               1,
				Rate:                int32(sampleRate),
				SuggestedBufferSize: 65536,
			},
			AudioFormat: riff.AVIStreamAudioFormat{
				FormatTag:      0x0007, // mu-law
				Channels:       int16(channelCount),
				SamplesPerSec:  int32(sampleRate),
				AvgBytesPerSec: int32(sampleRate * channelCount / (sourceBitDepth / 8)),
				BlockAlign:     int16(sourceBitDepth / 8 * channelCount),
				BitsPerSample:  int16(sourceBitDepth * channelCount),
			},
		}

		chunks := info.ChunksForStreamID(audioStreamID)
		for _, chunk := range chunks {
			decoder, err := opus.NewDecoder(sampleRate, channelCount)
			if err != nil {
				return nil, err
			}

			frameSizeMs := 60 // if you don't know, go with 60 ms.
			frameSize := channelCount * frameSizeMs * sampleRate / 1000
			pcm := make([]int16, int(frameSize))
			pcmSize, err := decoder.Decode(chunk.Audio.Channels[0], pcm)
			if err != nil {
				return nil, err
			}

			intBuffer := &audio.IntBuffer{
				Format: &audio.Format{
					NumChannels: channelCount,
					SampleRate:  sampleRate,
				},
				SourceBitDepth: sourceBitDepth,
			}
			for d := 0; d < pcmSize; d++ {
				intBuffer.Data = append(intBuffer.Data, int(pcm[d]))
			}
			var rawBytes []byte
			rawBytes, err = MakeRawAudio(intBuffer)
			if err != nil {
				return nil, err
			}

			streamChunk := riff.Chunk{
				ID:        "01wb",
				Data:      rawBytes,
				Timestamp: chunk.Audio.Timestamp,
			}
			audioStream.Chunks = append(audioStream.Chunks, streamChunk)
		}

		file.Streams = append(file.Streams, audioStream)
		file.Header.Streams++
	}

	return file, nil
}
