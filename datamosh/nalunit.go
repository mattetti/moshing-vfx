package datamosh

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/mattetti/moshing-vfx/internal/bitio"
)

type NALType int

const (
	NAL_SLICE      = 1
	NAL_DPA        = 2
	NAL_DPB        = 3
	NAL_DPC        = 4
	NAL_IDR_SLICE  = 5
	NAL_SEI        = 6
	NAL_SPS        = 7
	NAL_PPS        = 8
	NAL_AUD        = 9
	NAL_END_SEQ    = 10
	NAL_END_STREAM = 11
	NAL_FILLER     = 12
	NAL_SPS_EXT    = 13
	NAL_PREFIX     = 14
	NAL_SUBSET_SPS = 15
	NAL_DEPTH_SPS  = 16
	NAL_AUX_SLICE  = 19
)

const EVC_MAX_PPS_COUNT = 64

// NALUnit represents a Network Abstraction Layer unit in a video file.
type NALUnit struct {
	Type      byte
	Offset    int64  // inside the mdat box, first byte of the NAL unit data (header byte)
	Length    uint32 // of the NAL unit data
	TrackID   uint32
	Chunk     uint32
	SampleID  uint32
	Timestamp uint64 // in the timescale of the track
}

type NALHeader struct {
	NalRefIdc      uint32
	NalUnitType    uint32
	FirstMBInSlice uint32
	SliceType      uint32
	PicParamID     uint32
}

// NALSlice represents the parsed slice header data.
type NALSlice struct {
	FirstMbInSlice         uint32
	SliceType              uint32
	PicParameterSetID      uint32
	FrameNum               uint32
	IdrPicID               uint32 // Only for NAL unit type 5
	FieldPicFlag           uint32 // Only if !frame_mbs_only_flag
	BottomFieldFlag        uint32 // Only if field_pic_flag
	PictType               string
	SlicePicParameterSetID uint32
}

// temp
var frameMbsOnlyFlag = true

// Parse parses the slice header data from the reader.
func (s *NALSlice) Parse(r bitio.Reader) error {
	var err error

	// 7.3.3 Slice header syntax
	s.FirstMbInSlice, err = r.ReadUE()
	if err != nil {
		return fmt.Errorf("failed to read first_mb_in_slice: %v", err)
	}

	s.SliceType, err = r.ReadUE()
	if err != nil {
		return fmt.Errorf("failed to read slice_type: %v", err)
	}

	// Read the pic_parameter_set_id
	s.PicParameterSetID, err = r.ReadUE()
	if err != nil {
		return fmt.Errorf("failed to read pic_parameter_set_id: %v", err)
	}
	// TODO: get the pps and sps from the NALUnit and use that to parse further.

	// hexDump(rs, 8)
	// bitDump(rs, 8)

	return nil
}

// Nullify replaces the NAL unit data with zeros.
func (n *NALUnit) Nullify(w io.WriteSeeker) error {
	// Create a byte slice filled with zeros of length n.Length.
	data := make([]byte, n.Length-1)

	// Assert that the io.WriteSeeker also implements io.WriterAt.
	writerAt, ok := w.(io.WriterAt)
	if !ok {
		return fmt.Errorf("writer does not implement io.WriterAt")
	}

	// Write the zeroed data at the correct offset.
	_, err := writerAt.WriteAt(data, n.Offset+1)
	if err != nil {
		return fmt.Errorf("failed to write the NAL unit data: %v", err)
	}

	return nil
}
func (n *NALUnit) SkipHeader(r io.ReadSeeker) error {
	// Seek to the start of the NAL unit data, past the header
	_, err := r.Seek(n.Offset+1, io.SeekStart)
	return err
}

func (n *NALUnit) ParseHeader(r io.ReadSeeker) (NALHeader, error) {
	header := NALHeader{}
	// Seek to the start of the NAL unit data.
	_, err := r.Seek(n.Offset, io.SeekStart)
	if err != nil {
		return header, fmt.Errorf("failed to seek to the NAL unit data: %v", err)
	}

	br := bitio.NewReadSeeker(r)

	// forbidden_zero_bit  f(1)
	bits, err := br.ReadBits(1)
	if err != nil {
		return header, fmt.Errorf("failed to read forbidden_zero_bit: %v", err)
	}
	forbiddenZeroBit := bits[0]
	if forbiddenZeroBit != 0 {
		return header, fmt.Errorf("forbidden_zero_bit is not 0")
	}

	// nal_ref_idc u(2)
	header.NalRefIdc, err = br.ReadUInt(2)
	if err != nil {
		return header, fmt.Errorf("failed to read nal_ref_idc: %v", err)
	}
	// nal_unit_type u(5)
	header.NalUnitType, err = br.ReadUInt(5)
	if err != nil {
		return header, fmt.Errorf("failed to read nal_unit_type: %v", err)
	}

	if header.NalUnitType != uint32(n.Type) {
		return header, errors.New("unexpected NAL unit type")
	}

	// TODO: extract optional svc_extension_flag, avc_3d_extension_flag based on the unit type.

	// data verification
	if header.NalRefIdc == 0 {
		if n.Type == NAL_IDR_SLICE {
			return header, errors.New("unexpected NAL ref idc for IDR slice")
		} else if n.Type == NAL_SEI ||
			n.Type == NAL_AUD ||
			n.Type == NAL_END_SEQ ||
			n.Type == NAL_END_STREAM ||
			n.Type == NAL_FILLER {
			return header, errors.New("unexpected NAL ref idc for non-reference slice")
		}
	}

	header.FirstMBInSlice, err = br.ReadUE()
	if err != nil {
		return header, fmt.Errorf("failed to read first_mb_in_slice: %v", err)
	}

	header.SliceType, err = br.ReadUE()
	if err != nil {
		return header, fmt.Errorf("failed to read slice_type: %v", err)
	}
	header.SliceType = header.SliceType % 5

	header.PicParamID, err = br.ReadUE()
	if err != nil {
		return header, fmt.Errorf("failed to read pic_parameter_set_id: %v", err)
	}

	return header, nil
}

