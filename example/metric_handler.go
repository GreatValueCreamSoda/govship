package main

import (
	"fmt"
	"os"

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
	pool        BlockingPool[*vship.ButteraugliHandler]
	handlerList []*vship.ButteraugliHandler
}

func (h *ButterHandler) Name() string { return "butter" }

func NewButterHandler(numWorkers int, colorA, colorB *vship.Colorspace,
	qNorm int, displayBrightness float32) (*ButterHandler, error) {
	var handler ButterHandler
	handler.pool = NewBlockingPool[*vship.ButteraugliHandler](numWorkers)

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

func (h *ButterHandler) Compute(a, b *frame) (map[string]float64, error) {
	handler := h.pool.Get()
	defer h.pool.Put(handler)

	var score vship.ButteraugliScore

	exception := handler.ComputeScore(&score, nil, 0,
		a.data, b.data,
		a.lineSize, b.lineSize,
	)
	if !exception.IsNone() {
		return nil, fmt.Errorf("ssimu2 failed: %v", exception.GetError())
	}

	scores := map[string]float64{
		h.Name() + "NormQ": score.NormQ,
		h.Name() + "Norm3": score.Norm3,
		h.Name() + "Inf":   score.NormInf,
	}

	return scores, nil
}

type CVVDPHandler struct {
	pool        BlockingPool[*vship.CVVDPHandler]
	handlerList []*vship.CVVDPHandler

	useTemporal bool
}

func (h *CVVDPHandler) Name() string { return "cvvdp" }

func NewCVVDPHandler(numWorkers int, colorA, colorB *vship.Colorspace,
	cfg *ComparatorConfig) (*CVVDPHandler, error) {

	var handler CVVDPHandler
	handler.pool = NewBlockingPool[*vship.CVVDPHandler](numWorkers)
	handler.useTemporal = cfg.CVVDPUseTemporalScore

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

	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return nil, err
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	err = vship.DisplayModelsToCVVDPJSONFile([]vship.DisplayModel{displayModel},
		tmp.Name())
	if err != nil {
		return nil, err
	}

	for range numWorkers {
		var vsHandler *vship.CVVDPHandler
		var code vship.ExceptionCode

		vsHandler, code = vship.NewCVVDPHandlerWithConfig(
			colorA, colorB, 24, cfg.CVVDPResizeToDisplay, "Custom", tmp.Name())

		if !code.IsNone() {
			handler.Close()
			return nil, fmt.Errorf("cvvdp init failed: %w", code.GetError())
		}

		handler.pool.Put(vsHandler)
		handler.handlerList = append(handler.handlerList, vsHandler)
	}

	return &handler, nil
}

func (h *CVVDPHandler) Close() {
	for _, handler := range h.handlerList {
		if handler != nil {
			handler.Close()
		}
	}
	h.handlerList = nil
}

func (h *CVVDPHandler) Compute(a, b *frame) (map[string]float64,
	error) {
	handler := h.pool.Get()
	defer h.pool.Put(handler)

	var code vship.ExceptionCode
	var score float64

	if h.useTemporal {
		score, code = handler.ComputeScore(nil, 0, a.data, b.data, a.lineSize,
			b.lineSize)
	} else {
		// Spatial-only mode: reset accumulated score, compute only current
		// frame
		code = handler.Reset()
		if !code.IsNone() {
			return nil, fmt.Errorf("cvvdp Reset failed: %w", code.GetError())
		}
		code = handler.ResetScore()
		if !code.IsNone() {
			return nil, fmt.Errorf("cvvdp ResetScore failed: %w",
				code.GetError())
		}
		score, code = handler.ComputeScore(nil, 0, a.data, b.data, a.lineSize,
			b.lineSize)
	}

	if !code.IsNone() {
		return nil, fmt.Errorf("cvvdp Compute failed: %w", code.GetError())
	}

	return map[string]float64{
		"cvvdp": score,
	}, nil
}
