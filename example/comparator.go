package main

import (
	"context"
	"fmt"
	"maps"
	"sync"

	vship "github.com/GreatValueCreamSoda/govship"
)

// frame represents a single video frame's data. It holds the pixel data for
// the three color planes (typically Y, U, V in YUV format) and the line sizes
// (stride) for each plane.
type frame struct {
	data     [3][]byte // Pixel data for each of the three planes.
	lineSize [3]int64  // Line size (stride) for each plane, in bytes.
}

// framePair represents a paired set of frames from video A and video B, along
// with their indices for tracking.
type framePair struct {
	// The pair's index in the sequence of comparisons (0 to numFrames-1).
	index int
	aIdx  int    // Original frame index in video A.
	bIdx  int    // Original frame index in video B.
	a     *frame // Pointer to the frame from video A.
	b     *frame // Pointer to the frame from video B.
}

// metricResult holds the computed metric scores for a specific frame pair.
// The scores are a map of metric names to their float64 values.
type metricResult struct {
	// The index of the frame pair these scores belong to.
	index  int
	scores map[string]float64 // Map of metric names to computed scores.
}

// VideoComparator is the main struct for comparing two videos frame by frame.
// It manages video opening, frame reading, pairing, metric computation, and
// result aggregation. It uses concurrency for efficiency: separate goroutines
// for reading each video, pairing frames, computing metrics in multiple
// workers, and aggregating results.
type VideoComparator struct {
	// Configuration for the comparator, including paths, metrics, etc.
	cfg ComparatorConfig

	// Opened video handles for A and B.
	videoA, videoB openedVideo

	colorA, colorB vship.Colorspace // Colorspaces of videos A and B.

	// Memory pools for reusing frame buffers for A and B to avoid allocations.
	framePoolA, framePoolB sync.Pool

	// Total number of frames to compare.
	numFrames int

	// List of metric handlers to compute on each frame pair.
	metrics []MetricHandler

	// Channels for streaming frames from A and B readers.
	framesA, framesB chan *frame

	// Channel for paired frames ready for metric computation.
	pairs chan framePair

	// Channel for computed metric results from workers.
	results chan metricResult

	// Channel for propagating errors from any goroutine.
	errs chan error

	// Aggregated final scores: metric name to slice of scores per frame.
	finalMetricScores map[string][]float64
}

// NewVideoComparator creates and initializes a new VideoComparator based on
// the provided config. It validates the config, opens videos, determines frame
// count and colorspaces, builds metrics, and sets up channels and pools.
// Returns an error if any step fails.
func NewVideoComparator(cfg ComparatorConfig) (*VideoComparator, error) {
	// Validate the configuration to ensure all required fields are set correctly.
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Open both videos using the config.
	videoA, videoB, err := cfg.OpenVideos()
	if err != nil {
		return nil, err
	}

	// Determine the number of frames to compare, typically the minimum of
	// the two videos' lengths.
	numFrames, err := cfg.FrameCount(videoA, videoB)
	if err != nil {
		return nil, err
	}

	// Get the colorspaces for both videos.
	colorA, colorB, err := cfg.GetColorspaces(&videoA, &videoB)
	if err != nil {
		return nil, err
	}

	// Build the list of metric handlers based on the config and colorspaces.
	metrics, err := cfg.BuildMetrics(&colorA, &colorB)
	if err != nil {
		return nil, err
	}

	// Initialize the comparator struct.
	vc := &VideoComparator{
		cfg:       cfg,
		videoA:    videoA,
		videoB:    videoB,
		colorA:    colorA,
		colorB:    colorB,
		numFrames: numFrames,
		metrics:   metrics,
	}

	// Set up communication channels.
	vc.initChannels()
	// Set up memory pools for frames.
	vc.initPools()

	return vc, nil
}

// FinalScores returns the aggregated metric scores after Run() has completed.
// It's a map of metric names to slices of float64 scores, one per frame.
func (vc *VideoComparator) FinalScores() map[string][]float64 {
	return vc.finalMetricScores
}

