package govship_test

import (
	"testing"

	vship "github.com/GreatValueCreamSoda/govship"
)

func Test_SSIMU2Handler_ComputeScore(t *testing.T) {
	// Dummy handler – the real one probably holds a C pointer

	var handler *vship.SSIMU2Handler
	var exception vship.ExceptionCode
	var colorspace vship.Colorspace
	colorspace.SetDefaults(1920, 1080, vship.SamplingFormatUInt8)
	colorspace.ChromaSubsamplingHeight = 1
	colorspace.ChromaSubsamplingWidth = 1
	colorspace.ColorFamily = vship.ColorFamilyRGB

	handler, exception = vship.NewSSIMU2Handler(&colorspace, &colorspace)
	if !exception.IsNone() {
		t.Log(exception.GetError())
		t.FailNow()
	}

	// Create three planes of fake YUV data (1920×1080 Y + half-size UV)
	width, height := 1920, 1080
	ySize := width * height
	uvSize := ySize / 4

	sourceData := [3][]byte{
		make([]byte, ySize),  // Y
		make([]byte, uvSize), // U
		make([]byte, uvSize), // V
	}
	distortedData := [3][]byte{
		make([]byte, ySize),
		make([]byte, uvSize),
		make([]byte, uvSize),
	}

	// Fill with something non-zero so the mock is happy
	for i := range sourceData {
		for j := range sourceData[i] {
			sourceData[i][j] = byte(j%255 + i)
			distortedData[i][j] = byte((j+1)%255 + i)
		}
	}

	sourceLineSize := [3]int64{int64(width), int64(width / 2), int64(width / 2)}
	distortedLineSize := sourceLineSize // same geometry in this test

	score, exception := handler.ComputeScore(sourceData, distortedData, sourceLineSize, distortedLineSize)
	if !exception.IsNone() {
		t.Log(exception.GetError())
		t.FailNow()
	}

	t.Log(score)
}
