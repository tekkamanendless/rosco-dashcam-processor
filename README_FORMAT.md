# Format
Each NVR file consists of two parts:

1. (65536 bytes) Header
1. (remainder) List of stream data

## File header

1. (4 bytes) Filetype ID; always "SAYS"
1. (32 bytes) ???
1. (128 bytes) Filename
1. (4 bytes) Length of metadata
1. (see above) List of metadata; see "Metadata"

## Metadata

1. (1 byte) Type
1. (until a null byte is read) Name
1. (depends on type) Value

## Stream data

1. (2 bytes) Stream identifier (two ASCII digits)
1. (2 bytes) Stream type; either "dc" or "wb"
1. If stream type is "dc":
   1. (4 bytes) Encoding; always "H264"
   1. (4 bytes) Length of media (everything after the metadata); note that this will need to be padded to 8 bytes
   1. (2 bytes) Length of metadata
   1. (2 bytes) ???
   1. (4 bytes) Timestamp (appears to be in 1/1000000 seconds)
   1. (4 bytes) ??? (always zero)
   1. (4 bytes) Length of metadata (including these 4 bytes)
   1. (see above) List of metadata; see "Metadata"
   1. (see above) Stream data
1. If stream type is "wb":
   1. (2 bytes) Length of audio channel data
   1. (2 bytes) Length of everything until the end of the first channel
   1. (4 bytes) Timestamp (included in that second length) (appears to be in 1/1000000 seconds)
   1. (4 bytes) ??? (always zero) (included in that second length)
   1. (see above) Audio channel
   1. (see above) Audio channel

## Metadata

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

* `ts`; the unix timestamp, in milliseconds
