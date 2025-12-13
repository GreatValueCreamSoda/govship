package govship

// #include <VshipColor.h>
// #include <stdint.h>
import "C"

// SamplingFormat describes how pixel values are stored in memory. any non
// whole byte value is rounded up to a whole byte. EX: Uint10 is represented
// as a uint16 in memory.
type SamplingFormat int

const (
	SamplingFormatFloat  SamplingFormat = C.Vship_SampleFLOAT
	SamplingFormatHalf   SamplingFormat = C.Vship_SampleHALF
	SamplingFormatUInt8  SamplingFormat = C.Vship_SampleUINT8
	SamplingFormatUInt9  SamplingFormat = C.Vship_SampleUINT9
	SamplingFormatUInt10 SamplingFormat = C.Vship_SampleUINT10
	SamplingFormatUInt12 SamplingFormat = C.Vship_SampleUINT12
	SamplingFormatUInt14 SamplingFormat = C.Vship_SampleUINT14
	SamplingFormatUInt16 SamplingFormat = C.Vship_SampleUINT16
)

// ColorRange indicates whether the image uses limited (TV) or full (PC) range.
//
// Limited range typically maps black/white to 16–235; full range uses 0–255
// for 8bit images
type ColorRange int

const (
	ColorRangeLimited ColorRange = C.Vship_RangeLimited
	ColorRangeFull    ColorRange = C.Vship_RangeFull
)

// ChromaLocation specifies the relative position of chroma (U/V) samples
// within a subsampled image plane.
//
// This is relevant only if ChromaSubsamplingWidth/Height > 1.
type ChromaLocation int

const (
	ChromaLocationLeft    = C.Vship_ChromaLoc_Left
	ChromaLocationCenter  = C.Vship_ChromaLoc_Center
	ChromaLocationTopLeft = C.Vship_ChromaLoc_TopLeft
	ChromaLocationTop     = C.Vship_ChromaLoc_Top
)

// ColorFamily identifies whether an image uses RGB or YUV color channels.
//
// RGB images have independent red, green, and blue channels.
// YUV images have a luma (Y) channel and two chroma channels (U, V).
type ColorFamily int

const (
	ColorFamilyYUV ColorFamily = C.Vship_ColorYUV
	ColorFamilyRGB ColorFamily = C.Vship_ColorRGB
)

// ColorMatrix defines how YUV values are mapped to RGB or vice versa.
type ColorMatrix int

const (
	ColorMatrixRGB         ColorMatrix = C.Vship_MATRIX_RGB
	ColorMatrixBT709       ColorMatrix = C.Vship_MATRIX_BT709
	ColorMatrixBT470BG     ColorMatrix = C.Vship_MATRIX_BT470_BG
	ColorMatrixST170M      ColorMatrix = C.Vship_MATRIX_ST170_M
	ColorMatrixBT2020NCL   ColorMatrix = C.Vship_MATRIX_BT2020_NCL
	ColorMatrixBT2020CL    ColorMatrix = C.Vship_MATRIX_BT2020_CL
	ColorMatrixBT2100ICTCP ColorMatrix = C.Vship_MATRIX_BT2100_ICTCP
)

// ColorTransfer specifies the transfer function (gamma or PQ/HLG curve) used
// for the image.
//
// This affects perceptual computations and linearization for quality metrics.
type ColorTransfer int

const (
	ColorTransferTRCBT709    ColorTransfer = C.Vship_TRC_BT709
	ColorTransferTRCBT470_M  ColorTransfer = C.Vship_TRC_BT470_M
	ColorTransferTRCBT470_BG ColorTransfer = C.Vship_TRC_BT470_BG
	ColorTransferTRCBT601    ColorTransfer = C.Vship_TRC_BT601
	ColorTransferTRCLinear   ColorTransfer = C.Vship_TRC_Linear
	ColorTransferTRCSRGB     ColorTransfer = C.Vship_TRC_sRGB
	ColorTransferTRCPQ       ColorTransfer = C.Vship_TRC_PQ
	ColorTransferTRCST428    ColorTransfer = C.Vship_TRC_ST428
	ColorTransferTRCHLG      ColorTransfer = C.Vship_TRC_HLG
)

