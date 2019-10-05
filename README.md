# Rosco Dashcam Processor
This provides a command-line utility to read Rosco dashcam files (with the extension ".nvr") and extract their audio and video components.

The official Windows application provided by Rosco is extremely cumbersome and often buggy, so this provides an open-source alternative.

This has been tested with the following camera models:

* DV440

## File Format
The NVR file format is a fairly simple wrapper around raw PCM audio and h264 video.
Unfortunately, it is non-standard and cannot be played directly.
But that's why this tool is here!

For more information on the file format, see [here](README_FORMAT.md)

## Requirements

* `libopus-dev`
* `libopusfile-dev`

## Command Line
### Download
If you want to just use the `rosco` binary, you can get it using:

```
go get github.com/tekkamanendless/rosco-dashcam-processor/cmd/rosco
```

### Usage
You can see everything that this can do using:

```
rosco --help
```

### Examples
Here are some quick examples:

Show the info about a file:

```
rosco info /path/to/file.nvr
```

Export a single NVR file to its component AVI files.

```
rosco export dvpro /path/to/file.nvr
```

Export all of the NVR files to AVI files.

```
rosco export dvpro /path/to/files
```

Export all of the NVR files to AVI files and put the results in a separate directory.

```
rosco export dvpro /path/to/files --output-directory /my/output/files
```

Extract the audio from a file as a WAV file:

```
rosco export audio /path/to/file.nvr 1 /tmp/audio.wav
```

Extract the outside camera's video from a file as an AVI file:

```
rosco export video /path/to/file.nvr 0 /tmp/camera0.avi
```

Extract the inside camera's video from a file as an AVI file:

```
rosco export video /path/to/file.nvr 1 /tmp/camera0.avi
```

## Packages
The following Go packages are provided:

* `riff`, which provides the minimal support necessary to build a simple AVI file.
* `rosco`, which provides the data structures and functions necessary to work with Rosco NVR files.
* `roscoconv`, which provides tools for converting from Rosco NVR files to other formats.

## Future Development
Ideas for future development:

* Export to MP4 or other (better) file formats.
* Video concatenation (I have a list of files; make them one big video file).
