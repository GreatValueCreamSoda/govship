package main

import (
	"fmt"

	vship "github.com/GreatValueCreamSoda/govship"
)

type ComparatorConfig struct {
	VideoAPath, VideoBPath      string
	AStartIdx, BStartIdx        int
	MaxFrames                   int
	WorkerCount                 int
	Metrics                     []string
	ButteraugliQNorm            int
	DisplayBrightness           float64
	CVVDPUseTemporalScore       bool
	CVVDPResizeToDisplay        bool
	DisplayWidth, DisplayHeight int
	DisplayDiagonal             float64
	ViewingDistance             float64
	MonitorContrastRatio        int
	RoomBrightness              int

	outputDistortionMapToStdout bool
}

func (c *ComparatorConfig) Validate() error {
	logf(LogInfo, "Validating comparator configuration")

	if c.WorkerCount <= 0 {
		logf(LogInfo, "WorkerCount <= 0, defaulting to 1")
		c.WorkerCount = 1
	}
	if len(c.Metrics) == 0 {
		err := fmt.Errorf("at least one metric must be specified")
		logf(LogError, "Validation failed: %v", err)
		return err
	}

	logf(LogInfo, "Configuration validated successfully: WorkerCount=%d, "+
		"Metrics=%v", c.WorkerCount, c.Metrics)
	return nil
}

func (c *ComparatorConfig) OpenVideos() (openedVideo, openedVideo, error) {
	logf(LogInfo, "Opening videos: A='%s', B='%s'", c.VideoAPath, c.VideoBPath)

	videoA, videoB, err := openVideoAAndB(c.VideoAPath, c.VideoBPath)
	if err != nil {
		logf(LogError, "Failed to open videos: %v", err)
		return openedVideo{}, openedVideo{}, err
	}

	logf(LogInfo, "Successfully opened videos: A frames=%d, B frames=%d",
		videoA.props.NumFrames, videoB.props.NumFrames)
	return videoA, videoB, nil
}

func (c *ComparatorConfig) FrameCount(a, b openedVideo) (int, error) {
	logf(LogInfo, "Calculating frame count for comparison")

	maxA := a.props.NumFrames - c.AStartIdx
	maxB := b.props.NumFrames - c.BStartIdx

	logf(LogDebug, "Available frames after start indices: A=%d, B=%d", maxA,
		maxB)

	n := maxA
	if maxB < n {
		n = maxB
		logf(LogDebug, "Limited by video B to %d frames", n)
	}
	if c.MaxFrames > 0 && c.MaxFrames < n {
		n = c.MaxFrames
		logf(LogDebug, "Limited by MaxFrames config to %d frames", n)
	}
	if n <= 0 {
		err := fmt.Errorf("no frames to compare")
		logf(LogError, "Frame count calculation resulted in zero frames: "+
			"AStartIdx=%d, BStartIdx=%d, MaxFrames=%d", c.AStartIdx,
			c.BStartIdx, c.MaxFrames)
		return 0, err
	}

	logf(LogInfo, "Will compare %d frames (A from %d, B from %d)", n,
		c.AStartIdx, c.BStartIdx)
	return n, nil
}

func (c *ComparatorConfig) GetColorspaces(a, b *openedVideo) (vship.Colorspace,
	vship.Colorspace, error) {
	logf(LogInfo, "Determining colorspaces for both videos")

	colorA, err := getVideoColorspace(a)
	if err != nil {
		logf(LogError, "Failed to get colorspace for video A: %v", err)
		return vship.Colorspace{}, vship.Colorspace{}, err
	}
	logf(LogDebug, "Video A colorspace: %+v", colorA)

	colorB, err := getVideoColorspace(b)
	if err != nil {
		logf(LogError, "Failed to get colorspace for video B: %v", err)
		return vship.Colorspace{}, vship.Colorspace{}, err
	}
	logf(LogDebug, "Video B colorspace: %+v", colorB)

	logf(LogInfo, "Colorspaces determined successfully")
	return colorA, colorB, nil
}

func (c *ComparatorConfig) BuildMetrics(colorA, colorB *vship.Colorspace) (
	[]MetricHandler, error) {

	logf(LogInfo, "Building %d metrics: %v", len(c.Metrics), c.Metrics)

	var metrics []MetricHandler

	for _, name := range c.Metrics {
		logf(LogInfo, "Building metric: %s", name)
		metric, err := c.BuildMetric(colorA, colorB, name)
		if err != nil {
			logf(LogError, "Failed to build metric '%s': %v", name, err)
			return nil, err
		}
		metrics = append(metrics, metric)
		logf(LogDebug, "Successfully built metric '%s'", name)
	}

	logf(LogInfo, "All %d metrics built successfully", len(metrics))
	return metrics, nil
}

func (c *ComparatorConfig) BuildMetric(colorA, colorB *vship.Colorspace,
	name string) (MetricHandler, error) {
	logf(LogDebug, "Constructing handler for metric '%s'", name)

	switch name {
	case "ssimu2":
		logf(LogInfo, "Creating SSIMU2 handler with %d workers", c.WorkerCount)
		m, err := NewSSIMU2Handler(c.WorkerCount, colorA, colorB)
		if err != nil {
			logf(LogError, "SSIMU2 handler creation failed: %v", err)
			return nil, err
		}
		return m, nil

	case "butter":
		logf(LogInfo, "Creating Butteraugli handler (QNorm=%d, Display"+
			"Brightness=%.2f)", c.ButteraugliQNorm, c.DisplayBrightness)
		m, err := NewButterHandler(c.WorkerCount, colorA, colorB,
			c.ButteraugliQNorm, float32(c.DisplayBrightness), c)
		if err != nil {
			logf(LogError, "Butteraugli handler creation failed: %v", err)
			return nil, err
		}
		return m, nil

	case "cvvdp":
		logf(LogInfo, "Creating CVVDP handler with custom display parameters")
		m, err := NewCVVDPHandler(c.WorkerCount, colorA, colorB, c)
		if err != nil {
			logf(LogError, "CVVDP handler creation failed: %v", err)
			return nil, err
		}
		return m, nil

	default:
		err := fmt.Errorf("unknown metric %s", name)
		logf(LogError, "Unknown metric requested: %s", name)
		return nil, err
	}
}
