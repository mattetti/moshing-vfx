package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/abema/go-mp4"
	"github.com/sunfish-shogi/bufseekio"
)

func datamoshNALUnits(data []byte) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < len(data)-4; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			nalStart := i + 3
			nalEnd := nalStart
			for nalEnd < len(data)-4 && !(data[nalEnd] == 0x00 && data[nalEnd+1] == 0x00 && data[nalEnd+2] == 0x01) {
				nalEnd++
			}
			fmt.Printf("nalStart: %d, nalEnd: %d\n", nalStart, nalEnd)
			maxMosh := int(math.Floor(float64(nalEnd-nalStart) * 0.01))
			fmt.Printf("maxMosh: %d\n", maxMosh)
			for j := nalStart; j < nalEnd; j++ {
				if (rnd.Intn(100)%3 == 0) && maxMosh > 0 {
					data[j] ^= 0xFF
					maxMosh--
				}
			}
			i = nalEnd - 1
		}
	}
}

func main() {
	// take the first argument as the input file name
	// verify that it exists.
	if len(os.Args) < 2 {
		fmt.Println("Please provide an input file name.")
		return
	}
	if os.Args[1] == "" {
		fmt.Println("Please provide an input file name.")
		return
	}
	if _, err := os.Stat(os.Args[1]); os.IsNotExist(err) {
		fmt.Println("The input file does not exist.")
		return
	}
	inputFileName := os.Args[1]

	outputFileName := "output_datamosh.mp4"

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

	_, err = mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
		fmt.Println("Processing box:", h.BoxInfo.Type.String())
		switch h.BoxInfo.Type {
		case mp4.BoxTypeMdat():
			// Write box header to the output file
			if _, err := w.StartBox(&h.BoxInfo); err != nil {
				return nil, err
			}
			// Read mdat payload into a buffer

			// read payload
			box, _, err := h.ReadPayload()
			if err != nil {
				return nil, err
			}

			// update MessageData
			dataBox := box.(*mp4.Mdat)
			datamoshNALUnits(dataBox.Data)

			if _, err := mp4.Marshal(w, dataBox, h.BoxInfo.Context); err != nil {
				return nil, err
			}

			// End the box
			if _, err := w.EndBox(); err != nil {
				return nil, err
			}
		default:
			// Copy all other boxes to the output file
			return nil, w.CopyBox(r, &h.BoxInfo)
		}
		return nil, nil
	})

	if err != nil {
		fmt.Println("Error processing MP4 file:", err)
	}
}
