package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"unsafe"

	vship "github.com/GreatValueCreamSoda/govship"
)

// MetricHandler is the interface that every metric must implement
type MetricHandler interface {
	Name() string
	Close()
	Compute(a, b *frame) (map[string]float64, error)
}

type ssimu2Handler struct {
	pool        BlockingPool[*vship.SSIMU2Handler]
	handlerList []*vship.SSIMU2Handler
}

func (h *ssimu2Handler) Name() string { return "ssimu2" }

func NewSSIMU2Handler(numWorkers int, colorA, colorB *vship.Colorspace) (
	*ssimu2Handler, error) {
	var handler ssimu2Handler
	handler.pool = NewBlockingPool[*vship.SSIMU2Handler](numWorkers)

	for range numWorkers {
		vsHandler, exception := vship.NewSSIMU2Handler(colorA, colorB)
		if !exception.IsNone() {
			defer handler.Close()
			var err error = exception.GetError()
			return nil, fmt.Errorf("ssimu2 init failed: %w", err)
		}
		handler.pool.Put(vsHandler)
		handler.handlerList = append(handler.handlerList, vsHandler)
	}

	return &handler, nil
}

func (h *ssimu2Handler) Close() {
	for _, handler := range h.handlerList {
		if handler != nil {
			handler.Close()
		}
	}
	h.handlerList = nil
}

func (h *ssimu2Handler) Compute(a, b *frame) (map[string]float64, error) {
	handler := h.pool.Get()
	defer h.pool.Put(handler)

	score, exception := handler.ComputeScore(
		a.data, b.data,
		a.lineSize, b.lineSize,
	)
	if !exception.IsNone() {
		return nil, fmt.Errorf("ssimu2 failed: %v", exception.GetError())
	}
	return map[string]float64{h.Name(): score}, nil
}

type ButterHandler struct {
	pool             BlockingPool[*vship.ButteraugliHandler]
	handlerList      []*vship.ButteraugliHandler
	width, height    int
	distortionBuffer []float32
	stdoutDistMap    bool
}

func (h *ButterHandler) Name() string { return "butter" }

func NewButterHandler(numWorkers int, colorA, colorB *vship.Colorspace,
	qNorm int, displayBrightness float32, cfg *ComparatorConfig) (
	*ButterHandler, error) {
	var handler ButterHandler
	handler.pool = NewBlockingPool[*vship.ButteraugliHandler](numWorkers)
	handler.width = int(colorA.TargetWidth)
	handler.height = int(colorA.TargetHeight)
	handler.stdoutDistMap = cfg.outputDistortionMapToStdout

	for range numWorkers {
		vsHandler, exception := vship.NewButteraugliHandler(colorA, colorB,
			qNorm, displayBrightness)
		if !exception.IsNone() {
			defer handler.Close()
			var err error = exception.GetError()
			return nil, fmt.Errorf("butter init failed: %w", err)
		}
		handler.pool.Put(vsHandler)
		handler.handlerList = append(handler.handlerList, vsHandler)
	}

	return &handler, nil
}

func (h *ButterHandler) Close() {
	for _, handler := range h.handlerList {
		if handler != nil {
			handler.Close()
		}
	}
	h.handlerList = nil
}

func (h *ButterHandler) getDistortionBufferAndSize() ([]byte, int64) {
	var dstptr []byte = nil
	var dstStride int64 = 0

	if !h.stdoutDistMap {
		return nil, 0
	}

	dstStride = int64(h.width) * int64(unsafe.Sizeof(float32(0)))
	totalSize := h.width * h.height

	if h.distortionBuffer == nil || len(h.distortionBuffer) != totalSize {
		h.distortionBuffer = make([]float32, totalSize)
	}

	dstptr = unsafe.Slice(
		(*byte)(unsafe.Pointer(&h.distortionBuffer[0])), totalSize*4)

	logf(LogDebug, "CVVDP distortion map: %dx%d, buffer size %d bytes",
		h.width, h.height, len(dstptr))

	return dstptr, dstStride

}

func (h *ButterHandler) Compute(a, b *frame) (map[string]float64, error) {
	handler := h.pool.Get()
	defer h.pool.Put(handler)

	var score vship.ButteraugliScore

	dstptr, dstStride := h.getDistortionBufferAndSize()

	exception := handler.ComputeScore(&score, dstptr, dstStride,
		a.data, b.data,
		a.lineSize, b.lineSize,
	)
	if !exception.IsNone() {
		return nil, fmt.Errorf("butter failed: %w", exception.GetError())
	}

	if dstptr != nil {
		io.Copy(os.Stdout, bytes.NewReader(dstptr))
	}

	scores := map[string]float64{
		h.Name() + "NormQ": score.NormQ,
		h.Name() + "Norm3": score.Norm3,
		h.Name() + "Inf":   score.NormInf,
	}

	return scores, nil
}

type CVVDPHandler struct {
	pool             BlockingPool[*vship.CVVDPHandler]
	handlerList      []*vship.CVVDPHandler
	width, height    int
	distortionBuffer []float32
	stdoutDistMap    bool

	useTemporal bool
}

func (h *CVVDPHandler) Name() string { return "cvvdp" }