// Run executes the video comparison process. It starts goroutines for reading
// videos, pairing frames, computing metrics in workers, and aggregating
// results. It handles cancellation via context and error propagation. Returns
// nil on success, or an error if something fails or the context is canceled.
func (vc *VideoComparator) Run(ctx context.Context) error {
	// Create a cancelable child context to allow clean shutdown.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// WaitGroup for video reader goroutines.
	var readerWg sync.WaitGroup
	readerWg.Add(2)
	// Goroutine to read frames from video A.
	go func() {
		defer readerWg.Done()
		defer close(vc.framesA)
		vc.readVideo(ctx, vc.videoA, vc.cfg.AStartIdx, &vc.framePoolA,
			vc.framesA)
	}()
	// Goroutine to read frames from video B.
	go func() {
		defer readerWg.Done()
		defer close(vc.framesB)
		vc.readVideo(ctx, vc.videoB, vc.cfg.BStartIdx, &vc.framePoolB,
			vc.framesB)
	}()

	// Goroutine to pair frames from A and B.
	go func() {
		defer close(vc.pairs)
		vc.pairFrames(ctx)
	}()

	// WaitGroup for metric worker goroutines.
	var metricWg sync.WaitGroup
	metricWg.Add(vc.cfg.WorkerCount)
	// Start worker goroutines for computing metrics.
	for i := range vc.cfg.WorkerCount {
		go func() {
			defer metricWg.Done()
			vc.metricWorker(ctx, i)
		}()
	}

	// Channel to signal when metric workers are done.
	done := make(chan struct{})
	go func() {
		metricWg.Wait()
		for _, i := range vc.metrics {
			i.Close()
		}
		close(vc.results)
		close(done)
	}()

	// WaitGroup for the aggregator goroutine.
	var aggWg sync.WaitGroup
	aggWg.Add(1)
	// Goroutine to aggregate results.
	go func() {
		defer aggWg.Done()
		vc.aggregateResults(ctx)
	}()

	// Wait for completion, error, or cancellation.
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

// initChannels initializes the communication channels with appropriate
// buffers.
//
// framesA/B: for individual frames (buffer 1 to avoid blocking readers
// unnecessarily).
//
// pairs: for frame pairs (buffer 1).
//
// results: for metric results (buffer sized to WorkerCount * 1.5 for some
// headroom).
//
// errs: for errors (buffer sized to total possible sources: workers +
// readers + pairer + aggregator).
func (vc *VideoComparator) initChannels() {
	vc.framesA = make(chan *frame, 1)
	vc.framesB = make(chan *frame, 1)
	vc.pairs = make(chan framePair, 1)
	vc.results = make(chan metricResult, vc.cfg.WorkerCount*3/2)
	vc.errs = make(chan error, vc.cfg.WorkerCount+4)
}

// initPools sets up sync.Pools for reusing frame buffers. Each pool creates
// frames with pre-allocated byte slices matching the plane sizesfrom the first
// frame of each video, to avoid repeated allocations.
func (vc *VideoComparator) initPools() {
	// Determine plane sizes for video A from its first frame.
	sizesA := [3]int{
		len(vc.videoA.firstFrame.Data[0]),
		len(vc.videoA.firstFrame.Data[1]),
		len(vc.videoA.firstFrame.Data[2]),
	}
	// Determine plane sizes for video B from its first frame.
	sizesB := [3]int{
		len(vc.videoB.firstFrame.Data[0]),
		len(vc.videoB.firstFrame.Data[1]),
		len(vc.videoB.firstFrame.Data[2]),
	}

	// Pool for video A frames: creates new frames with allocated data slices.
	vc.framePoolA.New = func() any {
		return &frame{
			data: [3][]byte{
				make([]byte, sizesA[0]),
				make([]byte, sizesA[1]),
				make([]byte, sizesA[2]),
			},
		}
	}

	// Pool for video B frames: similar to A.
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

// readVideo reads frames from a video starting at startIdx and sends them to
// the out channel. It gets frames from the video, copies data into pooled
// buffers, and handles errors/cancellation
func (vc *VideoComparator) readVideo(ctx context.Context, ov openedVideo,
	startIdx int, pool *sync.Pool, out chan<- *frame) {
	logf(LogInfo, "Starting video read from index %d", startIdx)

	for i := 0; i < vc.numFrames; i++ {
		if ctx.Err() != nil {
			vc.errs <- ctx.Err()
			logf(LogError, "Video read canceled at frame %d: %v", i, ctx.Err())
			return
		}

		src, _, err := ov.video.GetFrame(startIdx + i)
		if err != nil {
			vc.errs <- err
			logf(LogError, "Error reading frame %d: %v", startIdx+i, err)
			return
		}

		buf := pool.Get().(*frame)
		for p := 0; p < 3; p++ {
			copy(buf.data[p], src.Data[p])
			buf.lineSize[p] = int64(src.Linesize[p])
		}

		select {
		case out <- buf:
			logf(LogDebug, "Read frame %d successfully", startIdx+i)
		case <-ctx.Done():
			pool.Put(buf)
			vc.errs <- ctx.Err()
			logf(LogError, "Video read context canceled at frame %d",
				startIdx+i)
			return
		}
	}
	logf(LogInfo, "Finished reading video starting at index %d", startIdx)
}

// pairFrames pairs frames from framesA and framesB channels and sends pairs to
// the pairs channel. It assumes frames arrive in order and pairs them
// sequentially.
func (vc *VideoComparator) pairFrames(ctx context.Context) {
	logf(LogInfo, "Starting frame pairing")

	for i := 0; i < vc.numFrames; i++ {
		if ctx.Err() != nil {
			vc.errs <- ctx.Err()
			logf(LogError, "Frame pairing canceled at index %d: %v", i,
				ctx.Err())
			return
		}

		// Receive frames from both channels and create a pair.
		pair := framePair{
			index: i,
			aIdx:  vc.cfg.AStartIdx + i,
			bIdx:  vc.cfg.BStartIdx + i,
			a:     <-vc.framesA,
			b:     <-vc.framesB,
		}

		select {
		case vc.pairs <- pair:
			logf(LogDebug, "Paired frame %d (A:%d, B:%d)", i, pair.aIdx,
				pair.bIdx)
		case <-ctx.Done():
			vc.errs <- ctx.Err()
			logf(LogError, "Frame pairing context canceled at index %d", i)
			return
		}
	}

	logf(LogInfo, "Finished pairing %d frames", vc.numFrames)
}

// metricWorker processes frame pairs from the pairs channel, computes metrics,
// and sends results. It runs in multiple instances (WorkerCount) for parallel
// processing. On error, sends to errs and skips sending results. Recycles
// frames back to pools after processing.
func (vc *VideoComparator) metricWorker(ctx context.Context, workerID int) {
	logf(LogInfo, "Metric worker thread %d starting", workerID)

	for pair := range withContext(ctx, vc.pairs) {
		scores := vc.computeMetrics(pair, workerID)
		if scores == nil {
			continue
		}
		// Send the result.
		vc.results <- metricResult{index: pair.index, scores: scores}
		// Return frames.
		vc.framePoolA.Put(pair.a)
		vc.framePoolB.Put(pair.b)
		logf(LogDebug, "Worker %d computed scores for frame %d: %s",
			workerID, pair.index, prettyMap(scores))
	}

	if ctx.Err() != nil {
		vc.errs <- ctx.Err()
		logf(LogError, "Worker %d exiting due to context cancellation: %v",
			workerID, ctx.Err())
	} else {
		logf(LogInfo, "Worker %d finished", workerID)
	}
}

// computeMetrics computes all configured metrics on a frame pair. Returns a
// map of metric names to scores, or nil if any metric fails (error sent to
// errs).
func (vc *VideoComparator) computeMetrics(pair framePair, workerID int,
) map[string]float64 {
	scores := make(map[string]float64)

	// Loop over each metric handler.
	for _, m := range vc.metrics {
		vals, err := m.Compute(pair.a, pair.b)
		if err != nil {
			vc.errs <- fmt.Errorf("metric %s worker %d: %w", m.Name(),
				workerID, err)
			logf(LogError, "Metric %s computation failed on worker %d, frame "+
				" %d: %v", m.Name(), workerID, pair.index, err)
			return nil
		}
		maps.Copy(scores, vals)
		logf(LogDebug, "Worker %d metric %s scores for frame %d: %s", workerID,
			m.Name(), pair.index, prettyMap(vals))
	}

	return scores
}

// aggregateResults collects results from the results channel and aggregates
// them into finalMetricScores. Initializes slices for each metric as needed
// and places scores at the correct frame index.
func (vc *VideoComparator) aggregateResults(ctx context.Context) {
	// Ensure final scores map is initialized.
	if vc.finalMetricScores == nil {
		vc.finalMetricScores = make(map[string][]float64)
	}
	logf(LogInfo, "Starting aggregation of results")

	for res := range withContext(ctx, vc.results) {
		for name, val := range res.scores {
			// Initialize slice if first time seeing this metric.
			if vc.finalMetricScores[name] == nil {
				vc.finalMetricScores[name] = make([]float64, vc.numFrames)
			}
			vc.finalMetricScores[name][res.index] = val
			logf(LogDebug, "Aggregated result for metric %s frame %d: %f",
				name, res.index, val)
		}
	}

	logf(LogInfo, "Finished aggregating results")
}
