# Rosco Dashcam Processor
This provides a command-line utility to read Rosco dashcam files and extract their audio and video components.

The official Windows application provided by Rosco is extremely cumbersome and often buggy, so this provides an open-source alternative.

## File Format
For more information on the file format, see [here](README_FORMAT.md)

## Command Line
You can see everything that this can do using:

```
rosco --help
```

Some quick examples:

Show the info about a file:

```
rosco info /path/to/file.nvr
```

Extract the audio from a file as a WAV file:

```
rosco extract audio /path/to/file.nvr 17 /tmp/audio.wav
```