// ColorPrimaries defines the chromaticity coordinates of the RGB channels.
type ColorPrimaries int

const (
	ColorPrimariesINTERNAL ColorPrimaries = C.Vship_PRIMARIES_INTERNAL
	ColorPrimariesBT709    ColorPrimaries = C.Vship_PRIMARIES_BT709
	ColorPrimariesBT470_M  ColorPrimaries = C.Vship_PRIMARIES_BT470_M
	ColorPrimariesBT470_BG ColorPrimaries = C.Vship_PRIMARIES_BT470_BG
	ColorPrimariesBT2020   ColorPrimaries = C.Vship_PRIMARIES_BT2020
)

// Colorspace contains all information describing an image's format and layout.
//
// This includes geometry, subsampling, color family, matrix, transfer,
// primaries, and optional crop rectangles.
//
// Users should configure Width/Height and SamplingFormat for each image.
// TargetWidth/TargetHeight can be set for automatic resizing during
// processing.
type Colorspace struct {
	Width, Height, TargetWidth, TargetHeight int64
	SamplingFormat                           SamplingFormat
	ColorRange                               ColorRange
	ChromaSubsamplingWidth                   int
	ChromaSubsamplingHeight                  int
	ChromaLocation                           ChromaLocation
	ColorFamily                              ColorFamily
	ColorMatrix                              ColorMatrix
	ColorTransfer                            ColorTransfer
	ColorPrimaries                           ColorPrimaries
	CropTop, CropBottom, CropLeft, CropRight int
}

// toC converts the Go Colorspace into the underlying Vship C struct.
//
// This Should never be called by a user directly. It is used internally by
// handlers to interface with the libvship.
func (c *Colorspace) toC() C.Vship_Colorspace_t {
	return C.Vship_Colorspace_t{
		width:         C.int64_t(c.Width),
		height:        C.int64_t(c.Height),
		target_width:  C.int64_t(c.TargetWidth),
		target_height: C.int64_t(c.TargetHeight),
		sample:        C.Vship_Sample_t(c.SamplingFormat),
		_range:        C.Vship_Range_t(c.ColorRange),
		subsampling: C.Vship_ChromaSubsample_t{
			subw: C.int(c.ChromaSubsamplingWidth),
			subh: C.int(c.ChromaSubsamplingHeight),
		},
		chromaLocation:   C.Vship_ChromaLocation_t(c.ChromaLocation),
		colorFamily:      C.Vship_ColorFamily_t(c.ColorFamily),
		YUVMatrix:        C.Vship_YUVMatrix_t(c.ColorMatrix),
		transferFunction: C.Vship_TransferFunction_t(c.ColorTransfer),
		primaries:        C.Vship_Primaries_t(c.ColorPrimaries),
		crop: C.Vship_CropRectangle_t{
			top:    C.int(c.CropTop),
			bottom: C.int(c.CropBottom),
			left:   C.int(c.CropLeft),
			right:  C.int(c.CropRight),
		},
	}
}

// SetDefaults fills the Colorspace with reasonable default values for a given
// resolution and sampling format.
//
// The defaults include limited range YUV, 4:2:0 subsampling, BT.709 matrix /
// transfer / primaries, no cropping, and a TargetWidth/Height of -1 (no
// resizing).
//
// This is useful for quickly configuring common image formats before using
// them in quality metrics or conversions.
func (c *Colorspace) SetDefaults(width, height int64, format SamplingFormat) {
	c.Width = width
	c.Height = height
	c.TargetWidth = -1
	c.TargetHeight = -1
	c.SamplingFormat = format
	c.ColorRange = ColorRangeLimited
	c.ChromaSubsamplingWidth = 1
	c.ChromaSubsamplingHeight = 1
	c.ColorFamily = ColorFamilyYUV
	c.ColorMatrix = ColorMatrixBT709
	c.ColorTransfer = ColorTransferTRCBT709
	c.ColorPrimaries = ColorPrimariesBT709
	c.CropTop, c.CropBottom, c.CropLeft, c.CropRight = 0, 0, 0, 0
}
