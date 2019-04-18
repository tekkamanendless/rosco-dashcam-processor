package rosco

// FileInfo contains all of the information from an NVR file.
type FileInfo struct {
	Filename string
	Metadata *Metadata
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