func NewCVVDPHandler(numWorkers int, colorA, colorB *vship.Colorspace,
	cfg *ComparatorConfig) (*CVVDPHandler, error) {
	var h CVVDPHandler
	h.pool = NewBlockingPool[*vship.CVVDPHandler](numWorkers)
	h.useTemporal = cfg.CVVDPUseTemporalScore

	if cfg.CVVDPResizeToDisplay {
		h.width, h.height = cfg.DisplayWidth, cfg.DisplayHeight

	} else {
		h.width, h.height = int(colorA.TargetWidth), int(colorA.TargetHeight)

	}

	h.stdoutDistMap = cfg.outputDistortionMapToStdout

	path, err := h.createJsonConfig(cfg)
	if err != nil {
		return nil, err
	}
	defer os.Remove(path)

	for range numWorkers {
		var vsHandler *vship.CVVDPHandler
		var code vship.ExceptionCode

		vsHandler, code = vship.NewCVVDPHandlerWithConfig(
			colorA, colorB, 24, cfg.CVVDPResizeToDisplay, "Custom", path)

		if !code.IsNone() {
			h.Close()
			return nil, fmt.Errorf("cvvdp init failed: %w", code.GetError())
		}

		h.pool.Put(vsHandler)
		h.handlerList = append(h.handlerList, vsHandler)
	}

	return &h, nil
}

func (CVVDPHandler) createJsonConfig(cfg *ComparatorConfig) (string, error) {
	var displayModel vship.DisplayModel
	displayModel.Name = "Custom"
	displayModel.ColorSpace = vship.DisplayModelColorspaceHDR
	displayModel.DisplayWidth = cfg.DisplayWidth
	displayModel.DisplayHeight = cfg.DisplayHeight
	displayModel.DisplayMaxLuminance = float32(cfg.DisplayBrightness)
	displayModel.DisplayDiagonalSizeInches = float32(cfg.DisplayDiagonal)
	displayModel.ViewingDistanceMeters = float32(cfg.ViewingDistance)
	displayModel.MonitorContrastRatio = cfg.MonitorContrastRatio
	displayModel.AmbientLightLevel = cfg.RoomBrightness
	displayModel.AmbientLightReflectionOnDisplay = 0.005
	displayModel.Exposure = 1

	tmp, e := os.CreateTemp("", "")
	if e != nil {
		return "", e
	}
	defer tmp.Close()

	e = vship.DisplayModelsToCVVDPJSONFile([]vship.DisplayModel{displayModel},
		tmp.Name())
	if e != nil {
		return "", e
	}

	return tmp.Name(), nil
}

func (h *CVVDPHandler) Close() {
	for _, handler := range h.handlerList {
		if handler != nil {
			handler.Close()
		}
	}
	h.handlerList = nil
}

func (h *CVVDPHandler) getDistortionBufferAndSize() ([]byte, int64) {
	var dstptr []byte = nil
	var dstStride int64 = 0

	if !h.stdoutDistMap {
		return nil, 0
	}

	dstStride = int64(h.width) * int64(unsafe.Sizeof(float32(0)))
	totalSize := h.width * h.height

	if h.distortionBuffer == nil || len(h.distortionBuffer) != totalSize {
		h.distortionBuffer = make([]float32, totalSize)
	}

	dstptr = unsafe.Slice(
		(*byte)(unsafe.Pointer(&h.distortionBuffer[0])), totalSize*4)

	logf(LogDebug, "CVVDP distortion map: %dx%d, buffer size %d bytes",
		h.width, h.height, len(dstptr))

	return dstptr, dstStride
}

func (h *CVVDPHandler) Compute(a, b *frame) (map[string]float64, error) {
	handler := h.pool.Get()
	defer h.pool.Put(handler)

	var code vship.ExceptionCode
	var score float64

	dstptr, dstStride := h.getDistortionBufferAndSize()

	if h.useTemporal {
		goto Temporal
	} else {
		goto Spatial
	}

Temporal:
	// Were reporting per frame scores, so this has to be reset.
	code = handler.ResetScore()
	if !code.IsNone() {
		return nil, fmt.Errorf("cvvdp ResetScore failed: %w", code.GetError())
	}
	score, code = handler.ComputeScore(dstptr, dstStride, a.data, b.data,
		a.lineSize, b.lineSize)

	goto End

Spatial:
	// Spatial-only mode: reset accumulated score, compute only current
	// frame
	code = handler.Reset()
	if !code.IsNone() {
		return nil, fmt.Errorf("cvvdp Reset failed: %w", code.GetError())
	}
	code = handler.ResetScore()
	if !code.IsNone() {
		return nil, fmt.Errorf("cvvdp ResetScore failed: %w", code.GetError())
	}
	score, code = handler.ComputeScore(dstptr, dstStride, a.data, b.data,
		a.lineSize, b.lineSize)
	goto End

End:

	if dstptr != nil {
		io.Copy(os.Stdout, bytes.NewReader(dstptr))
	}

	if !code.IsNone() {
		return nil, fmt.Errorf("cvvdp Compute failed: %w", code.GetError())
	}

	return map[string]float64{"cvvdp": score}, nil
}
