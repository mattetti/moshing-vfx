package main

import (
	"fmt"
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

	for i := 0; i < len(data)-4; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			nalType := data[i+3] & 0x1F
			nalStart := i + 3
			nalEnd := nalStart
			for nalEnd < len(data)-4 && !(data[nalEnd] == 0x00 && data[nalEnd+1] == 0x00 && data[nalEnd+2] == 0x01) {
				nalEnd++
			}

			// Calculate the time of the frame (currently broken)
			frameTime := float64(i) / float64(len(data)) * trackDuration
			fmt.Printf("Processing frame at time: %.2f seconds\n", frameTime)

			// Process P-frames (type 1) and B-frames (type 2)
			if nalType == 1 { // P-frame or B-frame
				newData = append(newData, data[i:nalEnd]...)
				if rnd.Intn(100) < duplicationChance {
					newData = append(newData, data[i:nalEnd]...)
				}
			} else if nalType == 5 { // I-frame
				if rnd.Intn(100) >= removalChance {
					newData = append(newData, data[i:nalEnd]...)
				}
			} else { //if nalType != 7 && nalType != 8 { // Exclude SPS (type 7) and PPS (type 8)
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

	r := bufseekio.NewReadSeeker(inputFile, 128*1024, 4)
	w := mp4.NewWriter(outputFile)

	var trackDuration float64

	// TODO: first extract the duration, then process the frames

	_, err = mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
		switch h.BoxInfo.Type {
		case mp4.BoxTypeMoov():
			// Traverse the moov box to find the mvhd box
			_, err := h.Expand()
			if err != nil {
				return nil, err
			}
		case mp4.BoxTypeMvhd():
			box, _, err := h.ReadPayload()
			if err != nil {
				return nil, err
			}
			mvhd := box.(*mp4.Mvhd)
			trackDuration = float64(mvhd.GetDuration()) / float64(mvhd.Timescale)
			fmt.Println("Track Duration:", trackDuration)
		case mp4.BoxTypeMdat():
			// Write box header to the output file
			if _, err := w.StartBox(&h.BoxInfo); err != nil {
				return nil, err
			}

			box, _, err := h.ReadPayload()
			if err != nil {
				return nil, err
			}
			mdat := box.(*mp4.Mdat)

			// Apply frame duplication and removal
			mdat.Data = duplicateAndRemoveFrames(mdat.Data, duplicationChance, removalChance, trackDuration)

			// Write modified payload to the output file
			if _, err := mp4.Marshal(w, mdat, h.BoxInfo.Context); err != nil {
				return nil, err
			}

			// End the box
			if _, err := w.EndBox(); err != nil {
				return nil, err
			}
		default:
			fmt.Println("Copying box:", h.BoxInfo.Type)
			// Copy all other boxes to the output file
			return nil, w.CopyBox(r, &h.BoxInfo)
		}
		return nil, nil
	})

	if err != nil {
		fmt.Println("Error processing MP4 file:", err)
	}
}
