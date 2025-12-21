package main

import (
	"github.com/GreatValueCreamSoda/gopixfmts"
	vship "github.com/GreatValueCreamSoda/govship"
)

// Does nothing but convert a ffms2 frame data into a vship.Colorspace

func getVideoColorspace(video *openedVideo) (vship.Colorspace, error) {
	logf(LogInfo, "Determining colorspace from video properties")

	var colorspace vship.Colorspace

	colorspace.Width = int64(video.firstFrame.ScaledWidth)
	colorspace.TargetWidth = colorspace.Width
	colorspace.Height = int64(video.firstFrame.ScaledHeight)
	colorspace.TargetHeight = colorspace.Height

	logf(LogDebug, "Video dimensions: %dx%d (scaled)", colorspace.Width,
		colorspace.Height)

	videoPixelFormat, err := gopixfmts.PixFmtDescGet(gopixfmts.PixelFormat(
		video.firstFrame.ConvertedPixelFormat))
	if err != nil {
		logf(LogError, "Failed to get pixel format descriptor for Converted"+
			"PixelFormat=%d: %v", video.firstFrame.ConvertedPixelFormat, err)
		return colorspace, err
	}

	logf(LogDebug, "Pixel format: %s", videoPixelFormat.Name())

	comp, err := videoPixelFormat.Component(0)
	if err != nil {
		logf(LogError, "Failed to get component 0 from pixel format: %v", err)
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
		logf(LogError, "Unsupported bit depth %d in pixel format %s",
			comp.Depth, videoPixelFormat.Name())
		panic("UNKNOWN PIXEL FORMAT")
	}

	colorspace.SamplingFormat = videoDepth
	logf(LogDebug, "Bit depth determined: %d-bit", comp.Depth)

	if video.firstFrame.ColorRange == int(gopixfmts.ColorRangeMPEG) ||
		video.firstFrame.ColorRange == 0 {
		colorspace.ColorRange = vship.ColorRangeLimited
		logf(LogDebug, "Color range: Limited (MPEG/TV range)")
	} else {
		colorspace.ColorRange = vship.ColorRangeFull
		logf(LogDebug, "Color range: Full (PC range)")
	}

	colorspace.ChromaSubsamplingHeight = videoPixelFormat.Log2ChromaH()
	colorspace.ChromaSubsamplingWidth = videoPixelFormat.Log2ChromaW()
	logf(LogDebug, "Chroma subsampling: %d:%d (log2 W/H = %d/%d)",
		1<<colorspace.ChromaSubsamplingWidth,
		1<<colorspace.ChromaSubsamplingHeight,
		colorspace.ChromaSubsamplingWidth, colorspace.ChromaSubsamplingHeight)

	if video.firstFrame.ChromaLocation > 0 {
		colorspace.ChromaLocation = vship.ChromaLocation(
			video.firstFrame.ChromaLocation)
		logf(LogDebug, "Chroma location: explicit value %d",
			video.firstFrame.ChromaLocation)
	} else {
		colorspace.ChromaLocation = 1 // Default assumption
		logf(LogDebug, "Chroma location: defaulting to 1 (likely left/center)")
	}

	if videoPixelFormat.Flags()&uint64(gopixfmts.PixFmtFlagRGB) == 0 {
		colorspace.ColorFamily = vship.ColorFamilyYUV
		logf(LogDebug, "Color family: YUV")
	} else {
		colorspace.ColorFamily = vship.ColorFamilyRGB
		logf(LogDebug, "Color family: RGB")
	}

	if video.firstFrame.ColorSpace > 0 {
		colorspace.ColorMatrix = vship.ColorMatrix(video.firstFrame.ColorSpace)
		logf(LogDebug,
			"Color matrix: explicit value %d", video.firstFrame.ColorSpace)
	} else {
		if colorspace.ColorFamily == vship.ColorFamilyYUV {
			colorspace.ColorMatrix = 1 // BT.709 assumed
			logf(LogDebug,
				"Color matrix: defaulting to BT.709 (1) for YUV")
		} else {
			colorspace.ColorMatrix = 0
			logf(LogDebug,
				"Color matrix: defaulting to unspecified (0) for RGB")
		}
	}

	if video.firstFrame.TransferCharateristics > 0 {
		colorspace.ColorTransfer = vship.ColorTransfer(
			video.firstFrame.TransferCharateristics)
		logf(LogDebug, "Transfer characteristics: explicit value %d",
			video.firstFrame.TransferCharateristics)
	} else {
		colorspace.ColorTransfer = 1 // BT.709
		logf(LogDebug, "Transfer characteristics: defaulting to BT.709 (1)")
	}

	if video.firstFrame.ColorPrimaries > 0 {
		colorspace.ColorPrimaries = vship.ColorPrimaries(
			video.firstFrame.ColorPrimaries)
		logf(LogDebug, "Color primaries: explicit value %d",
			video.firstFrame.ColorPrimaries)
	} else {
		colorspace.ColorPrimaries = 1 // BT.709
		logf(LogDebug, "Color primaries: defaulting to BT.709 (1)")
	}

	colorspace.CropTop, colorspace.CropBottom, colorspace.CropLeft = 0, 0, 0
	colorspace.CropRight = 0

	logf(LogInfo, "Colorspace determined successfully: %dx%d %t %t %v-bit %s",
		colorspace.Width, colorspace.Height, colorspace.ColorFamily,
		colorspace.ColorRange, comp.Depth, videoPixelFormat.Name())

	return colorspace, nil
}
