package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mattetti/moshing-vfx/datamosh"
)

var (
	inputFlag       = flag.String("input", "", "Input file")
	debugFlag       = flag.Bool("debug", false, "Enable debug mode")
	interactiveFlag = flag.Bool("interactive", false, "Enable interactive mode")
)

func main() {
	flag.Parse()
	if *inputFlag == "" {
		fmt.Println("input file is required")
		flag.Usage()
		os.Exit(1)
	}

	inputFileName := *inputFlag
	ext := filepath.Ext(inputFileName)
	outputFileName := filepath.Join(filepath.Dir(inputFileName), fmt.Sprintf("%s-iframoshed%s", filepath.Base(inputFileName)[:len(ext)], ext))

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

	// copy the input file to the output file
	if _, err := io.Copy(outputFile, inputFile); err != nil {
		fmt.Println("Error copying input file to output file:", err)
		return
	}

	// Initialize the context with the I-frame count
	ctx := context.WithValue(context.Background(), datamosh.IFrameCountKey, 0)
	ctx = context.WithValue(ctx, datamosh.DebugKey, *debugFlag)

	if *interactiveFlag {
		ctx = context.WithValue(ctx, datamosh.InteractiveKey, *interactiveFlag)
	}

	ctx, err = datamosh.ProcessFrames(ctx, outputFile, datamosh.NullifyIFrames)
	if err != nil {
		fmt.Println("Error processing frames:", err)
		return
	}

	// Retrieve the final I-frame count from context
	if iFrameCount, ok := ctx.Value(datamosh.IFrameCountKey).(int); ok {
		fmt.Printf("Total I-frames: %d\n", iFrameCount)
	}
	if iFrameCount, ok := ctx.Value(datamosh.IFrameRemovedCountKey).(int); ok {
		fmt.Printf("Total I-frames removed: %d\n", iFrameCount)
	}

	fmt.Println("File processed and available as", outputFileName)
}
