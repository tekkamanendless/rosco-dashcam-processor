package riff

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Write writes a RIFF file.
func Write(writer io.Writer, file *AVIFile) error {
	var err error
	buffer := new(bytes.Buffer)
	_, err = buffer.Write([]byte{'A', 'V', 'I', ' '})
	if err != nil {
		return err
	}
	// "hdrl" list
	{
		headerListBuffer := new(bytes.Buffer)
		{
			err = writeChunk(headerListBuffer, "avih", file.AVIHeader.Bytes())
			if err != nil {
				return err
			}
		}
		for _, stream := range file.Streams {
			streamListChunks := new(bytes.Buffer)
			err = writeChunk(streamListChunks, "strh", stream.Header.Bytes())
			if err != nil {
				return err
			}
			err = writeChunk(streamListChunks, "strf", stream.Format.Bytes())
			if err != nil {
				return err
			}

			err = writeList(headerListBuffer, "strl", streamListChunks.Bytes())
			if err != nil {
				return err
			}
		}
		err = writeList(buffer, "hdrl", headerListBuffer.Bytes())
		if err != nil {
			return err
		}
	}
	// "movi" list
	{
		movieListBuffer := new(bytes.Buffer)
		for _, stream := range file.Streams {
			for _, chunk := range stream.Chunks {
				err = writeChunk(movieListBuffer, chunk.ID, chunk.Data)
				if err != nil {
					return err
				}
				if len(chunk.Data)%2 != 0 {
					_, err = movieListBuffer.Write([]byte{0})
					if err != nil {
						return err
					}
				}
			}
		}
		err = writeList(buffer, "movi", movieListBuffer.Bytes())
		if err != nil {
			return err
		}
	}

	err = writeChunk(writer, "RIFF", buffer.Bytes())
	if err != nil {
		return err
	}
	return nil
}

// writeChunk writes a chunk.
//
// A chunk looks like: FourCC Length Data
func writeChunk(writer io.Writer, chunkType string, data []byte) error {
	var err error

	typeCode := []byte(chunkType)
	if len(typeCode) != 4 {
		return fmt.Errorf("Chunk type must be 4 bytes long")
	}
	typeCode = typeCode[0:4]

	_, err = writer.Write(typeCode)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.LittleEndian, int32(len(data)))
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func writeList(writer io.Writer, listType string, data []byte) error {
	var err error

	typeCode := []byte(listType)
	if len(typeCode) != 4 {
		return fmt.Errorf("List type must be 4 bytes long")
	}
	typeCode = typeCode[0:4]

	_, err = writer.Write([]byte{'L', 'I', 'S', 'T'})
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.LittleEndian, int32(len(typeCode)+len(data)))
	if err != nil {
		return err
	}
	_, err = writer.Write(typeCode)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	if err != nil {
		return err
	}
	return nil
}
