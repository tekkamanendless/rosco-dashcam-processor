package roscoconv

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/go-audio/audio"
	"github.com/hraban/opus"
)

// globalDecoder is the Opus decoder that we'll be using everywhere.
// For some reason, Opus likes to have all of its packets decoded by the same instance.
var globalDecoder *opus.Decoder

// MakePCM creates an `audio.IntBuffer` instance based on the raw data.
//
// If `rawPCM` is true, then the data will be interpreted as raw PCM data.
// Otherwise, it will be interpreted as Opus data.
func MakePCM(data []byte, rawPCM bool, bitDepth int) (*audio.IntBuffer, error) {
	var intBuffer *audio.IntBuffer

	if rawPCM {
		sampleRate := 8000
		channelCount := 1

		intBuffer = &audio.IntBuffer{
			Format: &audio.Format{
				NumChannels: channelCount,
				SampleRate:  sampleRate,
			},
			SourceBitDepth: bitDepth,
			Data:           make([]int, 0, len(data)),
		}

		bytesPerSample := bitDepth / 8
		sampleCount := len(data) / bytesPerSample

		reader := bytes.NewReader(data)
		for i := 0; i < sampleCount; i++ {
			var value int
			switch bytesPerSample {
			case 1:
				var temporaryValue int8
				err := binary.Read(reader, binary.LittleEndian, &temporaryValue)
				if err != nil {
					return nil, fmt.Errorf("Could not read 8-bit value: %v", err)
				}
				value = int(temporaryValue)
			case 2:
				var temporaryValue int16
				err := binary.Read(reader, binary.LittleEndian, &temporaryValue)
				if err != nil {
					return nil, fmt.Errorf("Could not read 16-bit value: %v", err)
				}
				value = int(temporaryValue)
			default:
				return nil, fmt.Errorf("Unsupported bit depth: %d", bitDepth)
			}

			intBuffer.Data = append(intBuffer.Data, int(value))
		}
	} else {
		sampleRate := 48000
		channelCount := 1

		intBuffer = &audio.IntBuffer{
			Format: &audio.Format{
				NumChannels: channelCount,
				SampleRate:  sampleRate,
			},
			SourceBitDepth: 16,
		}

		var decoder *opus.Decoder
		if globalDecoder == nil {
			var err error
			decoder, err = opus.NewDecoder(sampleRate, channelCount)
			if err != nil {
				return nil, err
			}
			globalDecoder = decoder
		} else {
			decoder = globalDecoder
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
