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
   1. (4 bytes) Length of media
   1. (2 bytes) Length of metadata
   1. (2 bytes) ???
   1. (4 bytes) Timestamp
   1. (4 bytes) ??? (always zero)
   1. (4 bytes) Length of metadata? again?
   1. (see above) List of metadata; see "Metadata"
   1. (see above) Stream data
1. If stream type is "wb":
   1. (2 bytes) Length of audio channel data
   1. (2 bytes) Lenth of remaining data
   1. Remaining data
      1. (4 bytes) Timestamp
	  1. (4 bytes) ??? (always zero)
	  1. (see above) Audio channel
	  1. ...

