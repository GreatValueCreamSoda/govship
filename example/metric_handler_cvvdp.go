package main

import (
	"fmt"
	"os"
	"unsafe"

	vship "github.com/GreatValueCreamSoda/govship"
)

var cvvdpName string = "CVVDP"

type CVVDPHandler struct {
	pool             BlockingPool[*vship.CVVDPHandler]
	handlerList      []*vship.CVVDPHandler
	width, height    int
	distortionBuffer []float32
	ffmpegCmd        *ffmpegHeatmap

	useTemporal bool
}

func (h *CVVDPHandler) Name() string { return "cvvdp" }

func NewCVVDPHandler(numWorkers int, colorA, colorB *vship.Colorspace,
	cfg *ComparatorConfig) (*CVVDPHandler, error) {
	var h CVVDPHandler
	var err error

	h.pool = NewBlockingPool[*vship.CVVDPHandler](numWorkers)
	h.useTemporal = cfg.CVVDPUseTemporalScore

	if cfg.CVVDPResizeToDisplay {
		h.width, h.height = cfg.DisplayWidth, cfg.DisplayHeight
	} else {
		h.width, h.height = int(colorA.TargetWidth), int(colorA.TargetHeight)
	}

	if cfg.ButteraugliDistMapVideo == "" {
		goto SKIPDISTMAP
	}

	h.ffmpegCmd, err = newFFmpegHeatmap(h.width, h.height, 25,
		cfg.DistortionMapEncoderSettings, cfg.CVVDPDistMapVideo,
		float32(cfg.CVVDPMaxDistortionClipping))
	if err != nil {
		return nil, err
	}

SKIPDISTMAP:

	path, err := h.createJsonConfig(cfg)
	if err != nil {
		return nil, err
	}
	defer os.Remove(path)

	for range numWorkers {
		err = h.createWorker(colorA, colorB, cfg, path)
		if err == nil {
			continue
		}
		defer h.Close()
		return nil, err
	}

	return &h, nil
}

func (h *CVVDPHandler) createWorker(colorA, colorB *vship.Colorspace,
	cfg *ComparatorConfig, path string) error {
	vsHandler, exception := vship.NewCVVDPHandlerWithConfig(
		colorA, colorB, 24, cfg.CVVDPResizeToDisplay, "Custom", path)

	if exception.IsNone() {
		h.pool.Put(vsHandler)
		h.handlerList = append(h.handlerList, vsHandler)
		return nil
	}
	return fmt.Errorf("%s initialization failed with error: %w", cvvdpName,
		exception.GetError())
}

func (h *CVVDPHandler) getDistortionBufferAndSize() ([]byte, int64) {
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

	dstptr = unsafe.Slice(
		(*byte)(unsafe.Pointer(&h.distortionBuffer[0])), totalSize*4)

	logf(LogDebug, "%s dist map: %dx%d, buffer size %d bytes", cvvdpName,
		h.width, h.height, len(dstptr))

	return dstptr, dstStride
}

func (h *CVVDPHandler) Compute(a, b *frame) (map[string]float64, error) {
	handler := h.pool.Get()
	defer h.pool.Put(handler)

	var code vship.ExceptionCode
	var score float64

	dstptr, dstStride := h.getDistortionBufferAndSize()

	if !h.useTemporal {
		goto Spatial
	}

	// Were reporting per frame scores, so this has to be reset.
	// We might want to add a flag to enable or disable this.
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

	if h.ffmpegCmd != nil {
		h.ffmpegCmd.WriteDistortion(dstptr, dstStride)
	}

	if !code.IsNone() {
		return nil, fmt.Errorf("%s failed to compute score with error: %w",
			butterName, code.GetError())
	}

	return map[string]float64{"cvvdp": score}, nil
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
