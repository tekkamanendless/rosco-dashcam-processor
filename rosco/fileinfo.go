package rosco

import "sort"

// StreamIDs returns the list of stream IDs present.
func (f *FileInfo) StreamIDs() []string {
	streamIDMap := map[string]bool{}

	for _, chunk := range f.Chunks {
		streamIDMap[chunk.ID] = true
	}

	streamIDs := []string{}
	for streamID := range streamIDMap {
		streamIDs = append(streamIDs, streamID)
	}
	sort.Strings(streamIDs)

	return streamIDs
}

// ChunksForStreamID returns all of the chunks for the given stream ID.
func (f *FileInfo) ChunksForStreamID(streamID string) []*Chunk {
	chunks := []*Chunk{}

	lastTimestamp := uint32(0)
	for _, chunk := range f.Chunks {
		if chunk.ID == streamID {
			if chunk.Video != nil {
				if chunk.Video.Timestamp < lastTimestamp {
					logger.Warnf("Chunk timestamp out of order: previous: %d, current: %v", lastTimestamp, chunk.Video.Timestamp)
				}
				lastTimestamp = chunk.Video.Timestamp
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}
