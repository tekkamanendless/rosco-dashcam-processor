package roscoconv

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nareix/joy4/codec/h264parser"
	"github.com/sirupsen/logrus"
	"github.com/tekkamanendless/rosco-dashcam-processor/riff"
	"github.com/tekkamanendless/rosco-dashcam-processor/rosco"
)

// MakeAVI creates a `riff.AVIFile` instance based on this `rosco.FileInfo` one.
//
// Stream ID is the ID of the stream to export.
func MakeAVI(info *rosco.FileInfo, streamID string) (*riff.AVIFile, error) {
	streamIDs := []string{}
	for _, id := range info.StreamIDs() {
		if len(streamID) == 1 {
			if strings.HasPrefix(id, streamID) {
				streamIDs = append(streamIDs, id)
			}
		} else if len(streamID) == 2 {
			if id == streamID {
				streamIDs = append(streamIDs, id)
			}
		}
	}

	audioStreamID := streamID
	if len(audioStreamID) == 1 {
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
					audioStreamID = id
					break
				}
			}
		}
	}

	videoChunks := []*rosco.Chunk{}
	for _, id := range streamIDs {
		substreamChunks := info.ChunksForStreamID(id)
		for _, chunk := range substreamChunks {
			if chunk.Video != nil {
				videoChunks = append(videoChunks, chunk)
			}
		}
	}
	sort.Slice(videoChunks, func(i, j int) bool {
		return videoChunks[i].Video.Timestamp < videoChunks[j].Video.Timestamp
	})

	// Figure out the video information.
	var videoWidth int32
	var videoHeight int32
	for chunkIndex, chunk := range videoChunks {
		// Only use the key frames.
		if !strings.HasSuffix(chunk.ID, "0") {
			continue
		}
		nalus, _ := h264parser.SplitNALUs(chunk.Video.Media)
		if len(nalus) == 0 {
			continue
		}
		for naluIndex, nalu := range nalus {
			spsInfo, err := h264parser.ParseSPS(nalu)
			if err != nil {
				logrus.Debugf("Chunk %d: NALU %d: Could not parse SPS: %v", chunkIndex, naluIndex, err)
				continue
			}
			newWidth := int32(spsInfo.Width)
			newHeight := int32(spsInfo.Height)
			logrus.Debugf("Chunk %d: NALU %d: profile_idc: %d, width: %d, height: %d", chunkIndex, naluIndex, spsInfo.ProfileIdc, newWidth, newHeight)
			switch spsInfo.ProfileIdc {
			case 66, 77, 88, 100, 110, 122, 244: // These profiles actually encode real video.
				if newWidth > videoWidth {
					videoWidth = newWidth
					logrus.Debugf("Chunk %d: NALU %d: Setting new video width: %d", chunkIndex, naluIndex, videoWidth)
				}
				if newHeight > videoHeight {
					videoHeight = newHeight
					logrus.Debugf("Chunk %d: NALU %d: Setting new video height: %d", chunkIndex, naluIndex, videoHeight)
				}
			}
		}
	}
	logrus.Debugf("Video dimensions: width: %d, height: %d", videoWidth, videoHeight)

	// Strip out any frames before the first keyframe.  We can't do anything
	// without a keyframe.
	firstKeyframeIndex := 0
	for chunkIndex, chunk := range videoChunks {
		// Only use the key frames.
		if !strings.HasSuffix(chunk.ID, "0") {
			continue
		}
		firstKeyframeIndex = chunkIndex
		break
	}
	videoChunks = videoChunks[firstKeyframeIndex:]

	var firstVideoTimestamp uint64
	var lastVideoTimestamp uint64
	for _, chunk := range videoChunks {
		if firstVideoTimestamp == 0 || chunk.Video.Timestamp < firstVideoTimestamp {
			firstVideoTimestamp = chunk.Video.Timestamp
		}
		if lastVideoTimestamp == 0 || chunk.Video.Timestamp > lastVideoTimestamp {
			lastVideoTimestamp = chunk.Video.Timestamp
		}
	}
	videoDuration := lastVideoTimestamp - firstVideoTimestamp
	framesPerSecond := 1000000.0 * float64(len(videoChunks)) / float64(videoDuration)

	logrus.Debugf("First video timestamp: %d", firstVideoTimestamp)
	logrus.Debugf("Last video timestamp: %d", lastVideoTimestamp)
	logrus.Debugf("Video duration: %d us", videoDuration)
	logrus.Debugf("Video frames: %d", len(videoChunks))
	logrus.Debugf("Frames per second: %v", framesPerSecond)

	videoStream := riff.Stream{
		Header: riff.AVIStreamHeader{
			Type:                [4]byte{'v', 'i', 'd', 's'},
			Handler:             [4]byte{'H', '2', '6', '4'},   // TODO: Pull this from the chunks.
			Rate:                int32(1000 * framesPerSecond), // Effective fps is Rate / Scale; this allows for fractional fps.
			Scale:               1000,                          // Effective fps is Rate / Scale; this allows for fractional fps.
			SuggestedBufferSize: 65536,
			Width:               int16(videoWidth),
			Height:              int16(videoHeight),
		},
		VideoFormat: riff.AVIStreamVideoFormat{
			Size:        int32(len(new(riff.AVIStreamVideoFormat).Bytes())),
			Width:       videoWidth,
			Height:      videoHeight,
			Planes:      1,
			BitCount:    24,                          // TODO: Pull this from the metadata.
			Compression: [4]byte{'H', '2', '6', '4'}, // TODO: Pull this from the chunks.
		},
	}
	videoStream.VideoFormat.SizeImage = videoStream.VideoFormat.Width * videoStream.VideoFormat.Height * int32(videoStream.VideoFormat.BitCount) / 8
	for _, chunk := range videoChunks {
		streamChunk := riff.Chunk{
			ID:         "00dc",
			Data:       chunk.Video.Media,
			IsKeyframe: strings.HasSuffix(chunk.ID, "0"),
			Timestamp:  chunk.Video.Timestamp,
		}
		videoStream.Chunks = append(videoStream.Chunks, streamChunk)
	}
	videoStream.Header.Length = int32(len(videoChunks))
	file := &riff.AVIFile{
		Header: riff.AVIHeader{
			MicroSecPerFrame:    int32(videoDuration) / int32(len(videoChunks)),
			MaxBytesPerSec:      0,
			PaddingGranularity:  0,
			Flags:               riff.AVIFlagIsInterleaved | riff.AVIFlagTrustCKType | riff.AVIFlagHasIndex,
			TotalFrames:         int32(len(videoStream.Chunks)),
			InitialFrames:       0,
			Streams:             0,
			SuggestedBufferSize: 65536,
			Width:               videoWidth,
			Height:              videoHeight,
			Scale:               0,
			Rate:                0,
			Start:               0,
			Length:              0,
		},
	}
	file.Streams = append(file.Streams, videoStream)
	file.Header.Streams++

	//spew.Dump(file.Header)
	//spew.Dump(videoStream.Header)

	fmt.Printf("Audio stream ID: %s\n", audioStreamID)
	{
		rawPCM := strings.HasSuffix(audioStreamID, "7")

		wavAudioFormat := 0x0001 // PCM
		audioBitDepth := 8
		if rawPCM {
			entry := info.Metadata.Entry("_audioBitDepth")
			if entry != nil {
				audioBitDepth = int(entry.Value.(int64))
				logrus.Debugf("Audio bit depth (from the metadata): %d", audioBitDepth)
			}

			entry = info.Metadata.Entry("_wavAudioFormat")
			if entry != nil {
				wavAudioFormat = int(entry.Value.(int64))
				logrus.Debugf("WAV audio format (from the metadata): %d", wavAudioFormat)
			}
		}
		logrus.Debugf("Audio bit depth: %d", audioBitDepth)
		logrus.Debugf("WAV audio format: %d", wavAudioFormat)

		audioStream := riff.Stream{}

		chunks := info.ChunksForStreamID(audioStreamID)
		for chunkIndex, chunk := range chunks {
			intBuffer, err := MakePCM(chunk.Audio.Media, rawPCM, audioBitDepth)
			if err != nil {
				return nil, err
			}

			if chunkIndex == 0 {
				audioStream.Header = riff.AVIStreamHeader{
					Type:                [4]byte{'a', 'u', 'd', 's'},
					Handler:             [4]byte{' ', ' ', ' ', ' '},
					Scale:               1,
					Rate:                int32(intBuffer.Format.SampleRate),
					SuggestedBufferSize: 65536,
				}
				audioStream.AudioFormat = riff.AVIStreamAudioFormat{
					FormatTag:      int16(wavAudioFormat),
					Channels:       int16(intBuffer.Format.NumChannels),
					SamplesPerSec:  int32(intBuffer.Format.SampleRate),
					AvgBytesPerSec: int32(intBuffer.Format.SampleRate * intBuffer.Format.NumChannels / (intBuffer.SourceBitDepth / 8)),
					BlockAlign:     int16(intBuffer.SourceBitDepth / 8 * intBuffer.Format.NumChannels),
					BitsPerSample:  int16(intBuffer.SourceBitDepth * intBuffer.Format.NumChannels),
				}
			}

			rawBytes, err := MakeRawAudio(intBuffer)
			if err != nil {
				return nil, err
			}

			streamChunk := riff.Chunk{
				ID:        "01wb",
				Data:      rawBytes,
				Timestamp: chunk.Audio.Timestamp,
			}
			audioStream.Chunks = append(audioStream.Chunks, streamChunk)
		}

		file.Streams = append(file.Streams, audioStream)
		file.Header.Streams++
	}

	return file, nil
}
