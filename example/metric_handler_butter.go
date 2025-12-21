package main

import (
	"fmt"
	"unsafe"

	vship "github.com/GreatValueCreamSoda/govship"
)

const butterName string = "Butteraugli"

type ButterHandler struct {
	pool             BlockingPool[*vship.ButteraugliHandler]
	handlerList      []*vship.ButteraugliHandler
	width, height    int
	distortionBuffer []float32

	ffmpegCmd *ffmpegHeatmap
}

func (h *ButterHandler) Name() string { return "butter" }

func NewButterHandler(numWorkers int, colorA, colorB *vship.Colorspace,
	cfg *ComparatorConfig) (
	*ButterHandler, error) {
	var h ButterHandler
	var err error

	h.pool = NewBlockingPool[*vship.ButteraugliHandler](numWorkers)
	h.width, h.height = int(colorA.TargetWidth), int(colorA.TargetHeight)

	if cfg.ButteraugliDistMapVideo == "" {
		goto SKIPDISTMAP
	}

	if numWorkers > 1 {
		logf(LogError, ">1 numWorkers was specifiedf with a distortion "+
			"map video output. was this a mistake?")
	}

	h.ffmpegCmd, err = newFFmpegHeatmap(h.width, h.height, 25,
		cfg.DistortionMapEncoderSettings, cfg.ButteraugliDistMapVideo,
		float32(cfg.ButteraugliMaxDistortionClipping))
	if err != nil {
		return nil, err
	}

SKIPDISTMAP:

	for range numWorkers {
		err = h.createWorker(colorA, colorB, cfg)
		if err == nil {
			continue
		}
		defer h.Close()
		return nil, err
	}

	return &h, nil
}

func (h *ButterHandler) createWorker(colorA, colorB *vship.Colorspace,
	cfg *ComparatorConfig) error {
	vsHandler, exception := vship.NewButteraugliHandler(colorA, colorB,
		cfg.ButteraugliQNorm, float32(cfg.DisplayBrightness))
	if exception.IsNone() {
		h.pool.Put(vsHandler)
		h.handlerList = append(h.handlerList, vsHandler)
		return nil
	}
	return fmt.Errorf("%s initialization failed with error: %w", butterName,
		exception.GetError())
}

func (h *ButterHandler) getDistortionBufferAndSize() ([]byte, int64) {
	var dstptr []byte = nil
	var dstStride int64 = 0

	if h.ffmpegCmd == nil {
		return nil, 0
	}

	dstStride = int64(h.width) * int64(unsafe.Sizeof(float32(0)))
	totalSize := h.width * h.height

	if h.distortionBuffer == nil || len(h.distortionBuffer) != totalSize {
		h.distortionBuffer = make([]float32, totalSize)
	}

	dstptr = unsafe.Slice((*byte)(unsafe.Pointer(&h.distortionBuffer[0])),
		totalSize*4)

	logf(LogDebug, "%s dist map: %dx%d, buffer size %d bytes", butterName,
		h.width, h.height, len(dstptr))

	return dstptr, dstStride

}

func (h *ButterHandler) Compute(a, b *frame) (map[string]float64, error) {
	handler := h.pool.Get()
	defer h.pool.Put(handler)

	var score vship.ButteraugliScore

	dstptr, dstStride := h.getDistortionBufferAndSize()

	exception := handler.ComputeScore(&score, dstptr, dstStride, a.data,
		b.data, a.lineSize, b.lineSize)
	if !exception.IsNone() {
		return nil, fmt.Errorf("%s failed to compute score with error: %w",
			butterName, exception.GetError())
	}

	if h.ffmpegCmd != nil {
		h.ffmpegCmd.WriteDistortion(dstptr, dstStride)

	}

	scores := map[string]float64{
		butterName + "NormQ": score.NormQ, butterName + "Norm3": score.Norm3,
		butterName + "Inf": score.NormInf,
	}

	return scores, nil
}

func (h *ButterHandler) Close() {
	for _, handler := range h.handlerList {
		if handler != nil {
			handler.Close()
		}
	}
	h.handlerList = nil

	if h.ffmpegCmd != nil {
		h.ffmpegCmd.Close()
	}
}
