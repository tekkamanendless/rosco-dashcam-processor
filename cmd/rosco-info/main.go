package main

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/tekkamanendless/rosco-dashcam-processor/rosco"
)

func main() {
	filename := os.Args[1]

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
				audioDataLength += len(chunk.Audio.Channels)
			}
			if chunk.Video != nil {
				videoDataLength += len(chunk.Video.Media)
			}
		}
		fmt.Printf("   Chunks: %d\n", len(chunks))
		fmt.Printf("   Audio: %d bytes\n", audioDataLength)
		fmt.Printf("   Video: %d bytes\n", videoDataLength)
	}

	/*
		{
			chunks := info.ChunksForStreamID("01")
			spew.Dump(chunks)
		}
	*/

	spew.Dump(info)
}
