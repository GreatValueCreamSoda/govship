package govship

/*
#include <VshipAPI.h>
#include <stdlib.h>
#include "flattened.h"
*/
import "C"
import "unsafe"

type CVVDPHandler struct {
	ptr  *C.Vship_CVVDPHandler
	init bool
}

// NewCVVDPHandler initializes a new CVVDP handler using a built-in display
// model preset.
//
// The modelKey selects one of CVVDP’s internal display model presets (for
// example "standard_fhd", "standard_4k", or other predefined models).
// These presets define the assumed physical display properties and viewing
// conditions used by the perceptual model.
//
// The fps parameter specifies the frame rate of the evaluated content and
// directly affects temporal sensitivity and masking. It must match the
// actual presentation rate of the video sequence for meaningful results.
//
// If resizeToDisplay is true, input frames are resampled to the resolution
// implied by the selected display model before perceptual evaluation.
// This should be enabled when the source resolution does not match the
// intended display resolution. Upscaling to larger resolutions generally
// increases the weight of smaller finer surface detail.
//
// Use this constructor when:
//   - You want CVVDP’s default, reference-calibrated display models
//   - You do not need to override display or viewing-condition parameters
//   - You want scores comparable to standard CVVDP evaluations
//
// For custom or overridden display models, use NewCVVDPHandlerWithConfig.
func NewCVVDPHandler(src, dst *Colorspace, fps float32, resizeToDisplay bool,
	modelKey string) (*CVVDPHandler, ExceptionCode) {
	var h CVVDPHandler
	var cHandler C.Vship_CVVDPHandler
	cModelKey := C.CString(modelKey)
	defer C.free(unsafe.Pointer(cModelKey))

	code := ExceptionCode(C.Vship_CVVDPInit(&cHandler, src.toC(), dst.toC(),
		C.float(fps), C.bool(resizeToDisplay), cModelKey))
	if !code.IsNone() {
		return nil, code
	}

	h.ptr = &cHandler
	h.init = true
	return &h, code
}

// NewCVVDPHandlerWithConfig initializes a new CVVDP handler using a custom
// display model configuration provided as JSON.
//
// The modelKey selects a display model entry inside the provided JSON
// configuration. The configJSON string must contain a valid CVVDP display
// model configuration, typically produced by DisplayModelsToCVVDPJSON.
//
// The JSON configuration may define entirely new display models or override
// specific properties of existing built-in presets. When overriding, CVVDP
// first loads the built-in model referenced by modelKey and then applies
// fields from the JSON on top of it.
//
// This constructor should be used when:
//   - You need to model specific physical displays or viewing environments
//   - You want to override luminance, contrast, ambient light, or geometry
//   - You are running controlled experiments or matching real-world setups
//
// The fps and resizeToDisplay parameters have the same meaning as in
// NewCVVDPHandler and must still reflect the actual viewing conditions.
//
// Incorrect or inconsistent display models will produce perceptually
// misleading scores, even if the API call succeeds. Prefer the built-in
// presets unless you have measured display and viewing data.
func NewCVVDPHandlerWithConfig(
	src, dst *Colorspace, fps float32, resizeToDisplay bool, modelKey,
	configJSON string) (*CVVDPHandler, ExceptionCode) {
	var h CVVDPHandler
	var cHandler C.Vship_CVVDPHandler
	cModelKey := C.CString(modelKey)
	cConfig := C.CString(configJSON)
	defer C.free(unsafe.Pointer(cModelKey))
	defer C.free(unsafe.Pointer(cConfig))

	code := ExceptionCode(C.Vship_CVVDPInit2(&cHandler, src.toC(), dst.toC(),
		C.float(fps), C.bool(resizeToDisplay), cModelKey, cConfig))
	if !code.IsNone() {
		return nil, code
	}

	h.ptr = &cHandler
	h.init = true
	return &h, code
}

// Reset clears the internal temporal frame history maintained by CVVDP.
//
// CVVDP is a temporal metric: it accumulates information across multiple
// frames to model temporal masking and adaptation. Calling Reset discards
// all previously submitted frames and returns the handler to an initial
// temporal state.
//
// This is significantly cheaper than destroying and recreating the handler
// and should be preferred when starting evaluation of a new sequence under
// the same display and viewing conditions.
func (h *CVVDPHandler) Reset() ExceptionCode {
	if h.ptr != nil && h.init {
		return ExceptionCode(C.Vship_ResetCVVDP(*h.ptr))
	}
	return ExceptionCodeNoError
}

