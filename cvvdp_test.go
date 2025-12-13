package govship_test

import (
	"testing"

	vship "github.com/GreatValueCreamSoda/govship"
)

func Test_CVVDPHandler_ComputeScore(t *testing.T) {
	// Set up a dummy Colorspace
	var colorspace vship.Colorspace
	colorspace.SetDefaults(1920, 1080, vship.SamplingFormatUInt8)
	colorspace.ChromaSubsamplingHeight = 1
	colorspace.ChromaSubsamplingWidth = 1
	colorspace.ColorFamily = vship.ColorFamilyRGB

	// Initialize CVVDP handler
	handler, exception := vship.NewCVVDPHandler(&colorspace, &colorspace, 30.0, true, "standard_4k")
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
	distortedData := [3][]byte{
		make([]byte, ySize),
		make([]byte, uvSize),
		make([]byte, uvSize),
	}

	// Fill with dummy values
	for i := range srcData {
		for j := range srcData[i] {
			srcData[i][j] = byte((j + i) % 255)
			distortedData[i][j] = byte((j + 1 + i) % 255)
		}
	}

	lineSizeSrc := [3]int64{int64(width), int64(width / 2), int64(width / 2)}
	lineSizeDst := lineSizeSrc

	// Optionally load frames into temporal filter first
	exception = handler.LoadTemporal(srcData, distortedData, lineSizeSrc, lineSizeDst)
	if !exception.IsNone() {
		t.Log(exception.GetError())
		t.FailNow()
	}

	// Compute score (dst can be nil for no distortion map)
	score, exception := handler.ComputeScore(nil, 0, srcData, distortedData, lineSizeSrc, lineSizeDst)
	if !exception.IsNone() {
		t.Log(exception.GetError())
		t.FailNow()
	}

	t.Logf("CVVDP Score: %.6f", score)
}
