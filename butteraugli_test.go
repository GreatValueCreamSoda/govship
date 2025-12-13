package govship_test

import (
	"testing"

	vship "github.com/GreatValueCreamSoda/govship"
)

func Test_ButteraugliHandler_ComputeScore(t *testing.T) {
	// Create a dummy Colorspace
	var colorspace vship.Colorspace
	colorspace.SetDefaults(1920, 1080, vship.SamplingFormatUInt8)
	colorspace.ChromaSubsamplingHeight = 1
	colorspace.ChromaSubsamplingWidth = 1
	colorspace.ColorFamily = vship.ColorFamilyRGB

	// Initialize Butteraugli handler
	handler, exception := vship.NewButteraugliHandler(&colorspace, &colorspace, 5, 255.0)
	if !exception.IsNone() {
		t.Log(exception.GetError())
		t.FailNow()
	}
	defer handler.Close()

	width, height := 1920, 1080
	ySize := width * height
	uvSize := ySize / 4

	// Allocate three planes for source and distorted images
	srcData := [3][]byte{
		make([]byte, ySize),  // Y
		make([]byte, uvSize), // U
		make([]byte, uvSize), // V
	}
	dstData := [3][]byte{
		make([]byte, ySize),
		make([]byte, uvSize),
		make([]byte, uvSize),
	}

	// Fill with dummy values
	for i := range srcData {
		for j := range srcData[i] {
			srcData[i][j] = byte((j + i) % 255)
			dstData[i][j] = byte((j + 1 + i) % 255)
		}
	}

	lineSizeSrc := [3]int64{int64(width), int64(width / 2), int64(width / 2)}
	lineSizeDst := lineSizeSrc

	// Prepare score struct
	var score vship.ButteraugliScore

	exception = handler.ComputeScore(&score, nil, 0, srcData, dstData, lineSizeSrc, lineSizeDst)
	if !exception.IsNone() {
		t.Log(exception.GetError())
		t.FailNow()
	}

	t.Logf("Butteraugli Score: NormQ=%.4f Norm3=%.4f NormInf=%.4f",
		score.NormQ, score.Norm3, score.NormInf)
}
