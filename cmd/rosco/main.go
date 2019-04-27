package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
		var infoCommand = &cobra.Command{
			Use:   "info <filename> [...]",
			Short: "Show the information from the given file(s)",
			//Long: ``,
			Args: cobra.MinimumNArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				for _, filename := range args {
					fmt.Printf("File: %s\n", filename)
					info, err := parseFilename(filename)
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
		rootCommand.AddCommand(infoCommand)
	}

	{
		var extractCommand = &cobra.Command{
			Use:   "extract",
			Short: "Extract a stream from a file",
			//Long: ``,
			Run: func(cmd *cobra.Command, args []string) {
				cmd.Help()
				os.Exit(1)
			},
		}
		rootCommand.AddCommand(extractCommand)

		{
			format := "wav"
			var extractAudioCommand = &cobra.Command{
				Use:   "audio <input-file> <stream> <output-file>",
				Short: "Extract an audio stream from a file",
				//Long: ``,
				Args: cobra.ExactArgs(3),
				Run: func(cmd *cobra.Command, args []string) {
					inputFile := args[0]
					streamID := args[1]
					destinationFilename := args[2]

					info, err := parseFilename(inputFile)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						os.Exit(1)
					}

					fmt.Printf("Extracting audio data from stream %s...\n", streamID)
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
						fmt.Printf("No audio data in stream.\n")
						os.Exit(1)
					}

					switch format {
					case "raw":
						leftBytes := []byte{}
						for _, chunk := range chunks {
							if chunk.Audio != nil {
								if len(chunk.Audio.Channels) != channelCount {
									panic("Wrong channel count")
								}

								leftBytes = append(leftBytes, chunk.Audio.Channels[0]...)
							}
						}
						ioutil.WriteFile(destinationFilename, leftBytes, 0644)
					case "wav":
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
			extractAudioCommand.Flags().StringVar(&format, "format", format, "The output file format (can be one of: raw, wav)")
			extractCommand.AddCommand(extractAudioCommand)
		}
	}

	err := rootCommand.Execute()
	if err != nil {
		panic(err)
	}
	os.Exit(0)
}

// parseFilename parses the given file and returns a `FileInfo` instance.
func parseFilename(filename string) (*rosco.FileInfo, error) {
	handle, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Could not open file '%s': %v\n", filename, err)
		return nil, err
	}
	defer handle.Close()

	info, err := rosco.ParseReader(handle)
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