// ResetScore clears the accumulated perceptual score without clearing the
// temporal frame history.
//
// This is useful when evaluating multiple segments or scenes within a
// continuous sequence, where temporal adaptation should persist but scores
// must be reported independently.
//
// Unlike Reset, this function preserves the internal temporal filters and
// adaptation state.
func (h *CVVDPHandler) ResetScore() ExceptionCode {
	if h.ptr != nil && h.init {
		return ExceptionCode(C.Vship_ResetScoreCVVDP(*h.ptr))
	}
	return ExceptionCodeNoError
}

// LoadTemporal feeds frames into the CVVDP temporal filter without
// contributing to the reported perceptual score.
//
// This allows modeling temporal adaptation by presenting prior frames to the
// metric before evaluating the frames of interest. The submitted frames affect
// internal state (e.g. masking and adaptation) but do not change the
// accumulated score.
//
// This function is typically used to preload past context when evaluating a
// clip extracted from a longer sequence.
func (h *CVVDPHandler) LoadTemporal(src, dst [3][]byte, srcLineSize,
	dstLineSize [3]int64) ExceptionCode {
	s0 := planePtr(src[0])
	s1 := planePtr(src[1])
	s2 := planePtr(src[2])

	d0 := planePtr(dst[0])
	d1 := planePtr(dst[1])
	d2 := planePtr(dst[2])

	return ExceptionCode(C.LoadTemporalCVVDP_flat(
		(*C.Vship_CVVDPHandler)(unsafe.Pointer(h.ptr)),
		s0, s1, s2,
		d0, d1, d2,
		C.int64_t(srcLineSize[0]), C.int64_t(srcLineSize[1]),
		C.int64_t(srcLineSize[2]),
		C.int64_t(dstLineSize[0]), C.int64_t(dstLineSize[1]),
		C.int64_t(dstLineSize[2]),
	))
}

// ComputeScore submits the current frame(s) to CVVDP and returns the
// accumulated perceptual score.
//
// The score represents the perceptual difference over all frames submitted
// since the last ResetScore or Reset call. For typical video evaluation, this
// function is called for every frame, and the score is read only after the
// final frame has been processed.
//
// If dst is non-nil, CVVDP may write a per-pixel distortion map into the
// provided buffer. The buffer must be large enough to hold dstStride * height
// * sizeof(float) bytes, where height depends on the selected display model
// and resize settings.
//
// Passing dst as nil disables distortion map generation and avoids
// the associated overhead.
func (h *CVVDPHandler) ComputeScore(
	dst []byte, dstStride int64, src, distorted [3][]byte, srcLineSize,
	dstLineSize [3]int64) (float64, ExceptionCode) {
	s0 := planePtr(src[0])
	s1 := planePtr(src[1])
	s2 := planePtr(src[2])

	d0 := planePtr(distorted[0])
	d1 := planePtr(distorted[1])
	d2 := planePtr(distorted[2])

	var score C.double
	dstPtr := planePtr(dst)

	code := C.ComputeCVVDP_flat(
		(*C.Vship_CVVDPHandler)(unsafe.Pointer(h.ptr)),
		&score,
		dstPtr,
		C.int64_t(dstStride),
		s0, s1, s2,
		d0, d1, d2,
		C.int64_t(srcLineSize[0]), C.int64_t(srcLineSize[1]),
		C.int64_t(srcLineSize[2]),
		C.int64_t(dstLineSize[0]), C.int64_t(dstLineSize[1]),
		C.int64_t(dstLineSize[2]),
	)
	return float64(score), ExceptionCode(code)
}

// Close releases all native resources associated with the CVVDP handler.
//
// After Close is called, the handler must not be used again. Calling Close
// multiple times is safe and has no effect after the first successful call.
func (h *CVVDPHandler) Close() ExceptionCode {
	if h.ptr != nil && h.init {
		h.init = false
		code := ExceptionCode(C.Vship_CVVDPFree(*h.ptr))
		h.ptr = nil
		return code
	}
	return ExceptionCodeNoError
}
