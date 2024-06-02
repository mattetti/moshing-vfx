package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/mattetti/moshing-vfx/datamosh"
)

var (
	duplicationChance int
	removalChance     int
	err               error
	outputFileName    string
)

var debug = flag.Bool("debug", false, "Enable debug mode")

func main() {
	flag.Parse()

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

	err = datamosh.ProcessFile(inputFile, outputFile)
	if err != nil {
		fmt.Println("Error processing file:", err)
	}

}
