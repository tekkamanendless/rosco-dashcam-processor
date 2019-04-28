package roscoconv

import (
	"github.com/go-audio/audio"
	"github.com/tekkamanendless/rosco-dashcam-processor/rosco"
)

// MakePCM creates an `audio.IntBuffer` instance based on this `rosco.FileInfo` one.
//
// Stream ID is the ID of the stream to export.
func MakePCM(info *rosco.FileInfo, streamID string) (*audio.IntBuffer, error) {
	chunks := info.ChunksForStreamID(streamID)

	channelCount := 0
	for _, chunk := range chunks {
		if chunk.Audio == nil {
			continue
		}
		if len(chunk.Audio.Channels) > channelCount {
			channelCount = len(chunk.Audio.Channels)
		}
	}

	intBuffer := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: channelCount,
			SampleRate:  8000,
		},
		SourceBitDepth: 8,
	}

	for _, chunk := range chunks {
		if chunk.Audio == nil {
			continue
		}
		if len(chunk.Audio.Channels) != channelCount {
			panic("Wrong channel count")
		}

		dataLength := 0
		for c, channelData := range chunk.Audio.Channels {
			if c == 0 {
				dataLength = len(channelData)
			} else {
				if len(channelData) != dataLength {
					panic("Wrong data length")
				}
			}
		}

		for d := 0; d < dataLength; d++ {
			for c := range chunk.Audio.Channels {
				intBuffer.Data = append(intBuffer.Data, int(chunk.Audio.Channels[c][d]))
			}
		}
	}

	return intBuffer, nil
}
