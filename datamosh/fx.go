package datamosh

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
)

// function type to process frames/NAL units
type FrameProcessor func(context.Context, io.WriteSeeker, *NALUnit) (context.Context, error)

// NullifyIFrames nullifies I-frames
func NullifyIFrames(ctx context.Context, w io.WriteSeeker, nalUnit *NALUnit) (context.Context, error) {

	if nalUnit.Type != NAL_IDR_SLICE {
		return ctx, nil
	}

	// Retrieve and update I-frame count from context
	iFrameCount := 0
	if value, ok := ctx.Value(IFrameCountKey).(int); ok {
		iFrameCount = value
	}
	iFrameCount++
	ctx = context.WithValue(ctx, IFrameCountKey, iFrameCount)

	// Retrieve track and interactive flag from context
	track, _ := ctx.Value(TrackKey).(*Track)
	isInteractive, _ := ctx.Value(InteractiveKey).(bool)
	debug, _ := ctx.Value(DebugKey).(bool)

	if debug {
		if track != nil {
			fmt.Printf("I-Frame #%d: pts: %.2f\n", iFrameCount, float32(nalUnit.Timestamp)/float32(track.Timescale))
		} else {
			fmt.Printf("I-Frame #%d: offset: %d, length: %d\n", iFrameCount, nalUnit.Offset, nalUnit.Length)
		}
	}

	// Never nullify the first frame so the video starts properly
	if iFrameCount == 1 {
		return ctx, nil
	}

	var handleInteractiveMode = func(ctx context.Context, nalUnit *NALUnit, track *Track) (context.Context, bool) {
		fmt.Printf("Nullify I-frame at %.2f seconds? (y/n/a): ", float32(nalUnit.Timestamp)/float32(track.Timescale))
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil {
			log.Printf("Error reading user input: %v", err)
			return ctx, false
		}

		if strings.Contains(strings.ToLower(response), "n") {
			return ctx, false
		}
		// a means yes to all from now on
		if strings.Contains(strings.ToLower(response), "a") {
			ctx = context.WithValue(ctx, InteractiveKey, false)
		}
		return ctx, true
	}

	var err error
	if isInteractive {
		if track == nil {
			log.Printf("Track not found in context, can't nullify I-frame in interactive mode")
			err = nalUnit.Nullify(w)
		} else {
			var shouldNullify bool
			ctx, shouldNullify = handleInteractiveMode(ctx, nalUnit, track)
			if !shouldNullify {
				return ctx, nil
			}
		}
	} else {
		err = nalUnit.Nullify(w)
	}

	// Update I-frame removed count in context if nullification was successful
	if err == nil {
		iFrameRemovedCount := 0
		if value, ok := ctx.Value(IFrameRemovedCountKey).(int); ok {
			iFrameRemovedCount = value
		}
		iFrameRemovedCount++
		ctx = context.WithValue(ctx, IFrameRemovedCountKey, iFrameRemovedCount)
	}

	return ctx, err
}

// DuplicatePFrames duplicates P-frames for glitch effect
func DuplicatePFrames(track *Track) {
	nals := []*NALUnit{}
	// track.Chunks
	for _, nal := range track.NALs {
		nals = append(nals, nal)
		if nal.Type == NAL_SLICE {
			// nal.Chunk
			duplicate := *nal // Make a copy
			nals = append(nals, &duplicate)
		}
	}
}

// SwapPAndBFrames swaps P-frames and B-frames
func SwapPAndBFrames(nals []*NALUnit) []*NALUnit {
	for i := 0; i < len(nals)-1; i++ {
		if nals[i].Type == NAL_SLICE && (nals[i+1].Type == NAL_DPB || nals[i+1].Type == NAL_DPC) {
			nals[i], nals[i+1] = nals[i+1], nals[i]
		}
	}
	return nals
}