// ExtractRBSP extracts the Raw Byte Sequence Payload from the NAL unit.
func (n *NALUnit) ExtractRBSP(r io.ReadSeeker) ([]byte, error) {
	// Seek to the start of the NAL unit data.
	_, err := r.Seek(n.Offset+1, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to the NAL unit data: %v", err)
	}

	if (n.Length - 1) <= 0 {
		return nil, fmt.Errorf("NAL unit data is empty")
	}

	nalSize := int(n.Length - 1)
	nalBuf := make([]byte, nalSize)
	_, err = r.Read(nalBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read NAL unit data: %v", err)
	}

	rbspBuf := make([]byte, len(nalBuf))
	j := 0

	for i := 0; i < nalSize; i++ {
		if i+2 < nalSize && nalBuf[i] == 0x00 && nalBuf[i+1] == 0x00 && nalBuf[i+2] == 0x03 {
			rbspBuf[j] = nalBuf[i]
			j++
			rbspBuf[j] = nalBuf[i+1]
			j++
			i += 2 // Skip the 0x03 byte
		} else {
			rbspBuf[j] = nalBuf[i]
			j++
		}
	}

	return rbspBuf[:j], nil
}

// SPS represents the parsed sequence parameter set data.
type SPS struct {
	ProfileIDC            uint32
	ConstraintSetFlags    uint32
	SPSId                 uint32
	LevelIDC              uint32
	Log2MaxFrameNumMinus4 uint32 // range of 0 to 12 inclusive
	// Other relevant fields can be added here as needed
}

// ParseSPS parses the SPS NAL unit from the reader.
// See 7.3.2.1 Sequence parameter set RBSP syntax
func (n *NALUnit) ParseSPS(r io.ReadSeeker) (*SPS, error) {

	// Seek to the start of the NAL unit data.
	_, err := r.Seek(n.Offset+1, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to the NAL unit data: %v", err)
	}

	var bitPos int
	var currentByte byte
	buffer := make([]byte, 1)

	sps := &SPS{}

	// Read profile_idc (u(8))
	sps.ProfileIDC, err = readBits(r, &bitPos, &currentByte, &buffer, 8)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile_idc: %v", err)
	}

	// Read constraint_set_flags (u(3))
	sps.ConstraintSetFlags = 0
	for i := 0; i < 3; i++ {
		flag, err := readBits(r, &bitPos, &currentByte, &buffer, 1)
		if err != nil {
			return nil, fmt.Errorf("failed to read constraint_set_flag %d: %v", i, err)
		}
		sps.ConstraintSetFlags |= flag << i
	}

	// Skip reserved_zero_5bits (u(5))
	_, err = readBits(r, &bitPos, &currentByte, &buffer, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to skip reserved_zero_2bits: %v", err)
	}

	// Read level_idc (u(8))
	sps.LevelIDC, err = readBits(r, &bitPos, &currentByte, &buffer, 8)
	if err != nil {
		return nil, fmt.Errorf("failed to read level_idc: %v", err)
	}

	// Read seq_parameter_set_id (ue(v))
	sps.SPSId, err = readExpGolombCode(r, &bitPos, &currentByte, &buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read seq_parameter_set_id: %v", err)
	}

	// Read log2_max_frame_num_minus4 (ue(v))
	sps.Log2MaxFrameNumMinus4, err = readExpGolombCode(r, &bitPos, &currentByte, &buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read log2_max_frame_num_minus4: %v", err)
	}
	// TODO: Continue parsing other SPS fields

	return sps, nil
}

// ParseSlice parses the slice NAL data from the reader and returns the frame type.
func (n *NALUnit) ParseSlice(rs io.ReadSeeker) (string, error) {
	if n.Type != NAL_SLICE && n.Type != NAL_IDR_SLICE && n.Type != NAL_AUX_SLICE {
		return "", errors.New("not a slice NAL unit")
	}

	rbsp, err := n.ExtractRBSP(rs)
	if err != nil {
		return "", fmt.Errorf("failed to extract RBSP: %v", err)
	}

	rbspR := bitio.NewReader(bytes.NewReader(rbsp))
	slice := &NALSlice{}
	err = slice.Parse(rbspR)
	if err != nil {
		return "", fmt.Errorf("failed to parse slice header: %v", err)
	}

	var frameType string
	switch slice.SliceType % 5 {
	case 0, 5:
		frameType = "P"
	case 1, 6:
		frameType = "B"
	case 2, 7:
		frameType = "I"
	case 3, 8:
		frameType = "SP"
	case 4, 9:
		frameType = "SI"
	default:
		frameType = "Unknown"
	}

	return frameType, nil
}
