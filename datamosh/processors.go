package datamosh

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/abema/go-mp4"
	"github.com/sunfish-shogi/bufseekio"
)

func ProcessFile(inputFile *os.File, outputFile *os.File) error {
	r := bufseekio.NewReadSeeker(inputFile, 128*1024, 4)
	// w := mp4.NewWriter(outputFile)

	var track *Track
	// keeping track of NAL units so we can process them later
	NALunits := []NALUnit{}

	_, err := mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {

		if Debug {
			fmt.Println("Box:", h.BoxInfo.Type)
		}

		if !h.BoxInfo.IsSupportedType() {
			// copy all data
			// if err = w.CopyBox(r, &h.BoxInfo); err != nil {
			// 	return nil, err
			// }
			return nil, nil
		}

		bi := &h.BoxInfo
		var err error

		switch h.BoxInfo.Type {

		// header
		case mp4.BoxTypeMvhd():

			track = &Track{}
			var mvhd mp4.Mvhd
			if _, err := bi.SeekToPayload(r); err != nil {
				return nil, err
			}
			if _, err := mp4.Unmarshal(r, bi.Size-bi.HeaderSize, &mvhd, bi.Context); err != nil {
				return nil, err
			}
			track.Timescale = mvhd.Timescale
			if mvhd.GetVersion() == 0 {
				track.Duration = uint64(mvhd.DurationV0)
			} else {
				track.Duration = mvhd.DurationV1
			}

		case mp4.BoxTypeTrak():
			track, err = processTrak(r, bi)
			if err != nil {
				return nil, err
			}
			fmt.Printf("Track: %d\n", track.TrackID)
			fmt.Printf("  Duration: %d\n", track.Duration)
			fmt.Printf("  Timescale: %d\n", track.Timescale)
			if track.Codec == mp4.CodecAVC1 {
				fmt.Printf("  Codec: AVC1\n")
			} else {
				fmt.Printf("  Codec: MP4A\n")
			}
			fmt.Printf("  Encrypted: %t\n", track.Encrypted)
			fmt.Printf("  Chunks: %d\n", len(track.Chunks))
			fmt.Printf("  Samples: %d\n", len(track.Samples))
			fmt.Printf("  EditList: %d\n", len(track.EditList))
			if track.AVC != nil {
				fmt.Printf("  AVC: profile %d\n", track.AVC.Profile)
				fmt.Printf("  AVC: level %d\n", track.AVC.Level)
				fmt.Printf("  AVC: width %d\n", track.AVC.Width)
				fmt.Printf("  AVC: height %d\n", track.AVC.Height)
			}
			fmt.Println()
			if track.AVC != nil {
				trackNALs, err := processTrack(r, track)
				if err != nil {
					fmt.Println("Error processing track:", err)
					return nil, err
				}
				NALunits = append(NALunits, trackNALs...)
			}

		case mp4.BoxTypeMdat():
			if Debug {
				fmt.Println("mdat box found, TODO: process it")
				fmt.Printf("Offset: %d, Size: %d\n\n", bi.Offset, bi.Size)
			}
		default:
		}

		if _, err := h.Expand(); err != nil {
			return nil, err
		}

		return nil, nil
	})

	if err != nil {
		fmt.Println("Error reading box structure:", err)
		return err
	}

	// create a copy of the input file
	if _, err := inputFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.Copy(outputFile, inputFile); err != nil {
		return err
	}

	// rewind the output file and processe the NALUnits
	if _, err := outputFile.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// TODO: keep track of SPS and PPS NAL units so we can look them up

	sawFirstIFrame := false
	for _, nalUnit := range NALunits {
		switch nalUnit.Type {
		case byte(NAL_SLICE):
			sliceType, err := nalUnit.ParseSlice(outputFile)
			if err != nil {
				fmt.Println("Error parsing slice:", err)
				return err
			}
			fmt.Printf("  [%d] Frame: %s\n", nalUnit.TrackID, sliceType)
		case byte(NAL_IDR_SLICE):
			sliceType, err := nalUnit.ParseSlice(outputFile)
			if err != nil {
				fmt.Println("Error parsing slice:", err)
				return err
			}
			fmt.Printf("  [%d] IDR Frame: %s\n", nalUnit.TrackID, sliceType)
			if !sawFirstIFrame {
				sawFirstIFrame = true
				// skip the first iframe since we want the video to start properly
				continue
			}
			// temp test, nullify I-frames
			nalUnit.Nullify(outputFile)
		case 6:
			fmt.Printf("  SEI Metadata | offset: %d, length: %d\n", nalUnit.Offset, nalUnit.Length)
		case 7:
			// sps, err = nalUnit.ParseSPS(outputFile)
			// if err != nil {
			// 	fmt.Println("Error parsing SPS:", err)
			// 	return err
			// }
		default:
			fmt.Printf("NAL type: %d, offset: %d, length: %d\n", nalUnit.Type, nalUnit.Offset, nalUnit.Length)
		}
	}

	return nil
}

