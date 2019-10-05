package roscoconv

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/go-audio/audio"
	"github.com/hraban/opus"
)

// MakePCM creates an `audio.IntBuffer` instance based on the raw data.
//
// If `rawPCM` is true, then the data will be interpreted as raw PCM data.
// Otherwise, it will be interpreted as Opus data.
func MakePCM(data []byte, rawPCM bool) (*audio.IntBuffer, error) {
	sampleRate := 8000
	channelCount := 1

	intBuffer := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: channelCount,
			SampleRate:  sampleRate,
		},
	}

	if rawPCM {
		intBuffer.SourceBitDepth = 8
		intBuffer.Data = make([]int, 0, len(data))

		for _, value := range data {
			intBuffer.Data = append(intBuffer.Data, int(value))
		}
	} else {
		intBuffer.SourceBitDepth = 16

		decoder, err := opus.NewDecoder(sampleRate, channelCount)
		if err != nil {
			return nil, err
		}

		frameSizeMs := 60 // if you don't know, go with 60 ms.
		frameSize := channelCount * frameSizeMs * sampleRate / 1000
		pcm := make([]int16, int(frameSize))
		pcmSize, err := decoder.Decode(data, pcm)
		if err != nil {
			return nil, err
		}

		intBuffer.Data = make([]int, 0, pcmSize)
		for _, value := range pcm {
			intBuffer.Data = append(intBuffer.Data, int(value))
		}
	}

	return intBuffer, nil
}

// MakeRawAudio creates a raw audio stream based on an `IntBuffer`.
func MakeRawAudio(intBuffer *audio.IntBuffer) ([]byte, error) {
	depthInBytes := intBuffer.SourceBitDepth / 8

	buffer := new(bytes.Buffer)
	for _, currentInt := range intBuffer.Data {
		switch depthInBytes {
		case 1:
			newInt := int8(currentInt)
			binary.Write(buffer, binary.LittleEndian, newInt)
		case 2:
			newInt := int16(currentInt)
			binary.Write(buffer, binary.LittleEndian, newInt)
		default:
			return nil, fmt.Errorf("Unsupported bit depth: %d", intBuffer.SourceBitDepth)
		}
	}
	return buffer.Bytes(), nil
}
