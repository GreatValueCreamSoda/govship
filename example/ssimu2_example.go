// Command ssimu2_example demonstrates how to compute the SSIMU2 difference between two videos using govship.
//
// This example uses ffmpeg to decode two input videos to raw YUV420p frames, then computes the SSIMU2 score for each frame pair.
//
// Usage:
//
//	go run ssimu2_example.go -ref reference.mp4 -distorted distorted.mp4 -w 1920 -h 1080
//
// Requirements:
//   - ffmpeg must be installed and in your PATH
//   - govship must be built and able to find libvship
package main

import (
	"fmt"
	"io"
	"log"
	"math"

	"github.com/GreatValueCreamSoda/govship"
	"github.com/spf13/pflag"
)

func main() {
	refPath := pflag.String("ref", "", "Reference video file")
	distPath := pflag.String("distorted", "", "Distorted video file")
	width := pflag.Int("w", 0, "Frame width")
	height := pflag.Int("h", 0, "Frame height")
	pflag.Parse()

	if *refPath == "" || *distPath == "" || *width == 0 || *height == 0 {
		log.Fatalf("Usage: go run ssimu2_example.go -ref ref.mp4 -distorted dist.mp4 -w WIDTH -h HEIGHT")
	}

	// Create video sources
	refSource, err := NewVideoSource(*refPath, *width, *height)
	if err != nil {
		log.Fatal(err)
	}
	defer refSource.Close()

	distSource, err := NewVideoSource(*distPath, *width, *height)
	if err != nil {
		log.Fatal(err)
	}
	defer distSource.Close()

	// Set up colorspace
	var cs govship.Colorspace
	cs.SetDefaults(int64(*width), int64(*height), govship.SamplingFormatUInt8)
	cs.ChromaSubsamplingWidth = 1
	cs.ChromaSubsamplingHeight = 1
	cs.ColorFamily = govship.ColorFamilyYUV

	handler, exc := govship.NewSSIMU2Handler(&cs, &cs)
	if !exc.IsNone() {
		log.Fatalf("Failed to create SSIMU2 handler: %v", exc.GetError())
	}
	defer handler.Close()

	lineSizes := [3]int64{int64(*width), int64(*width) / 2, int64(*width) / 2}

	// Allocate frame buffers
	frameSize := *width**height + 2*(*width**height/4) // YUV420p
	refFrame := make([]byte, frameSize)
	distFrame := make([]byte, frameSize)

	frame := 0
	var sum float64
	min := math.Inf(1)
	max := math.Inf(-1)

	for {
		_, errRef := refSource.ReadFrame(refFrame)
		_, errDist := distSource.ReadFrame(distFrame)

		if errRef == io.EOF || errDist == io.EOF {
			break
		}
		if errRef != nil || errDist != nil {
			log.Fatalf("Error reading frames: %v %v", errRef, errDist)
		}

		refPlanes := [3][]byte{
			refFrame[:*width**height],
			refFrame[*width**height : *width**height+(*width**height/4)],
			refFrame[*width**height+(*width**height/4):],
		}
		distPlanes := [3][]byte{
			distFrame[:*width**height],
			distFrame[*width**height : *width**height+(*width**height/4)],
			distFrame[*width**height+(*width**height/4):],
		}

		score, exc := handler.ComputeScore(refPlanes, distPlanes, lineSizes, lineSizes)
		if !exc.IsNone() {
			log.Fatalf("SSIMU2 error on frame %d: %v", frame, exc.GetError())
		}

		fmt.Printf("Frame %d: SSIMU2 = %.6f\n", frame, score)
		sum += score
		if score < min {
			min = score
		}
		if score > max {
			max = score
		}

		frame++
	}

	fmt.Printf("\n%d frames compared.\n", frame)
	if frame > 0 {
		fmt.Printf("Average SSIMU2: %.6f\nMin: %.6f\nMax: %.6f\n", sum/float64(frame), min, max)
	}
}
