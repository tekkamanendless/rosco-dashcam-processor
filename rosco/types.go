package rosco

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
}

// AudioChunk is an audio chunk.
type AudioChunk struct {
	Timestamp int32
	Unknown1  []byte
	Channels  [][]byte
}

// VideoChunk is a video chunk.
type VideoChunk struct {
	Codec     string
	Unknown1  []byte
	Timestamp int32
	Unknown2  []byte
	Metadata  *Metadata
	Media     []byte
}
