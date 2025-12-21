package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"unsafe"
)

// This is not a metric handler. its a helper for saving distortion maps for
// metrics that do infact support outputting them.

type ffmpegHeatmap struct {
	ffmpegCmd  *exec.Cmd
	ffmpegPipe io.WriteCloser
	videoPath  string
	maxDist    float32
}

func newFFmpegHeatmap(width, height int, frameRate float32, settings []string,
	outputPath string, maxVal float32) (*ffmpegHeatmap, error) {
	var heatmap ffmpegHeatmap
	heatmap.maxDist = maxVal
	heatmap.videoPath = outputPath
	frameRateString := strconv.FormatFloat(float64(frameRate), 'f', 2, 64)
	resolution := fmt.Sprintf("%dx%d", width, height)
	heatmapFilter := "format=rgb24,pseudocolor=p=heat"

	args := append([]string{
		"-y", "-f", "rawvideo", "-pixel_format", "grayf32le", "-s", resolution,
		"-r", frameRateString, "-i", "-", "-vf", heatmapFilter, "-pix_fmt",
		"yuv420p"}, append(settings, outputPath)...)

	heatmap.ffmpegCmd = exec.Command("ffmpeg", args...)

	var err error

	heatmap.ffmpegPipe, err = heatmap.ffmpegCmd.StdinPipe()

	if err != nil {
		return nil, fmt.Errorf("ffmpeg stdin pipe failed: %w", err)
	}

	if err = heatmap.ffmpegCmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg start failed: %w", err)
	}

	logf(LogInfo, "Distortion heatmap video will be saved to %s", outputPath)

	return &heatmap, nil
}

func (h *ffmpegHeatmap) WriteDistortion(dstptr []byte, dstStride int64) error {
	if dstStride == 0 || dstptr == nil || h.ffmpegCmd == nil {
		return nil
	}

	distortionBuffer := unsafe.Slice((*float32)(unsafe.Pointer(&dstptr[0])),
		len(dstptr)/4)

	for i := 0; i < len(distortionBuffer); i++ {
		distortionBuffer[i] = distortionBuffer[i] / h.maxDist
	}

	_, err := io.Copy(h.ffmpegPipe, bytes.NewReader(dstptr))

	if err != nil {
		logf(LogError, "Failed to write distortion heatmap to ffmpeg: %v", err)
	}

	return err
}

func (h *ffmpegHeatmap) Close() {
	if h.ffmpegPipe != nil {
		h.ffmpegPipe.Close()
	}
	err := h.ffmpegCmd.Wait()
	if err != nil {
		logf(LogError, "FFmpeg failed to save distortion map (%s): %v",
			h.videoPath, err)
	} else {
		logf(LogInfo, "Heatmap video saved to path: \"%s\"", h.videoPath)
	}
}
