package main

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/abema/go-mp4"
	"github.com/sunfish-shogi/bufseekio"
)

var (
	duplicationChance int
	removalChance     int
	err               error
	outputFileName    string
)

// duplicateAndRemoveFrames processes the video data, removing I-frames and optionally duplicating D-frames.
func duplicateAndRemoveFrames(data []byte, duplicationChance int, removalChance int, trackDuration float64) []byte {
	var newData []byte
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	fmt.Println("mdata length", len(data))

	for i := 0; i < len(data)-4; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			nalType := data[i+3] & 0x1F
			nalStart := i // + 3
			nalEnd := nalStart + 4
			for nalEnd < len(data)-4 && !(data[nalEnd] == 0x00 && data[nalEnd+1] == 0x00 && data[nalEnd+2] == 0x01) {
				nalEnd++
			}

			// Network Abstraction Layer units

			// Calculate the time of the frame
			frameTime := float64(i) / float64(len(data)) * trackDuration
			fmt.Printf("Processing frame at time: %.2f seconds\n", frameTime)

			// Coded slice of a non-IDR picture (P-frame or B-frame)
			if nalType == 1 { // P-frame or B-frame
				newData = append(newData, data[i:nalEnd]...) // Copy the frame as is
				if rnd.Intn(100) < duplicationChance {
					fmt.Printf("Duplicating frame type %d at time: %.2f\n", nalType, frameTime)
					newData = append(newData, data[i:nalEnd]...)
				} else {
					fmt.Printf("Not duplicating frame type %d at time: %.2f\n", nalType, frameTime)
				}
				// Coded slice of an IDR picture (I-frame)
			} else if nalType == 5 { // I-frame
				if rnd.Intn(100) >= removalChance {
					fmt.Printf("Keeping frame type %d at time: %.2f\n", nalType, frameTime)
					newData = append(newData, data[i:nalEnd]...)
				} else {
					fmt.Printf("Removing frame type %d at time: %.2f\n", nalType, frameTime)
				}
			} else { //if nalType != 7 && nalType != 8 { // Exclude SPS (type 7) and PPS (type 8)
				fmt.Printf("Copying frame type %d at time: %.2f\n", nalType, frameTime)
				newData = append(newData, data[i:nalEnd]...)
			}

			i = nalEnd - 1
		}
	}

	return newData
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: <input_file> <duplication_chance> <removal_chance> <output_file>")
		return
	}
	inputFileName := os.Args[1]

	if len(os.Args) > 2 {
		duplicationChance, err = strconv.Atoi(os.Args[2])
		if err != nil || duplicationChance < 0 || duplicationChance > 100 {
			fmt.Println("Duplication chance must be an integer between 0 and 100.")
			return
		}
	} else {
		duplicationChance = 30
	}

	if len(os.Args) > 3 {
		removalChance, err = strconv.Atoi(os.Args[3])
		if err != nil || removalChance < 0 || removalChance > 100 {
			fmt.Println("Removal chance must be an integer between 0 and 100.")
			return
		}
	} else {
		removalChance = 10

	}

	if len(os.Args) > 4 {
		outputFileName = os.Args[4]
	} else {
		outputFileName = "datamosh-output.mp4"
	}

	if _, err := os.Stat(inputFileName); os.IsNotExist(err) {
		fmt.Println("The input file does not exist.")
		return
	}

	inputFile, err := os.Open(inputFileName)
	if err != nil {
		fmt.Println("Error opening input file:", err)
		return
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputFileName)
	if err != nil {
		fmt.Println("Error creating output file:", err)
		return
	}
	defer outputFile.Close()

	// First extract the duration, then process the frames
	// Traverse the moov box to find the mvhd box
	// Write box header to the output file
	// Apply frame duplication and removal
	// Write modified payload to the output file
	// End the box
	// Copy all other boxes to the output file
	err = processFile(inputFile, outputFile)
	if err != nil {
		fmt.Println("Error processing file:", err)
	}

}

