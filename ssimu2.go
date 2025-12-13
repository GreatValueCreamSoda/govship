package govship

// #include <VshipAPI.h>
// #include <stdlib.h>
// #include "flattened.h"
import "C"
import (
	"unsafe"
)

// SSIMU2Handler evaluates structural similarity between two images using the
// SSIU2 perceptual metric.
//
// A SSIMU2Handler is configured for specific source and distorted image
// colorspaces and geometry. Once created, it can be reused to score many
// frame pairs that share the same layout.
//
// Each score is computed independently. The handler does not accumulate
// history and does not retain information between calls to ComputeScore.
type SSIMU2Handler struct {
	ptr  *C.Vship_SSIMU2Handler
	init bool
}

// NewSSIMU2Handler creates a new SSIMU2Handler for the given source and
// distorted colorspaces.
//
// The returned handler can be used to compute SSIM2 scores for multiple
// frames that share the same layout and colorspace.
//
// Returns the handler and an ExceptionCode indicating success or failure.
func NewSSIMU2Handler(source, distortion *Colorspace) (*SSIMU2Handler,
	ExceptionCode) {
	var handler SSIMU2Handler
	var handlerSize C.Vship_SSIMU2Handler
	handler.ptr = (*C.Vship_SSIMU2Handler)(C.malloc(C.size_t(unsafe.Sizeof(
		handlerSize))))

	var code ExceptionCode = ExceptionCode(C.Vship_SSIMU2Init(handler.ptr,
		source.toC(), distortion.toC()))

	if !code.IsNone() {
		handler.Close()
	}

	handler.init = true

	return &handler, code
}

// ComputeScore calculates the SSIU2 score between a source and a distorted
// frame.
//
// sourceData and distortedData are arrays of three planes (YUV or RGB), and
// sourceLineSize/distortedLineSize provide the line sizes for each plane.
//
// Returns the SSIM2 score and an ExceptionCode indicating success or failure.
func (handler *SSIMU2Handler) ComputeScore(sourceData, distortedData [3][]byte,
	sourceLineSize, distortedLineSize [3]int64) (float64, ExceptionCode) {

	s0 := planePtr(sourceData[0])
	s1 := planePtr(sourceData[1])
	s2 := planePtr(sourceData[2])

	d0 := planePtr(distortedData[0])
	d1 := planePtr(distortedData[1])
	d2 := planePtr(distortedData[2])

	var score C.double

	var code C.Vship_Exception = C.ComputeSSIMU2_flat(
		(*C.Vship_SSIMU2Handler)(unsafe.Pointer(handler.ptr)),
		&score,
		s0, s1, s2,
		C.int64_t(sourceLineSize[0]), C.int64_t(sourceLineSize[1]),
		C.int64_t(sourceLineSize[2]),
		d0, d1, d2,
		C.int64_t(distortedLineSize[0]), C.int64_t(distortedLineSize[1]),
		C.int64_t(distortedLineSize[2]),
	)

	return float64(score), ExceptionCode(code)
}

// Close frees all resources associated with the SSIMU2Handler.
//
// After calling Close, the handler should no longer be used. Returns an
// ExceptionCode indicating whether the operation succeeded.
func (handler *SSIMU2Handler) Close() ExceptionCode {
	if handler.ptr != nil {
		defer func() {
			C.free(unsafe.Pointer(handler))
			handler.ptr = nil

		}()
	}
	if handler.init {
		handler.init = false
		return ExceptionCode(C.Vship_SSIMU2Free(*handler.ptr))
	}

	return ExceptionCodeNoError
}
