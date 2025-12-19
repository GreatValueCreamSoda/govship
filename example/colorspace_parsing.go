package main

import (
	"github.com/GreatValueCreamSoda/gopixfmts"
	vship "github.com/GreatValueCreamSoda/govship"
)

func getVideoColorspace(video *openedVideo) (vship.Colorspace, error) {
	var colorspace vship.Colorspace

	colorspace.Width = int64(video.firstFrame.ScaledWidth)
	colorspace.TargetWidth = colorspace.Width
	colorspace.Height = int64(video.firstFrame.ScaledHeight)
	colorspace.TargetHeight = colorspace.Height

	videoPixelFormat, err := gopixfmts.PixFmtDescGet(gopixfmts.PixelFormat(video.firstFrame.ConvertedPixelFormat))

	comp, err := videoPixelFormat.Component(0)
	if err != nil {
		return colorspace, err
	}

	var videoDepth vship.SamplingFormat

	switch comp.Depth {
	case 8:
		videoDepth = vship.SamplingFormatUInt8
	case 9:
		videoDepth = vship.SamplingFormatUInt9
	case 10:
		videoDepth = vship.SamplingFormatUInt10
	case 12:
		videoDepth = vship.SamplingFormatUInt12
	case 14:
		videoDepth = vship.SamplingFormatUInt14
	case 16:
		videoDepth = vship.SamplingFormatUInt16
	default:
		panic("UNKNOWN PIXEL FORMAT")
	}

	colorspace.SamplingFormat = videoDepth

	if video.firstFrame.ColorRange == int(gopixfmts.ColorRangeMPEG) || (video.firstFrame.ColorRange == 0) {
		colorspace.ColorRange = vship.ColorRangeLimited
	} else {
		colorspace.ColorRange = vship.ColorRangeFull
	}

	colorspace.ChromaSubsamplingHeight = videoPixelFormat.Log2ChromaH()
	colorspace.ChromaSubsamplingWidth = videoPixelFormat.Log2ChromaW()

	if video.firstFrame.ChromaLocation > 0 {
		colorspace.ChromaLocation = vship.ChromaLocation(video.firstFrame.ChromaLocation)
	} else {
		colorspace.ChromaLocation = 1
	}

	if 0 == (videoPixelFormat.Flags() & uint64(gopixfmts.PixFmtFlagRGB)) {
		colorspace.ColorFamily = vship.ColorFamilyYUV
	} else {
		colorspace.ColorFamily = vship.ColorFamilyRGB
	}

	if video.firstFrame.ColorSpace > 0 {
		colorspace.ColorMatrix = vship.ColorMatrix(video.firstFrame.ColorSpace)
	} else {
		if colorspace.ColorFamily == vship.ColorFamilyYUV {
			colorspace.ColorMatrix = 1
		} else {
			colorspace.ColorMatrix = 0
		}
	}

	if video.firstFrame.TransferCharateristics > 0 {
		colorspace.ColorTransfer = vship.ColorTransfer(video.firstFrame.TransferCharateristics)
	} else {
		colorspace.ColorTransfer = 1
	}

	if video.firstFrame.ColorPrimaries > 0 {
		colorspace.ColorPrimaries = vship.ColorPrimaries(video.firstFrame.ColorPrimaries)
	} else {
		colorspace.ColorPrimaries = 1
	}

	colorspace.CropTop, colorspace.CropBottom, colorspace.CropLeft, colorspace.CropRight = 0, 0, 0, 0

	return colorspace, nil
}
