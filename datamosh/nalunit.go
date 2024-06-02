package datamosh

import (
	"fmt"
	"io"
)

// NALUnit represents a Network Abstraction Layer unit in a video file.
type NALUnit struct {
	HeaderSize uint32 // size of the NAL unit header until the actual data (including the type)
	Type       byte
	Offset     int64  // inside the mdat box
	Length     uint32 // of the NAL unit data
	TrackID    uint32
	Chunk      uint32
	SampleID   uint32
}

// Nullify replaces the NAL unit data with zeros.
func (n *NALUnit) Nullify(w io.WriteSeeker) error {
	// Create a byte slice filled with zeros of length n.Length.
	data := make([]byte, n.Length)

	// Assert that the io.WriteSeeker also implements io.WriterAt.
	writerAt, ok := w.(io.WriterAt)
	if !ok {
		return fmt.Errorf("writer does not implement io.WriterAt")
	}

	// Write the zeroed data at the correct offset.
	_, err := writerAt.WriteAt(data, n.Offset+int64(n.HeaderSize))
	if err != nil {
		return fmt.Errorf("failed to write the NAL unit data: %v", err)
	}

	return nil
}
