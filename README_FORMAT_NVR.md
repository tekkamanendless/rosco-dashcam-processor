# Format (NVR)
An NVR file is a proprietary audio/video container that attaches metadata to specific parts of the video streams, including voltage, speed, etc.

Each NVR file consists of two parts:

1. (65536 bytes) Header
1. (remainder) List of stream data

## File header
The file header contains the filename, which itself contains some time information, as well as some metadata about the camera.

1. (4 bytes) Filetype ID; always "SAYS"
1. (32 bytes) ???
1. (128 bytes) Filename
1. (4 bytes) Length of metadata
1. (see above) List of metadata; see "Metadata"

## Stream data
Streams can be:

1. Audio
2. Video
3. Image (JPEG)

### Audio
Audio streams can be raw PCM data or Opus-encoded data.

If the stream ID ends in `7`, then it is raw PCM data encoded as 8-bit (Mu-law) audio, one channel at a time.

If the stream ID ends in `9`, then it is Opus-encoded.

### Video
Video streams are encoded as h.264 packets.

1. (2 bytes) Stream identifier (two ASCII digits)
   1. (1 byte) Actual stream identifier (`0` for the first stream, `1` for the second stream, and so on)
   1. (1 byte) Substream identifier (`0` for key frames, `1` for deltas, `7` for raw PCM audio, `9` for Opus-encoded audio)
1. (2 bytes) Stream type; either "dc" or "wb"
1. If stream type is "dc" (then this is a video stream):
   1. (4 bytes) Encoding; always "H264"
   1. (4 bytes) Length of media (everything after the metadata); note that this will need to be padded to 8 bytes
   1. (2 bytes) Length of metadata
   1. (2 bytes) ???
   1. (8 bytes) Timestamp (in 1/1000000 seconds)
   1. (4 bytes) Length of metadata (including these 4 bytes)
   1. (see above) List of metadata; see "Metadata"
   1. (see above) Stream data
1. If stream type is "wb" (then this is an audio stream):
   1. (2 bytes) Length of audio channel data
   1. (2 bytes) Length of everything until the end of the first channel
   1. (8 bytes) Timestamp (included in that second length) (in 1/1000000 seconds)
   1. (see above) Audio channel
   1. [old versions only] (see above) Audio channel (this always seemed to be an exact copy of the actual audio channel)

### Image
Image streams are just raw JPEGs, which begin with `0xffd8` (the JPEG "start of image" marker).
There is no concept of stream ID for these, and there is no metadata.

## Metadata
Metadata is used to store arbitrary data.

1. (1 byte) Type
   1. `1`; 64-bit floating point
   1. `2`; String (4 bytes for the length, followed by "length" bytes)
   1. `3`; ??? 32-bit integer?
   1. `4`; Metadata (4 bytes for the length of the metadata, including those 4 bytes)
   1. `8`; 8-bit integer
   1. `9`; 64-bit integer
   1. `10`; ??? 32-bit integer?
1. (null-terminated) Name
1. (see above by type) Value

Common metadata keys:

* `ts`; the unix timestamp, in milliseconds
