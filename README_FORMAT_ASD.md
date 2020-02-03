# Format (ASD)
An ASD file is a proprietary audio/video container that attaches metadata to specific parts of the video streams, location.

Each ASD file consists of two parts:

1. A collection of "packets".
1. (remainder) Some additional binary data.

## Packets
Each packet has a type, a fixed size, and sometimes an additional payload.
Packets are listed back to back with no delimiters.

### Packet
1. (1 byte) Packet type.
1. (depends) The rest of the packet.
1. (sometimes) An additional payload (for use with audio and video data).
