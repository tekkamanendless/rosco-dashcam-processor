package rosco

import "image"

// FileInfo contains all of the information from an NVR file.
type FileInfo struct {
	Filename string
	Unknown1 []byte
	Metadata *Metadata
	Chunks   []*Chunk
}

// Metadata defines a collection of metadata entries.
type Metadata struct {
	Entries []MetadataEntry
}

func (m *Metadata) Entry(name string) *MetadataEntry {
	for _, entry := range m.Entries {
		if entry.Name == name {
			return &entry
		}
	}
	return nil
}

// MetadataEntry is a single metadata entry.
type MetadataEntry struct {
	Type  int8
	Name  string
	Value interface{}
}

// Chunk is a chunk from a stream (either audio or video).
type Chunk struct {
	ID    string
	Type  string
	Audio *AudioChunk
	Video *VideoChunk
	Image image.Image
}

// AudioChunk is an audio chunk.
type AudioChunk struct {
	Timestamp  uint64
	Media      []byte
	ExtraMedia []byte
}

// VideoChunk is a video chunk.
type VideoChunk struct {
	Codec     string
	Unknown1  []byte
	Timestamp uint64
	Metadata  *Metadata
	Media     []byte
}
