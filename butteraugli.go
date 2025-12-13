package govship

/*
#include <VshipAPI.h>
#include <stdlib.h>
#include "flattened.h"
*/
import "C"
import "unsafe"

// ButteraugliHandler evaluates visual differences between two images using the
// Butteraugli perceptual metric.
//
// A ButteraugliHandler is configured for a specific image format and viewing
// setup. Once created, it can be reused to score many frame pairs that share
// the same geometry and colorspace.
//
// Each score is computed independently. The handler does not accumulate
// history and does not retain information between calls to ComputeScore.
type ButteraugliHandler struct {
	ptr  *C.Vship_ButteraugliHandler
	init bool
}

// ButteraugliScore contains the results of a Butteraugli comparison.
//
// The values represent different ways of summarizing perceived distortion:
//
//   - NormQ is the X norm of all per pixel scores specified when the
//     handler was created.
//   - Norm3 emphasizes structured and spatially correlated errors.
//   - NormInf is the distance value of the sigle worse pixel of the distortion
//     relative to the source.
//
// Butteraugli Computes a distance score per pixel. Where 1 means 1k pixels in
// distance. This distance is how far the viewer has to be from the distortion
// for it to look identical to the source images in a in place image swap.
type ButteraugliScore struct {
	NormQ   float64
	Norm3   float64
	NormInf float64
}

// NewButteraugliHandler creates a Butteraugli evaluator for a specific image
// format and display brightness.
//
// src and dst describe the format of the reference and distorted images. All
// frames scored with this handler must match these formats.
//
// Qnorm controls how aggressively differences are weighted perceptually.
// DisplayBrightnessInNits defines the assumed peak brightness of the display
// used when interpreting visual differences.
//
// The returned handler can be reused for multiple comparisons and should be
// closed when no longer needed.
func NewButteraugliHandler(src, dst *Colorspace, Qnorm int,
	DisplayBrightnessInNits float32) (*ButteraugliHandler, ExceptionCode) {
	var handler ButteraugliHandler
	var h C.Vship_ButteraugliHandler

	code := ExceptionCode(C.Vship_ButteraugliInit(&h, src.toC(), dst.toC(),
		C.int(Qnorm), C.float(DisplayBrightnessInNits)))
	if !code.IsNone() {
		return nil, code
	}

	handler.ptr = &h
	handler.init = true
	return &handler, code
}

// ComputeScore compares a reference image against a distorted image and
// produces Butteraugli quality metrics.
//
// src1 and src2 contain the reference and distorted image planes. Line sizes
// describe the byte stride for each plane. All inputs must match the format
// specified when the handler was created.
//
// If dst is non-nil, a per-pixel distortion map is written to it using
// dstStride bytes per row. If dst is nil, no distortion map is produced.
// The returned distortion map is the computed distance per pixel represetned
// as a float32 value. The resolution of the map is identical to the largest
// plane of the source image.
//
// On success, score is populated with the computed quality metrics.
func (handler *ButteraugliHandler) ComputeScore(
	score *ButteraugliScore, dst []byte, dstStride int64, src1, src2 [3][]byte,
	srcLineSize1, srcLineSize2 [3]int64) ExceptionCode {

	s0 := planePtr(src1[0])
	s1 := planePtr(src1[1])
	s2 := planePtr(src1[2])

	d0 := planePtr(src2[0])
	d1 := planePtr(src2[1])
	d2 := planePtr(src2[2])

	var cScore C.Vship_ButteraugliScore

	dstPtr := planePtr(dst)

	code := C.ComputeButteraugli_flat(
		(*C.Vship_ButteraugliHandler)(unsafe.Pointer(handler.ptr)),
		&cScore,
		dstPtr,
		C.int64_t(dstStride),
		s0, s1, s2,
		d0, d1, d2,
		C.int64_t(srcLineSize1[0]), C.int64_t(srcLineSize1[1]),
		C.int64_t(srcLineSize1[2]),
		C.int64_t(srcLineSize2[0]), C.int64_t(srcLineSize2[1]),
		C.int64_t(srcLineSize2[2]),
	)

	if code == 0 {
		*score = ButteraugliScore{float64(cScore.normQ), float64(cScore.norm3),
			float64(cScore.norminf)}
	}

	return ExceptionCode(code)
}

// Close releases the resources associated with the handler.
//
// After Close is called, the handler must not be used again. Calling Close
// multiple times is safe and has no effect after the first call.
func (handler *ButteraugliHandler) Close() ExceptionCode {
	if handler.ptr != nil && handler.init {
		handler.init = false
		code := ExceptionCode(C.Vship_ButteraugliFree(*handler.ptr))
		handler.ptr = nil
		return code
	}
	return ExceptionCodeNoError
}