func processTrak(r io.ReadSeeker, bi *mp4.BoxInfo) (*mp4.Track, error) {

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
	var track mp4.Track

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
			fmt.Printf("Segment: time: %d duration: %d\n", elst.GetMediaTime(i), elst.GetSegmentDuration(i))
		}
		track.EditList = editList
	}

	if mdhd == nil {
		return nil, errors.New("mdhd box not found")
	}
	track.Timescale = mdhd.Timescale
	track.Duration = mdhd.GetDuration()

	if avc1 != nil && avcC != nil {
		track.AVC = &mp4.AVCDecConfigInfo{
			ConfigurationVersion: avcC.ConfigurationVersion,
			Profile:              avcC.Profile,
			ProfileCompatibility: avcC.ProfileCompatibility,
			Level:                avcC.Level,
			LengthSize:           uint16(avcC.LengthSizeMinusOne) + 1,
			Width:                avc1.Width,
			Height:               avc1.Height,
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

func processFile(inputFile *os.File, outputFile *os.File) error {
	r := bufseekio.NewReadSeeker(inputFile, 128*1024, 4)
	// w := mp4.NewWriter(outputFile)
	// TODO: extract all boxes and copy them to the output file.

	bis, err := mp4.ExtractBoxes(r, nil, []mp4.BoxPath{
		{mp4.BoxTypeMoov(), mp4.BoxTypeMvhd()},
		{mp4.BoxTypeMoov(), mp4.BoxTypeTrak()},
		{mp4.BoxTypeMdat()},
	})
	if err != nil {
		fmt.Println("Error extracting boxes from MP4 file:", err)
		return err
	}

	var track *mp4.Track

	for _, bi := range bis {
		switch bi.Type {
		case mp4.BoxTypeMvhd():
			track = &mp4.Track{}
			var mvhd mp4.Mvhd
			if _, err := bi.SeekToPayload(r); err != nil {
				return err
			}
			if _, err := mp4.Unmarshal(r, bi.Size-bi.HeaderSize, &mvhd, bi.Context); err != nil {
				return err
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
				return err
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
				if err = processTrack(r, track); err != nil {
					fmt.Println("Error processing track:", err)
					return err
				}
			}
		case mp4.BoxTypeMoof():
		case mp4.BoxTypeMdat():
			fmt.Println("mdat box found")
		}
	}

	// 	boxes, err := mp4.ExtractBoxWithPayload(r, nil, mp4.BoxPath{mp4.BoxTypeMoov()})
	// 	if err != nil {
	// 		fmt.Println("Error processing MP4 file:", err)
	// 		return err
	// 	}

	// 	for _, box := range boxes {
	// 		switch box.Info.Type {
	// 		case mp4.BoxTypeMoov():

	// 			nestedBoxes, err := mp4.ExtractBoxWithPayload(r, &box.Info, mp4.BoxPath{mp4.BoxTypeMvhd()})
	// 			if err != nil {
	// 				fmt.Println("Error processing moov box:", err)
	// 				continue
	// 			}

	// 			for _, nestedBox := range nestedBoxes {
	// 				switch nestedBox.Info.Type {
	// 				case mp4.BoxTypeMvhd():
	// 					mvhd := nestedBox.Payload.(*mp4.Mvhd)
	// 					trackDuration = float64(mvhd.GetDuration()) / float64(mvhd.Timescale)
	// 					fmt.Println("Track Duration:", trackDuration)
	// 				}
	// 			}
	// 		}
	// 	}

	// 	_, err = mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
	// 		switch h.BoxInfo.Type {

	// 		case mp4.BoxTypeMdat():

	// 			if _, err := w.StartBox(&h.BoxInfo); err != nil {
	// 				return nil, err
	// 			}

	// 			box, _, err := h.ReadPayload()
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			mdat := box.(*mp4.Mdat)

	// 			mdat.Data = duplicateAndRemoveFrames(mdat.Data, duplicationChance, removalChance, trackDuration)

	// 			if _, err := mp4.Marshal(w, mdat, h.BoxInfo.Context); err != nil {
	// 				return nil, err
	// 			}

	// 			if _, err := w.EndBox(); err != nil {
	// 				return nil, err
	// 			}
	// 		default:
	// 			fmt.Println("Copying box:", h.BoxInfo.Type)

	// 			return nil, w.CopyBox(r, &h.BoxInfo)
	// 		}
	// 		return nil, nil
	// 	})

	// 	if err != nil {
	// 		fmt.Println("Error processing MP4 file:", err)
	// 	}
	return nil
}

func processTrack(r io.ReadSeeker, track *mp4.Track) error {
	if track.AVC == nil {
		return errors.New("AVC configuration not found")
	}
	lengthSize := uint32(track.AVC.LengthSize)

	var si int
	idxs := make([]int, 0, 8)
	for _, chunk := range track.Chunks {
		end := si + int(chunk.SamplesPerChunk)
		dataOffset := chunk.DataOffset
		for ; si < end && si < len(track.Samples); si++ {
			sample := track.Samples[si]
			if sample.Size == 0 {
				continue
			}
			for nalOffset := uint32(0); nalOffset+lengthSize+1 <= sample.Size; {
				if _, err := r.Seek(int64(dataOffset+uint64(nalOffset)), io.SeekStart); err != nil {
					return err
				}
				data := make([]byte, lengthSize+1)
				if _, err := io.ReadFull(r, data); err != nil {
					return err
				}
				var length uint32
				for i := 0; i < int(lengthSize); i++ {
					length = (length << 8) + uint32(data[i])
				}
				nalHeader := data[lengthSize]
				nalType := nalHeader & 0x1f
				fmt.Println("NAL type:", nalType)
				if nalType == 5 {
					idxs = append(idxs, si)
					break
				}
				nalOffset += lengthSize + length
			}
			dataOffset += uint64(sample.Size)
		}
	}
	return nil
}
