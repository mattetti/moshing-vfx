package datamosh

import "github.com/abema/go-mp4"

type Track struct {
	TrackID   uint32
	Timescale uint32
	Duration  uint64
	Codec     mp4.Codec
	Encrypted bool
	EditList  mp4.EditList
	Samples   mp4.Samples
	Chunks    mp4.Chunks
	AVC       *AVCDecoderConfig
	MP4A      *mp4.MP4AInfo
}

type AVCDecoderConfig struct {
	mp4.AVCDecoderConfiguration

	LengthSize uint16
	Width      uint16
	Height     uint16
}
