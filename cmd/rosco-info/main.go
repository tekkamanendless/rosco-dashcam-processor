package main

import (
	"fmt"
	"os"

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
		fmt.Printf("   * %s = %v\n", entry.Name, entry.Value)
	}

	streamIDs := info.StreamIDs()
	fmt.Printf("Streams: (%d)\n", len(streamIDs))
	for i, streamID := range streamIDs {
		fmt.Printf("   %d. %s\n", i, streamID)
	}

	for _, streamID := range streamIDs {
		fmt.Printf("Stream: %s\n", streamID)
		chunks := info.ChunksForStreamID(streamID)
		fmt.Printf("   Chunks: %d\n", len(chunks))
	}

	//spew.Dump(info)
}
