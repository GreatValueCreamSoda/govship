package main

import (
	"context"
	"fmt"
	"maps"
	"sync"

	vship "github.com/GreatValueCreamSoda/govship"
)

type frame struct {
	data     [3][]byte
	lineSize [3]int64
}

type framePair struct {
	index int
	aIdx  int
	bIdx  int
	a     *frame
	b     *frame
}

type metricResult struct {
	index  int
	scores map[string]float64
}

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
}

func (c *ComparatorConfig) Validate() error {
	if c.WorkerCount <= 0 {
		c.WorkerCount = 1
	}
	if len(c.Metrics) == 0 {
		return fmt.Errorf("at least one metric must be specified")
	}
	return nil
}

func (c *ComparatorConfig) OpenVideos() (openedVideo, openedVideo, error) {
	return openVideoAAndB(c.VideoAPath, c.VideoBPath)
}

func (c *ComparatorConfig) FrameCount(a, b openedVideo) (int, error) {
	maxA := a.props.NumFrames - c.AStartIdx
	maxB := b.props.NumFrames - c.BStartIdx

	n := maxA
	if maxB < n {
		n = maxB
	}
	if c.MaxFrames > 0 && c.MaxFrames < n {
		n = c.MaxFrames
	}
	if n <= 0 {
		return 0, fmt.Errorf("no frames to compare")
	}

	return n, nil
}

func (c *ComparatorConfig) GetColorspaces(a, b *openedVideo) (vship.Colorspace,
	vship.Colorspace, error) {
	colorA, err := getVideoColorspace(a)
	if err != nil {
		return vship.Colorspace{}, vship.Colorspace{}, err
	}
	colorB, err := getVideoColorspace(b)
	if err != nil {
		return vship.Colorspace{}, vship.Colorspace{}, err
	}

	return colorA, colorB, nil
}

func (c *ComparatorConfig) BuildMetrics(colorA, colorB *vship.Colorspace) (
	[]MetricHandler, error) {

	var metrics []MetricHandler

	for _, name := range c.Metrics {
		metric, err := c.BuildMetric(colorA, colorB, name)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, metric)

	}
	return metrics, nil
}

func (c *ComparatorConfig) BuildMetric(colorA, colorB *vship.Colorspace,
	name string) (MetricHandler, error) {
	switch name {
	case "ssimu2":
		m, err := NewSSIMU2Handler(c.WorkerCount, colorA, colorB)
		if err != nil {
			return nil, err
		}
		return m, nil

	case "butter":
		m, err := NewButterHandler(c.WorkerCount, colorA, colorB,
			c.ButteraugliQNorm, float32(c.DisplayBrightness))
		if err != nil {
			return nil, err
		}
		return m, nil

	case "cvvdp":
		m, err := NewCVVDPHandler(c.WorkerCount, colorA, colorB, c)
		if err != nil {
			return nil, err
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unknown metric %s", name)
	}
}

type VideoComparator struct {
	cfg                    ComparatorConfig
	videoA, videoB         openedVideo
	colorA, colorB         vship.Colorspace
	framePoolA, framePoolB sync.Pool
	numFrames              int
	metrics                []MetricHandler
	framesA, framesB       chan *frame
	pairs                  chan framePair
	results                chan metricResult
	errs                   chan error

	finalMetricScores map[string][]float64
}

func NewVideoComparator(cfg ComparatorConfig) (*VideoComparator, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	videoA, videoB, err := cfg.OpenVideos()
	if err != nil {
		return nil, err
	}

	numFrames, err := cfg.FrameCount(videoA, videoB)
	if err != nil {
		return nil, err
	}

	colorA, colorB, err := cfg.GetColorspaces(&videoA, &videoB)
	if err != nil {
		return nil, err
	}

	metrics, err := cfg.BuildMetrics(&colorA, &colorB)
	if err != nil {
		return nil, err
	}

	vc := &VideoComparator{
		cfg:       cfg,
		videoA:    videoA,
		videoB:    videoB,
		colorA:    colorA,
		colorB:    colorB,
		numFrames: numFrames,
		metrics:   metrics,
	}

	vc.initChannels()
	vc.initPools()

	return vc, nil
}

func (vc *VideoComparator) FinalScores() map[string][]float64 {
	return vc.finalMetricScores
}

func (vc *VideoComparator) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(vc.cfg.WorkerCount)

	go vc.readVideo(ctx, vc.videoA, vc.cfg.AStartIdx, &vc.framePoolA, vc.framesA)
	go vc.readVideo(ctx, vc.videoB, vc.cfg.BStartIdx, &vc.framePoolB, vc.framesB)

	go vc.pairFrames(ctx)

	for i := 0; i < vc.cfg.WorkerCount; i++ {
		go vc.metricWorker(ctx, i, &wg)
	}

	var aggWg sync.WaitGroup
	aggWg.Add(1)
	go func() {
		defer aggWg.Done()
		vc.aggregateResults(ctx)
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(vc.results)
		close(done)
	}()

	select {
	case err := <-vc.errs:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		aggWg.Wait()
		return nil
	}
}

func (vc *VideoComparator) initChannels() {
	vc.framesA = make(chan *frame, 1)
	vc.framesB = make(chan *frame, 1)
	vc.pairs = make(chan framePair, 1)
	vc.results = make(chan metricResult, vc.cfg.WorkerCount*3/2)
	vc.errs = make(chan error, vc.cfg.WorkerCount+4)
}

