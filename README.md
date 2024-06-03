# Data Moshing library for h264 video files

This library is a simple implementation of data moshing for mp4/h264 video files. It is based on the [data moshing techniques](https://en.wikipedia.org/wiki/Data_moshing) that consists of messing with the structure of a video file. This technique is used to create glitch art.

This is an experimental Work-In-Progress, unmaintained and unsupported library. It is not recommended for production use.

This is a pure Go implementation, no ffmpeg or other external dependencies are required.

## Usage

See cmd/iframe-remover/main.go for an example of how to use the library.