func processTrak(r io.ReadSeeker, bi *mp4.BoxInfo) (*Track, error) {

	bips, err := mp4.ExtractBoxesWithPayload(r, bi, []mp4.BoxPath{
		{mp4.BoxTypeTkhd()},
		{mp4.BoxTypeEdts(), mp4.BoxTypeElst()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMdhd()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeAvc1()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeAvc1(), mp4.BoxTypeAvcC()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeEncv()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeEncv(), mp4.BoxTypeAvcC()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeMp4a()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeMp4a(), mp4.BoxTypeEsds()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeMp4a(), mp4.BoxTypeWave(), mp4.BoxTypeEsds()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeEnca()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd(), mp4.BoxTypeEnca(), mp4.BoxTypeEsds()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStco()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeCo64()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStts()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeCtts()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsc()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsz()},
	})
	if err != nil {
		return nil, err
	}
	var tkhd *mp4.Tkhd
	var elst *mp4.Elst
	var mdhd *mp4.Mdhd
	var avc1 *mp4.VisualSampleEntry
	var avcC *mp4.AVCDecoderConfiguration
	// var audioSampleEntry *mp4.AudioSampleEntry
	// var esds *mp4.Esds
	var stco *mp4.Stco
	var stts *mp4.Stts
	var stsc *mp4.Stsc
	var ctts *mp4.Ctts
	var stsz *mp4.Stsz
	var co64 *mp4.Co64
	var track Track

	for _, bip := range bips {
		switch bip.Info.Type {
		case mp4.BoxTypeTkhd():
			tkhd = bip.Payload.(*mp4.Tkhd)
		case mp4.BoxTypeElst():
			elst = bip.Payload.(*mp4.Elst)
		case mp4.BoxTypeMdhd():
			mdhd = bip.Payload.(*mp4.Mdhd)
		case mp4.BoxTypeAvc1():
			track.Codec = mp4.CodecAVC1
			avc1 = bip.Payload.(*mp4.VisualSampleEntry)
		case mp4.BoxTypeAvcC():
			avcC = bip.Payload.(*mp4.AVCDecoderConfiguration)
		case mp4.BoxTypeEncv():
			track.Codec = mp4.CodecAVC1
			track.Encrypted = true
		case mp4.BoxTypeMp4a():
			track.Codec = mp4.CodecMP4A
			// audioSampleEntry = bip.Payload.(*mp4.AudioSampleEntry)
		case mp4.BoxTypeEnca():
			track.Codec = mp4.CodecMP4A
			track.Encrypted = true
			// audioSampleEntry = bip.Payload.(*mp4.AudioSampleEntry)
		case mp4.BoxTypeEsds():
			// esds = bip.Payload.(*mp4.Esds)
		case mp4.BoxTypeStco():
			stco = bip.Payload.(*mp4.Stco)
		case mp4.BoxTypeStts():
			stts = bip.Payload.(*mp4.Stts)
		case mp4.BoxTypeStsc():
			stsc = bip.Payload.(*mp4.Stsc)
		case mp4.BoxTypeCtts():
			ctts = bip.Payload.(*mp4.Ctts)
		case mp4.BoxTypeStsz():
			stsz = bip.Payload.(*mp4.Stsz)
		case mp4.BoxTypeCo64():
			co64 = bip.Payload.(*mp4.Co64)
		}
	}

	if tkhd == nil {
		return nil, errors.New("tkhd box not found")
	}
	track.TrackID = tkhd.TrackID

	if elst != nil {
		editList := make([]*mp4.EditListEntry, 0, len(elst.Entries))
		for i := range elst.Entries {
			editList = append(editList, &mp4.EditListEntry{
				MediaTime:       elst.GetMediaTime(i),
				SegmentDuration: elst.GetSegmentDuration(i),
			})
			if Debug {
				fmt.Printf("Segment: time: %d duration: %d\n", elst.GetMediaTime(i), elst.GetSegmentDuration(i))
			}
		}
		track.EditList = editList
	}

	if mdhd == nil {
		return nil, errors.New("mdhd box not found")
	}
	track.Timescale = mdhd.Timescale
	track.Duration = mdhd.GetDuration()

	if avc1 != nil && avcC != nil {
		track.AVC = &AVCDecoderConfig{
			AVCDecoderConfiguration: *avcC,
			LengthSize:              uint16(avcC.LengthSizeMinusOne) + 1,
			Width:                   avc1.Width,
			Height:                  avc1.Height,
		}
	}

	// if audioSampleEntry != nil && esds != nil {
	// 	oti, audOTI, err := mp4.detectAACProfile(esds)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	track.MP4A = &mp4.MP4AInfo{
	// 		OTI:          oti,
	// 		AudOTI:       audOTI,
	// 		ChannelCount: audioSampleEntry.ChannelCount,
	// 	}
	// }

	track.Chunks = make([]*mp4.Chunk, 0)
	if stco != nil {
		for _, offset := range stco.ChunkOffset {
			track.Chunks = append(track.Chunks, &mp4.Chunk{
				DataOffset: uint64(offset),
			})
		}
	} else if co64 != nil {
		for _, offset := range co64.ChunkOffset {
			track.Chunks = append(track.Chunks, &mp4.Chunk{
				DataOffset: offset,
			})
		}
	} else {
		return nil, errors.New("stco/co64 box not found")
	}

	if stts == nil {
		return nil, errors.New("stts box not found")
	}
	track.Samples = make([]*mp4.Sample, 0)
	for _, entry := range stts.Entries {
		for i := uint32(0); i < entry.SampleCount; i++ {
			track.Samples = append(track.Samples, &mp4.Sample{
				TimeDelta: entry.SampleDelta,
			})
		}
	}

	if stsc == nil {
		return nil, errors.New("stsc box not found")
	}
	for si, entry := range stsc.Entries {
		end := uint32(len(track.Chunks))
		if si != len(stsc.Entries)-1 && stsc.Entries[si+1].FirstChunk-1 < end {
			end = stsc.Entries[si+1].FirstChunk - 1
		}
		for ci := entry.FirstChunk - 1; ci < end; ci++ {
			track.Chunks[ci].SamplesPerChunk = entry.SamplesPerChunk
		}
	}

	if ctts != nil {
		var si uint32
		for ci, entry := range ctts.Entries {
			for i := uint32(0); i < entry.SampleCount; i++ {
				if si >= uint32(len(track.Samples)) {
					break
				}
				track.Samples[si].CompositionTimeOffset = ctts.GetSampleOffset(ci)
				si++
			}
		}
	}

	if stsz != nil {
		for i := 0; i < len(stsz.EntrySize) && i < len(track.Samples); i++ {
			track.Samples[i].Size = stsz.EntrySize[i]
		}
	}

	return &track, nil
}

