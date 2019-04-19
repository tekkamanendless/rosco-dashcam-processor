package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/sirupsen/logrus"
	"github.com/tekkamanendless/rosco-dashcam-processor/rosco"
)

func main() {
	debugValue := flag.Bool("debug", false, "Enable debug output")
	flag.Parse()

	if *debugValue {
		rosco.SetLogLevel(logrus.DebugLevel)
	}

	filename := flag.Args()[0]

	handle, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Could not open file '%s': %v\n", filename, err)
		os.Exit(1)
	}

	info, err := rosco.ParseReader(handle)
	if err != nil {
		fmt.Printf("Could not parse file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Metadata: (%d)\n", len(info.Metadata.Entries))
	for _, entry := range info.Metadata.Entries {
		if entry.Type == rosco.MetadataType4 {
			fmt.Printf("   * %s:\n", entry.Name)
			subMetadata := entry.Value.(*rosco.Metadata)
			for _, subEntry := range subMetadata.Entries {
				fmt.Printf("      * %s = %v\n", subEntry.Name, subEntry.Value)
			}
		} else {
			fmt.Printf("   * %s = %v\n", entry.Name, entry.Value)
		}
	}

	fmt.Printf("Unknown file header data:\n")
	spew.Dump(info.Unknown1)

	streamIDs := info.StreamIDs()
	fmt.Printf("Streams: (%d)\n", len(streamIDs))
	for i, streamID := range streamIDs {
		fmt.Printf("   %d. %s\n", i, streamID)
	}

	for _, streamID := range streamIDs {
		fmt.Printf("Stream: %s\n", streamID)
		chunks := info.ChunksForStreamID(streamID)
		audioDataLength := 0
		videoDataLength := 0
		for _, chunk := range chunks {
			if chunk.Audio != nil {
				for _, channelData := range chunk.Audio.Channels {
					audioDataLength += len(channelData)
				}
			}
			if chunk.Video != nil {
				videoDataLength += len(chunk.Video.Media)
			}
		}
		fmt.Printf("   Chunks: %d\n", len(chunks))
		fmt.Printf("   Audio: %d bytes\n", audioDataLength)
		fmt.Printf("   Video: %d bytes\n", videoDataLength)
	}

	fmt.Printf("Extracting audio data...\n")
	for _, streamID := range streamIDs {
		fmt.Printf("Stream: %s\n", streamID)
		chunks := info.ChunksForStreamID(streamID)

		channelCount := 0
		for _, chunk := range chunks {
			if chunk.Audio != nil {
				if len(chunk.Audio.Channels) > channelCount {
					channelCount = len(chunk.Audio.Channels)
				}
			}
		}

		if channelCount == 0 {
			continue
		}

		{
			leftBytes := []byte{}
			for _, chunk := range chunks {
				if chunk.Audio != nil {
					if len(chunk.Audio.Channels) != channelCount {
						panic("Wrong channel count")
					}

					leftBytes = append(leftBytes, chunk.Audio.Channels[0]...)
				}
			}
			ioutil.WriteFile("/tmp/stream-"+streamID+".raw", leftBytes, 0644)
		}

		{
			intBuffer := &audio.IntBuffer{
				Format: &audio.Format{
					NumChannels: channelCount,
					SampleRate:  8000,
				},
				SourceBitDepth: 8,
			}

			for _, chunk := range chunks {
				if chunk.Audio != nil {
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
			}

			out, err := os.Create("/tmp/stream-" + streamID + ".wav")
			if err != nil {
				panic(fmt.Sprintf("Couldn't create output file: %v", err))
			}
			wavAudioFormat := 1
			wavEncoder := wav.NewEncoder(out, intBuffer.Format.SampleRate, 8, intBuffer.Format.NumChannels, wavAudioFormat)
			wavEncoder.Write(intBuffer)
			wavEncoder.Close()
			out.Close()
		}
	}

	/*
		{
			chunks := info.ChunksForStreamID("01")
			spew.Dump(chunks)
		}
	*/

	spew.Dump(info)
}
