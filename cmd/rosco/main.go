package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/davecgh/go-spew/spew"
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

					// Audio streams appear to be "x7".
					if len(streamID) == 1 {
						streamID += "7"
					}

					info, err := parseFilename(inputFile, false)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						os.Exit(1)
					}

					intBuffer, err := roscoconv.MakePCM(info, streamID)
					if err != nil {
						fmt.Printf("Error: %v\n", err)
						os.Exit(1)
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
