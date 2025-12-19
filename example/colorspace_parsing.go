package main

import (
	"github.com/GreatValueCreamSoda/gopixfmts"
	vship "github.com/GreatValueCreamSoda/govship"
)

func getVideoColorspace(video *openedVideo) (vship.Colorspace, error) {
	logf(logInfo, "Determining colorspace from video properties")

	var colorspace vship.Colorspace

	colorspace.Width = int64(video.firstFrame.ScaledWidth)
	colorspace.TargetWidth = colorspace.Width
	colorspace.Height = int64(video.firstFrame.ScaledHeight)
	colorspace.TargetHeight = colorspace.Height

	logf(logDebug, "Video dimensions: %dx%d (scaled)", colorspace.Width,
		colorspace.Height)

	videoPixelFormat, err := gopixfmts.PixFmtDescGet(gopixfmts.PixelFormat(
		video.firstFrame.ConvertedPixelFormat))
	if err != nil {
		logf(logError, "Failed to get pixel format descriptor for Converted"+
			"PixelFormat=%d: %v", video.firstFrame.ConvertedPixelFormat, err)
		return colorspace, err
	}

	logf(logDebug, "Pixel format: %s", videoPixelFormat.Name())

	comp, err := videoPixelFormat.Component(0)
	if err != nil {
		logf(logError, "Failed to get component 0 from pixel format: %v", err)
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
		logf(logError, "Unsupported bit depth %d in pixel format %s",
			comp.Depth, videoPixelFormat.Name())
		panic("UNKNOWN PIXEL FORMAT")
	}

	colorspace.SamplingFormat = videoDepth
	logf(logDebug, "Bit depth determined: %d-bit", comp.Depth)

	if video.firstFrame.ColorRange == int(gopixfmts.ColorRangeMPEG) ||
		video.firstFrame.ColorRange == 0 {
		colorspace.ColorRange = vship.ColorRangeLimited
		logf(logDebug, "Color range: Limited (MPEG/TV range)")
	} else {
		colorspace.ColorRange = vship.ColorRangeFull
		logf(logDebug, "Color range: Full (PC range)")
	}

	colorspace.ChromaSubsamplingHeight = videoPixelFormat.Log2ChromaH()
	colorspace.ChromaSubsamplingWidth = videoPixelFormat.Log2ChromaW()
	logf(logDebug, "Chroma subsampling: %d:%d (log2 W/H = %d/%d)",
		1<<colorspace.ChromaSubsamplingWidth,
		1<<colorspace.ChromaSubsamplingHeight,
		colorspace.ChromaSubsamplingWidth, colorspace.ChromaSubsamplingHeight)

	if video.firstFrame.ChromaLocation > 0 {
		colorspace.ChromaLocation = vship.ChromaLocation(
			video.firstFrame.ChromaLocation)
		logf(logDebug, "Chroma location: explicit value %d",
			video.firstFrame.ChromaLocation)
	} else {
		colorspace.ChromaLocation = 1 // Default assumption
		logf(logDebug, "Chroma location: defaulting to 1 (likely left/center)")
	}

	if videoPixelFormat.Flags()&uint64(gopixfmts.PixFmtFlagRGB) == 0 {
		colorspace.ColorFamily = vship.ColorFamilyYUV
		logf(logDebug, "Color family: YUV")
	} else {
		colorspace.ColorFamily = vship.ColorFamilyRGB
		logf(logDebug, "Color family: RGB")
	}

	if video.firstFrame.ColorSpace > 0 {
		colorspace.ColorMatrix = vship.ColorMatrix(video.firstFrame.ColorSpace)
		logf(logDebug,
			"Color matrix: explicit value %d", video.firstFrame.ColorSpace)
	} else {
		if colorspace.ColorFamily == vship.ColorFamilyYUV {
			colorspace.ColorMatrix = 1 // BT.709 assumed
			logf(logDebug,
				"Color matrix: defaulting to BT.709 (1) for YUV")
		} else {
			colorspace.ColorMatrix = 0
			logf(logDebug,
				"Color matrix: defaulting to unspecified (0) for RGB")
		}
	}

	if video.firstFrame.TransferCharateristics > 0 {
		colorspace.ColorTransfer = vship.ColorTransfer(
			video.firstFrame.TransferCharateristics)
		logf(logDebug, "Transfer characteristics: explicit value %d",
			video.firstFrame.TransferCharateristics)
	} else {
		colorspace.ColorTransfer = 1 // BT.709
		logf(logDebug, "Transfer characteristics: defaulting to BT.709 (1)")
	}

	if video.firstFrame.ColorPrimaries > 0 {
		colorspace.ColorPrimaries = vship.ColorPrimaries(
			video.firstFrame.ColorPrimaries)
		logf(logDebug, "Color primaries: explicit value %d",
			video.firstFrame.ColorPrimaries)
	} else {
		colorspace.ColorPrimaries = 1 // BT.709
		logf(logDebug, "Color primaries: defaulting to BT.709 (1)")
	}

	colorspace.CropTop, colorspace.CropBottom, colorspace.CropLeft = 0, 0, 0
	colorspace.CropRight = 0

	logf(logInfo, "Colorspace determined successfully: %dx%d %t %t %v-bit %s",
		colorspace.Width, colorspace.Height, colorspace.ColorFamily,
		colorspace.ColorRange, comp.Depth, videoPixelFormat.Name())

	return colorspace, nil
}
