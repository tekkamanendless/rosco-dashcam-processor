package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tekkamanendless/rosco-dashcam-processor/riff"
	"github.com/tekkamanendless/rosco-dashcam-processor/rosco"
	"github.com/tekkamanendless/rosco-dashcam-processor/roscoconv"
)

func main() {
	debugValue := false

	var rootCommand = &cobra.Command{
		Use:   "rosco",
		Short: "Rosco dashcam video file processor",
		Long: `
This tool processes Rosco dashcam files (typically with the extension ".nvr").
`,
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
			Long: `
The output here isn't particularly pretty, but it should be enough for you to do whatever you need to do with the files.

For a more aggressive output, use the --dump flag.
`,
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
		var byteLimit int
		var debugCommand = &cobra.Command{
			Use:   "debug <filename> <stream> <chunk>",
			Short: "Show debug information from the given file",
			Long: `
The output here isn't particularly pretty, but it should be enough for you to do whatever you need to do with the files.

For a more aggressive output, use the --dump flag.
`,
			Args: cobra.MinimumNArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				if len(args) != 3 {
					cmd.Help()
					os.Exit(1)
				}
				filename := args[0]
				streamID := args[1]
				chunkID := args[2]

				fmt.Printf("File: %s\n", filename)
				info, err := parseFilename(filename, false /*headerOnly*/)
				if err != nil {
					panic(fmt.Sprintf("Error: %v\n", err))
				}

				fmt.Printf("Stream: %s\n", streamID)
				chunks := info.ChunksForStreamID(streamID)
				fmt.Printf("Chunks: %d\n", len(chunks))

				chunkIndex := 0
				{
					value, err := strconv.ParseInt(chunkID, 10, 64)
					if err != nil {
						panic(err)
					}
					chunkIndex = int(value)
				}
				fmt.Printf("Chunk index: %d\n", chunkIndex)
				if chunkIndex < 0 || chunkIndex >= len(chunks) {
					panic(fmt.Sprintf("Invalid chunk index: %d", chunkIndex))
				}

				chunk := chunks[chunkIndex]
				fmt.Printf("ID: %s\n", chunk.ID)
				fmt.Printf("Type: %s\n", chunk.Type)
				if chunk.Audio != nil {
					fmt.Printf("This is an audio chunk.\n")
					fmt.Printf("Timestamp: %v\n", chunk.Audio.Timestamp)
					fmt.Printf("Unknown1: %v\n", chunk.Audio.Unknown1)
					fmt.Printf("Media: (%d)\n", len(chunk.Audio.Media))
					printBinaryData(chunk.Audio.Media, byteLimit)
					if len(chunk.Audio.ExtraMedia) > 0 {
						fmt.Printf("Extra Media: (%d)\n", len(chunk.Audio.ExtraMedia))
						printBinaryData(chunk.Audio.ExtraMedia, byteLimit)
					}
				}
				if chunk.Video != nil {
					fmt.Printf("This is a video chunk.\n")
					fmt.Printf("Codec: %v\n", chunk.Video.Codec)
					fmt.Printf("Unknown1: %v\n", chunk.Video.Unknown1)
					fmt.Printf("Timestamp: %v\n", chunk.Video.Timestamp)
					fmt.Printf("Unknown2: %v\n", chunk.Video.Unknown2)
					printMetadata(chunk.Video.Metadata)
					fmt.Printf("Media: (%d)\n", len(chunk.Video.Media))
					printBinaryData(chunk.Video.Media, byteLimit)
				}
				//fmt.Printf("Length: %d\n", len(chunk))
			},
		}
		debugCommand.Flags().IntVar(&byteLimit, "byte-limit", 120, "The number of bytes to print (when printing raw data); use 0 for no limit")
		rootCommand.AddCommand(debugCommand)
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
				Long: `
This provides a more targeted approach to exporting audio data.
It operates on a single file at a time, allowing you to specify exactly which stream you want to export.
You may also choose the output format.
`,
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

					if len(streamID) == 1 {
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
									streamID = id
									break
								}
							}
						}
					}

					rawPCM := strings.HasSuffix(streamID, "7")

					var intBuffers []*audio.IntBuffer
					for _, chunk := range info.ChunksForStreamID(streamID) {
						if chunk.Audio == nil {
							continue
						}
						intBuffer, err := roscoconv.MakePCM(chunk.Audio.Media, rawPCM)
						if err != nil {
							fmt.Printf("Error: %v\n", err)
							os.Exit(1)
						}
						intBuffers = append(intBuffers, intBuffer)
					}

					if len(intBuffers) == 0 {
						fmt.Printf("Could not find any audio data.\n")
						os.Exit(1)
					}

					intBuffer := &audio.IntBuffer{
						Format: &audio.Format{
							NumChannels: intBuffers[0].Format.NumChannels,
							SampleRate:  intBuffers[0].Format.SampleRate,
						},
						SourceBitDepth: intBuffers[0].SourceBitDepth,
					}
					for _, b := range intBuffers {
						intBuffer.Data = append(intBuffer.Data, b.Data...)
					}

					fmt.Printf("Exporting audio data from stream %s...\n", streamID)

					if intBuffer.Format.NumChannels == 0 {
						fmt.Printf("No audio data in stream.\n")
						os.Exit(1)
					}

					switch format {
					case "raw":
						rawBytes, err := roscoconv.MakeRawAudio(intBuffer)
						if err != nil {
							fmt.Printf("Couldn't create audio buffer: %v\n", err)
							os.Exit(1)
						}
						ioutil.WriteFile(destinationFilename, rawBytes, 0644)
					case "wav":
						out, err := os.Create(destinationFilename)
						if err != nil {
							fmt.Printf("Couldn't create output file: %v\n", err)
							os.Exit(1)
						}
						defer out.Close()

						wavAudioFormat := 0x0001 // PCM
						if rawPCM {
							wavAudioFormat = 0x0007 // mu-law
						}
						wavEncoder := wav.NewEncoder(out, intBuffer.Format.SampleRate, intBuffer.SourceBitDepth, intBuffer.Format.NumChannels, wavAudioFormat)
						fmt.Printf("WAV encoder: Sample rate: %d, Bit Depth: %d, Channels: %d, Format: 0x%x\n", intBuffer.Format.SampleRate, intBuffer.SourceBitDepth, intBuffer.Format.NumChannels, wavAudioFormat)
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
				Long: `
This provides a more targeted approach to exporting video data.
It operates on a single file at a time, allowing you to specify exactly which stream you want to export.
`,
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

					switch format {
					case "avi":
						fmt.Printf("Exporting video data from stream %s...\n", streamID)
						file, err := roscoconv.MakeAVI(info, streamID)
						if err != nil {
							fmt.Printf("Error: %v\n", err)
							os.Exit(1)
						}

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

		{
			outputDirectory := ""
			var exportDvproCommand = &cobra.Command{
				Use:   "dvpro <input-file>[ ...]",
				Short: "Export a video streams from a list of files and/or directories",
				Long: `
The intent of this command is to replicate the functionality from the DV-Pro tools provided by Rosco.
With this, you can quickly export all of the videos from a particular directory or collection of files.
`,
				Args: cobra.MinimumNArgs(1),
				Run: func(cmd *cobra.Command, args []string) {
					inputFiles := []string{}
					for _, arg := range args {
						fileInfo, err := os.Stat(arg)
						if err != nil {
							fmt.Printf("Error: %v\n", err)
							os.Exit(1)
						}
						if fileInfo.IsDir() {
							fileInfos, err := ioutil.ReadDir(arg)
							if err != nil {
								fmt.Printf("Error: %v\n", err)
								os.Exit(1)
							}
							for _, fileInfo := range fileInfos {
								if !fileInfo.IsDir() && strings.HasSuffix(fileInfo.Name(), ".nvr") {
									inputFiles = append(inputFiles, arg+"/"+fileInfo.Name())
								}
							}
						} else {
							inputFiles = append(inputFiles, arg)
						}
					}

					for _, inputFile := range inputFiles {
						info, err := parseFilename(inputFile, false)
						if err != nil {
							fmt.Printf("Error: %v\n", err)
							os.Exit(1)
						}

						logicalStreamIDs := []string{}
						{
							logicalStreamMap := map[string]bool{}
							for _, streamID := range info.StreamIDs() {
								if len(streamID) < 1 {
									continue
								}
								logicalStreamMap[string(streamID[0])] = true
							}
							for id := range logicalStreamMap {
								logicalStreamIDs = append(logicalStreamIDs, id)
							}
							sort.Strings(logicalStreamIDs)
						}

						for streamIndex, streamID := range logicalStreamIDs {
							fmt.Printf("Exporting video data from stream %s...\n", streamID)
							file, err := roscoconv.MakeAVI(info, streamID)
							if err != nil {
								fmt.Printf("Error: %v\n", err)
								os.Exit(1)
							}

							destinationFolder := outputDirectory
							if len(outputDirectory) == 0 {
								destinationFolder = path.Dir(inputFile)
							}
							destinationBaseName := strings.TrimSuffix(path.Base(inputFile), ".nvr")
							destinationFilename := fmt.Sprintf("%s_%d.avi", destinationBaseName, streamIndex+1)
							destinationFullPath := destinationFilename
							if len(destinationFolder) > 0 {
								destinationFullPath = strings.TrimSuffix(destinationFolder, "/") + "/" + destinationFullPath
							}
							fmt.Printf("-> %s\n", destinationFullPath)
							out, err := os.Create(destinationFullPath)
							if err != nil {
								panic(fmt.Sprintf("Couldn't create output file: %v", err))
							}
							defer out.Close()
							err = riff.Write(out, file)
							if err != nil {
								panic(err)
							}
						}
					}
				},
			}
			exportDvproCommand.Flags().StringVar(&outputDirectory, "output-directory", outputDirectory, "The output directory; if not specified, the new files will be created next to the NVR files")
			exportCommand.AddCommand(exportDvproCommand)
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

func printBinaryData(buffer []byte, byteLimit int) {
	for line := 0; line < 2; line++ {
		for i := 0; i < len(buffer); i++ {
			if byteLimit > 0 && i >= byteLimit {
				break
			}

			currentByte := buffer[i]
			switch line {
			case 0:
				if currentByte < ' ' || currentByte > '~' {
					fmt.Printf("..")
				} else {
					fmt.Printf(" %c", currentByte)
				}
			case 1:
				fmt.Printf("%02x", currentByte)
			}
		}
		fmt.Printf("\n")
	}
}

// printMetadata prints metadata.
func printMetadata(metadata *rosco.Metadata) {
	fmt.Printf("Metadata: (%d)\n", len(metadata.Entries))
	for _, entry := range metadata.Entries {
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
}

// printFileInfo prints out the information about the file.
func printFileInfo(info *rosco.FileInfo) {
	printMetadata(info.Metadata)

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
				audioDataLength += len(chunk.Audio.Media)
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
