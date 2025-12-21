package main

import (
	"fmt"

	vship "github.com/GreatValueCreamSoda/govship"
)

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