func processTrack(r io.ReadSeeker, track *Track) ([]NALUnit, error) {
	if track.AVC == nil {
		return nil, errors.New("AVC configuration not found")
	}
	lengthSize := uint32(track.AVC.LengthSize)

	nalUnits := []NALUnit{}

	var si int
	for nChunk, chunk := range track.Chunks {
		end := si + int(chunk.SamplesPerChunk)
		dataOffset := chunk.DataOffset
		if Debug {
			fmt.Printf("Chunk %d: offset: %d, samples %d-%d\n", nChunk, dataOffset, si, end)
		}
		for ; si < end && si < len(track.Samples); si++ {
			sample := track.Samples[si]
			if sample.Size == 0 {
				continue
			}
			for nalOffset := uint32(0); nalOffset+lengthSize+1 <= sample.Size; {
				if _, err := r.Seek(int64(dataOffset+uint64(nalOffset)), io.SeekStart); err != nil {
					return nalUnits, err
				}
				data := make([]byte, lengthSize+1)
				if _, err := io.ReadFull(r, data); err != nil {
					return nalUnits, err
				}
				var length uint32
				for i := 0; i < int(lengthSize); i++ {
					length = (length << 8) + uint32(data[i])
				}
				nalHeader := data[lengthSize]
				nalType := nalHeader & 0x1f

				if Debug {
					switch nalType {
					case 1:
						fmt.Println("  P-frame or B-frame")
						// fmt.Println("\tCoded slice of a non-IDR picture")
						// fmt.Println("\tThis type represents a regular slice of a P-frame or B-frame. Non-IDR pictures are used for inter-prediction and are dependent on other frames for decoding.")
					case 2:
						fmt.Println("  Data Partition A")
					case 3:
						fmt.Println("  Data Partition B")
					case 4:
						fmt.Println("  Data Partition C")
					case 5:
						fmt.Println("  I-frame")
						// fmt.Println("\tCoded slice of an IDR (Instantaneous Decoding Refresh) picture")
						// fmt.Println("\tAn IDR picture is a special type of I-frame that serves as a recovery point for the decoder. When an IDR picture is encountered, the decoder discards all previously decoded pictures and starts decoding afresh from the IDR picture.")
					case 6:
						fmt.Println("  SEI Metadata")
						// fmt.Println("\tSupplemental Enhancement Information (SEI)")
						// fmt.Println("\tSEI messages contain metadata about the video stream that can be used for various purposes, such as buffering, picture timing, and user data.")
					case 7:
						fmt.Println("  Sequence parameter set")
					case 8:
						fmt.Println("  Picture parameter set")
					case 9:
						fmt.Println("  Access unit delimiter")
					case 10:
						fmt.Println("  End of sequence")
					case 11:
						fmt.Println("  End of stream")
					case 12:
						fmt.Println("  Filler data")
					case 13:
						fmt.Println("  Sequence parameter set extension")
					case 14:
						fmt.Println("  Prefix NAL unit")
					case 15:
						fmt.Println("  Subset sequence parameter set")
					case 19:
						fmt.Println("  Auxiliary coded picture without partitioning")
					case 20:
						fmt.Println("  Slice extension")
					case 21:
						fmt.Println("  Slice extension for depth view components")
					default:
						fmt.Println("  NAL type:", nalType)
					}
					fmt.Println(int64(dataOffset+uint64(nalOffset)), length)
					fmt.Println()
				}

				nalUnits = append(nalUnits, NALUnit{
					Type:     nalType,
					Offset:   int64(dataOffset+uint64(nalOffset)) + int64(lengthSize),
					Length:   length,
					TrackID:  track.TrackID,
					Chunk:    uint32(nChunk),
					SampleID: uint32(si),
				})

				nalOffset += lengthSize + length
			}
			dataOffset += uint64(sample.Size)
		}
	}

	return nalUnits, nil
}