func (vc *VideoComparator) initPools() {
	sizesA := [3]int{
		len(vc.videoA.firstFrame.Data[0]),
		len(vc.videoA.firstFrame.Data[1]),
		len(vc.videoA.firstFrame.Data[2]),
	}
	sizesB := [3]int{
		len(vc.videoB.firstFrame.Data[0]),
		len(vc.videoB.firstFrame.Data[1]),
		len(vc.videoB.firstFrame.Data[2]),
	}

	vc.framePoolA.New = func() any {
		return &frame{
			data: [3][]byte{
				make([]byte, sizesA[0]),
				make([]byte, sizesA[1]),
				make([]byte, sizesA[2]),
			},
		}
	}

	vc.framePoolB.New = func() any {
		return &frame{
			data: [3][]byte{
				make([]byte, sizesB[0]),
				make([]byte, sizesB[1]),
				make([]byte, sizesB[2]),
			},
		}
	}
}

func (vc *VideoComparator) readVideo(ctx context.Context, ov openedVideo,
	startIdx int, pool *sync.Pool, out chan<- *frame) {
	defer close(out)
	logf(logInfo, "Starting video read from index %d", startIdx)

	for i := 0; i < vc.numFrames; i++ {
		if ctx.Err() != nil {
			vc.errs <- ctx.Err()
			logf(logError, "Video read canceled at frame %d: %v", i, ctx.Err())
			return
		}

		src, _, err := ov.video.GetFrame(startIdx + i)
		if err != nil {
			vc.errs <- err
			logf(logError, "Error reading frame %d: %v", startIdx+i, err)
			return
		}

		buf := pool.Get().(*frame)
		for p := 0; p < 3; p++ {
			copy(buf.data[p], src.Data[p])
			buf.lineSize[p] = int64(src.Linesize[p])
		}

		select {
		case out <- buf:
			logf(logDebug, "Read frame %d successfully", startIdx+i)
		case <-ctx.Done():
			pool.Put(buf)
			vc.errs <- ctx.Err()
			logf(logError, "Video read context canceled at frame %d",
				startIdx+i)
			return
		}
	}
	logf(logInfo, "Finished reading video starting at index %d", startIdx)
}

func (vc *VideoComparator) pairFrames(ctx context.Context) {
	defer close(vc.pairs)
	logf(logInfo, "Starting frame pairing")

	for i := 0; i < vc.numFrames; i++ {
		if ctx.Err() != nil {
			vc.errs <- ctx.Err()
			logf(logError, "Frame pairing canceled at index %d: %v", i,
				ctx.Err())
			return
		}

		pair := framePair{
			index: i,
			aIdx:  vc.cfg.AStartIdx + i,
			bIdx:  vc.cfg.BStartIdx + i,
			a:     <-vc.framesA,
			b:     <-vc.framesB,
		}

		select {
		case vc.pairs <- pair:
			logf(logDebug, "Paired frame %d (A:%d, B:%d)", i, pair.aIdx,
				pair.bIdx)
		case <-ctx.Done():
			vc.errs <- ctx.Err()
			logf(logError, "Frame pairing context canceled at index %d", i)
			return
		}
	}

	logf(logInfo, "Finished pairing %d frames", vc.numFrames)
}

func (vc *VideoComparator) metricWorker(ctx context.Context, workerID int,
	wg *sync.WaitGroup) {
	defer wg.Done()
	logf(logInfo, "Metric worker thread %d starting", workerID)

	for pair := range withContext(ctx, vc.pairs) {
		scores := vc.computeMetrics(pair, workerID)
		if scores == nil {
			continue
		}
		vc.results <- metricResult{index: pair.index, scores: scores}
		vc.framePoolA.Put(pair.a)
		vc.framePoolB.Put(pair.b)
		logf(logDebug, "Worker %d computed scores for frame %d: %s",
			workerID, pair.index, prettyMap(scores))
	}

	if ctx.Err() != nil {
		vc.errs <- ctx.Err()
		logf(logError, "Worker %d exiting due to context cancellation: %v",
			workerID, ctx.Err())
	} else {
		logf(logInfo, "Worker %d finished", workerID)
	}
}

func (vc *VideoComparator) computeMetrics(pair framePair, workerID int,
) map[string]float64 {
	scores := make(map[string]float64)

	for _, m := range vc.metrics {
		vals, err := m.Compute(pair.a, pair.b)
		if err != nil {
			vc.errs <- fmt.Errorf("metric %s worker %d: %w", m.Name(),
				workerID, err)
			logf(logError, "Metric %s computation failed on worker %d, frame "+
				" %d: %v", m.Name(), workerID, pair.index, err)
			return nil
		}
		maps.Copy(scores, vals)
		logf(logDebug, "Worker %d metric %s scores for frame %d: %s", workerID,
			m.Name(), pair.index, prettyMap(vals))
	}

	return scores
}

func (vc *VideoComparator) aggregateResults(ctx context.Context) {
	if vc.finalMetricScores == nil {
		vc.finalMetricScores = make(map[string][]float64)
	}
	logf(logInfo, "Starting aggregation of results")

	for res := range withContext(ctx, vc.results) {
		for name, val := range res.scores {
			if vc.finalMetricScores[name] == nil {
				vc.finalMetricScores[name] = make([]float64, vc.numFrames)
			}
			vc.finalMetricScores[name][res.index] = val
			logf(logDebug, "Aggregated result for metric %s frame %d: %f",
				name, res.index, val)
		}
	}

	logf(logInfo, "Finished aggregating results")
}
