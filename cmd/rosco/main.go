package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/nareix/joy4/codec/h264parser"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tekkamanendless/rosco-dashcam-processor/riff"
	"github.com/tekkamanendless/rosco-dashcam-processor/rosco"
)

func main() {
	debugValue := false

	var rootCommand = &cobra.Command{
		Use:   "rosco",
		Short: "Rosco dashcam video file processor",
		//Long: ``,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debugValue {
				rosco.SetLogLevel(logrus.DebugLevel)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}
	rootCommand.PersistentFlags().BoolVar(&debugValue, "debug", false, "Enable debug output")

	{
		dumpValue := false
		headerOnlyValue := false
		var infoCommand = &cobra.Command{
			Use:   "info <filename> [...]",
			Short: "Show the information from the given file(s)",
			//Long: ``,
			Args: cobra.MinimumNArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				for _, filename := range args {
					fmt.Printf("File: %s\n", filename)
					info, err := parseFilename(filename, headerOnlyValue)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						continue
					}
					printFileInfo(info)

					if dumpValue {
						spew.Dump(info)
					}
				}
			},
		}
		infoCommand.Flags().BoolVar(&dumpValue, "dump", false, "Dump out everything about the file")
		infoCommand.Flags().BoolVar(&headerOnlyValue, "header-only", false, "Only read the header data")
		rootCommand.AddCommand(infoCommand)
	}

	{
		var exportCommand = &cobra.Command{
			Use:   "export",
			Short: "Export a stream from a file",
			//Long: ``,
			Run: func(cmd *cobra.Command, args []string) {
				cmd.Help()
				os.Exit(1)
			},
		}
		rootCommand.AddCommand(exportCommand)

		{
			format := "wav"
			var exportAudioCommand = &cobra.Command{
				Use:   "audio <input-file> <stream> <output-file>",
				Short: "Export an audio stream from a file",
				//Long: ``,
				Args: cobra.ExactArgs(3),
				Run: func(cmd *cobra.Command, args []string) {
					inputFile := args[0]
					streamID := args[1]
					destinationFilename := args[2]

					// Audio streams appear to be "x7".
					if len(streamID) == 1 {
						streamID += "7"
					}

					info, err := parseFilename(inputFile, false)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						os.Exit(1)
					}

					fmt.Printf("Exporting audio data from stream %s...\n", streamID)
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

					if channelCount == 0 {
						fmt.Printf("No audio data in stream.\n")
						os.Exit(1)
					}

					switch format {
					case "raw":
						rawBytes := []byte{}
						for _, chunk := range chunks {
							if chunk.Audio == nil {
								continue
							}
							if len(chunk.Audio.Channels) != channelCount {
								panic("Wrong channel count")
							}

							rawBytes = append(rawBytes, chunk.Audio.Channels[0]...)
						}
						ioutil.WriteFile(destinationFilename, rawBytes, 0644)
					case "wav":
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

						out, err := os.Create(destinationFilename)
						if err != nil {
							panic(fmt.Sprintf("Couldn't create output file: %v", err))
						}
						defer out.Close()
						wavAudioFormat := 0x0007 // mu-law
						wavEncoder := wav.NewEncoder(out, intBuffer.Format.SampleRate, intBuffer.SourceBitDepth, intBuffer.Format.NumChannels, wavAudioFormat)
						wavEncoder.Write(intBuffer)
						wavEncoder.Close()
					default:
						fmt.Printf("Invalid audio format: %s\n", format)
						os.Exit(1)
					}
				},
			}
			exportAudioCommand.Flags().StringVar(&format, "format", format, "The output file format (can be one of: raw, wav)")
			exportCommand.AddCommand(exportAudioCommand)
		}

		{
			format := "avi"
			var exportVideoCommand = &cobra.Command{
				Use:   "video <input-file> <stream> <output-file>",
				Short: "Export a video stream from a file",
				//Long: ``,
				Args: cobra.ExactArgs(3),
				Run: func(cmd *cobra.Command, args []string) {
					inputFile := args[0]
					streamID := args[1]
					destinationFilename := args[2]

					info, err := parseFilename(inputFile, false)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						os.Exit(1)
					}

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

					fmt.Printf("Exporting video data from stream %s...\n", streamID)
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

					switch format {
					case "avi":
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
							Format: riff.AVIStreamFormat{
								Size:        int32(len(new(riff.AVIStreamFormat).Bytes())),
								Width:       videoWidth,
								Height:      videoHeight,
								Planes:      1,
								BitCount:    24,                          // TODO: Pull this from the metadata.
								Compression: [4]byte{'H', '2', '6', '4'}, // TODO: Pull this from the chunks.
							},
						}
						stream.Format.SizeImage = stream.Format.Width * stream.Format.Height * int32(stream.Format.BitCount) / 8
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
							AVIHeader: riff.AVIHeader{
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

						out, err := os.Create(destinationFilename)
						if err != nil {
							panic(fmt.Sprintf("Couldn't create output file: %v", err))
						}
						defer out.Close()
						err = riff.Write(out, file)
						if err != nil {
							panic(err)
						}
					default:
						fmt.Printf("Invalid video format: %s\n", format)
						os.Exit(1)
					}
				},
			}
			exportVideoCommand.Flags().StringVar(&format, "format", format, "The output file format (can be one of: avi)")
			exportCommand.AddCommand(exportVideoCommand)
		}
	}

	err := rootCommand.Execute()
	if err != nil {
		panic(err)
	}
	os.Exit(0)
}

// parseFilename parses the given file and returns a `FileInfo` instance.
func parseFilename(filename string, headerOnly bool) (*rosco.FileInfo, error) {
	handle, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Could not open file '%s': %v\n", filename, err)
		return nil, err
	}
	defer handle.Close()

	info, err := rosco.ParseReader(handle, headerOnly)
	if err != nil {
		fmt.Printf("Could not parse file: %v\n", err)
		return nil, err
	}

	return info, nil
}

// printFileInfo prints out the information about the file.
func printFileInfo(info *rosco.FileInfo) {
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
}
