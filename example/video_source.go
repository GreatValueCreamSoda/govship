package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	ffms "github.com/GreatValueCreamSoda/goffms2"
	"github.com/GreatValueCreamSoda/gopixfmts"
	vship "github.com/GreatValueCreamSoda/govship"
)

var numThreads = runtime.NumCPU()

type logLevel int

const (
	logError logLevel = iota
	logInfo
	logDebug
)

var currentLogLevel = logInfo

func logf(level logLevel, format string, args ...any) {
	if level <= currentLogLevel {
		prefix := "[INFO]"
		if level == logDebug {
			prefix = "[DEBUG]"
		} else if level == logError {
			prefix = "[ERROR]"
		}
		log.Printf(prefix+" "+format, args...)
	}
}

type videoComparisonConfig struct {
	videoAPath, videoBPath                   string
	videoAStartingIndex, videoBStartingIndex int
	numFramesToCompare                       int
	numGpuThreads                            int
}

func parseConfig() *videoComparisonConfig {
	cfg := &videoComparisonConfig{}

	flag.StringVar(&cfg.videoAPath, "a", "", "path to source video A")
	flag.StringVar(&cfg.videoBPath, "b", "", "path to source video B")
	flag.IntVar(&cfg.videoAStartingIndex, "a-start", 0, "starting frame index for video A")
	flag.IntVar(&cfg.videoBStartingIndex, "b-start", 0, "starting frame index for video B")
	flag.IntVar(&cfg.numFramesToCompare, "frames", 0, "number of frames to compare (0 = min length)")
	flag.IntVar(&cfg.numGpuThreads, "workers", 3, "number of metric workers")
	verbosity := flag.String("log", "info", "log level: error, info, debug")

	flag.Parse()

	switch *verbosity {
	case "debug":
		currentLogLevel = logDebug
	case "error":
		currentLogLevel = logError
	default:
		currentLogLevel = logInfo
	}

	if cfg.videoAPath == "" || cfg.videoBPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	return cfg
}

// ---------- DATA TYPES ----------

type MetricScores map[string]float64

type metricResult struct {
	compareIdx int
	scores     MetricScores
}

type MetricStats struct {
	Min float64
	Max float64
	Avg float64
}

type frame struct {
	data     [3][]byte
	lineSize [3]int64
}

type framePair struct {
	compareIdx int
	aIdx       int
	bIdx       int
	a          *frame
	b          *frame
}

type openedVideo struct {
	video      *ffms.VideoSource
	props      *ffms.VideoProperties
	firstFrame *ffms.Frame
	err        error
}

type VideoComparator struct {
	videoA, videoB *ffms.VideoSource

	videoAStart int
	videoBStart int
	numFrames   int

	videoAColorspace vship.Colorspace
	videoBColorspace vship.Colorspace

	framePoolA sync.Pool
	framePoolB sync.Pool

	framesA chan *frame
	framesB chan *frame
	pairs   chan framePair

	workerCount int

	results chan metricResult

	finalScores map[string][]float64
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	cfg := parseConfig()

	start := time.Now()
	if err := CompareTwoVideos(cfg); err != nil {
		logf(logError, "fatal error: %v", err)
		os.Exit(1)
	}
	logf(logInfo, "completed in %s", time.Since(start))
}

func CompareTwoVideos(config *videoComparisonConfig) error {
	vc, err := NewVideoComparator(config)
	if err != nil {
		return err
	}

	if err := vc.Run(); err != nil {
		return err
	}

	printSummary(vc.finalScores)
	return nil
}

func NewVideoComparator(config *videoComparisonConfig) (*VideoComparator, error) {
	a, b, err := openVideoAAndB(config.videoAPath, config.videoBPath)
	if err != nil {
		return nil, err
	}

	maxA := a.props.NumFrames - config.videoAStartingIndex
	maxB := b.props.NumFrames - config.videoBStartingIndex

	numFrames := maxA
	if maxB < numFrames {
		numFrames = maxB
	}
	if config.numFramesToCompare > 0 && config.numFramesToCompare < numFrames {
		numFrames = config.numFramesToCompare
	}

	metricNames := []string{
		"ssimu2",
	}

	finalScores := make(map[string][]float64, len(metricNames))
	for _, name := range metricNames {
		finalScores[name] = make([]float64, numFrames)
	}

	logf(logInfo, "comparing %d frames (A start=%d, B start=%d)",
		numFrames, config.videoAStartingIndex, config.videoBStartingIndex)

	videoAColorspace, err := getVideoColorspace(&a)
	if err != nil {
		return nil, err
	}

	videoBColorspace, err := getVideoColorspace(&b)
	if err != nil {
		return nil, err
	}

	vc := &VideoComparator{
		videoA:           a.video,
		videoB:           b.video,
		videoAStart:      config.videoAStartingIndex,
		videoBStart:      config.videoBStartingIndex,
		numFrames:        numFrames,
		videoAColorspace: videoAColorspace,
		videoBColorspace: videoBColorspace,
		framesA:          make(chan *frame, 2),
		framesB:          make(chan *frame, 2),
		pairs:            make(chan framePair, 2),
		workerCount:      config.numGpuThreads,
		results:          make(chan metricResult, config.numGpuThreads),
		finalScores:      finalScores,
	}

	initFramePools(vc, &a, &b)
	return vc, nil
}

func initFramePools(vc *VideoComparator, a, b *openedVideo) {
	sizesA := [3]int{
		len(a.firstFrame.Data[0]),
		len(a.firstFrame.Data[1]),
		len(a.firstFrame.Data[2]),
	}
	sizesB := [3]int{
		len(b.firstFrame.Data[0]),
		len(b.firstFrame.Data[1]),
		len(b.firstFrame.Data[2]),
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

func (vc *VideoComparator) Run() error {
	var wg sync.WaitGroup
	wg.Add(vc.workerCount)

	go vc.readFrames(vc.videoA, vc.videoAStart, &vc.framePoolA, vc.framesA, "A")
	go vc.readFrames(vc.videoB, vc.videoBStart, &vc.framePoolB, vc.framesB, "B")
	go vc.pairFrames()

	go vc.aggregateResults()

	for i := 0; i < vc.workerCount; i++ {
		go vc.metricWorker(&wg, i)
	}

	wg.Wait()
	close(vc.results)

	return nil
}

func (vc *VideoComparator) readFrames(
	src *ffms.VideoSource,
	startIdx int,
	pool *sync.Pool,
	out chan<- *frame,
	label string,
) {
	defer close(out)

	for i := 0; i < vc.numFrames; i++ {
		frameIdx := startIdx + i

		srcFrame, _, err := src.GetFrame(frameIdx)
		if err != nil {
			panic(err)
		}

		buf := pool.Get().(*frame)
		for p := 0; p < 3; p++ {
			copy(buf.data[p], srcFrame.Data[p])
			buf.lineSize[p] = int64(srcFrame.Linesize[p])
		}

		logf(logDebug, "%s read frame %d (compare %d)", label, frameIdx, i)
		out <- buf
	}
}

func (vc *VideoComparator) pairFrames() {
	defer close(vc.pairs)

	for i := 0; i < vc.numFrames; i++ {
		a, okA := <-vc.framesA
		b, okB := <-vc.framesB
		if !okA || !okB {
			return
		}

		vc.pairs <- framePair{
			compareIdx: i,
			aIdx:       vc.videoAStart + i,
			bIdx:       vc.videoBStart + i,
			a:          a,
			b:          b,
		}
	}
}

func (vc *VideoComparator) metricWorker(wg *sync.WaitGroup, workerID int) {
	defer wg.Done()

	handler, exception := vship.NewSSIMU2Handler(
		&vc.videoAColorspace,
		&vc.videoBColorspace,
	)
	if !exception.IsNone() {
		panic(exception)
	}

	for pair := range vc.pairs {
		score, exception := handler.ComputeScore(
			pair.a.data, pair.b.data,
			pair.a.lineSize, pair.b.lineSize,
		)
		if !exception.IsNone() {
			panic(exception)
		}

		vc.results <- metricResult{
			compareIdx: pair.compareIdx,
			scores: MetricScores{
				"ssimu2": float64(score),
			},
		}

		vc.framePoolA.Put(pair.a)
		vc.framePoolB.Put(pair.b)
	}
}

func (vc *VideoComparator) aggregateResults() {
	for result := range vc.results {
		for name, value := range result.scores {
			list, ok := vc.finalScores[name]
			if !ok {
				panic("received score for unknown metric: " + name)
			}
			list[result.compareIdx] = value
		}
	}
}

func openVideoAAndB(pathA, pathB string) (openedVideo, openedVideo, error) {
	var wg sync.WaitGroup
	wg.Add(2)

	var a, b openedVideo

	go func() {
		defer wg.Done()
		a = openVideo(pathA)
	}()

	go func() {
		defer wg.Done()
		b = openVideo(pathB)
	}()

	wg.Wait()

	if a.err != nil {
		return openedVideo{}, openedVideo{}, a.err
	}
	if b.err != nil {
		return openedVideo{}, openedVideo{}, b.err
	}

	return a, b, nil
}

func openVideo(path string) openedVideo {
	indexer, _, err := ffms.CreateIndexer(path)
	if err != nil {
		return openedVideo{err: err}
	}

	index, _, err := indexer.DoIndexing(ffms.IEHAbort)
	if err != nil {
		return openedVideo{err: err}
	}

	track, _, err := index.GetFirstTrackOfType(ffms.TypeVideo)
	if err != nil {
		return openedVideo{err: err}
	}

	video, _, err := ffms.CreateVideoSource(path, index, track,
		numThreads, ffms.SeekNormal)
	if err != nil {
		return openedVideo{err: err}
	}

	props, err := video.GetVideoProperties()
	if err != nil {
		return openedVideo{err: err}
	}

	firstFrame, _, err := video.GetFrame(0)
	if err != nil {
		return openedVideo{err: err}
	}

	video.SetOutputFormatV2([]int{firstFrame.EncodedPixelFormat},
		firstFrame.EncodedWidth, firstFrame.EncodedHeight,
		ffms.ResizerBicubic)

	firstFrame, _, err = video.GetFrame(0)
	if err != nil {
		return openedVideo{err: err}
	}

	return openedVideo{
		video: video, props: &props, firstFrame: &firstFrame}
}

func getVideoColorspace(video *openedVideo) (vship.Colorspace, error) {
	var colorspace vship.Colorspace

	colorspace.Width = int64(video.firstFrame.ScaledWidth)
	colorspace.TargetWidth = colorspace.Width
	colorspace.Height = int64(video.firstFrame.ScaledHeight)
	colorspace.TargetHeight = colorspace.Height

	videoPixelFormat, err := gopixfmts.PixFmtDescGet(
		gopixfmts.PixelFormat(video.firstFrame.ConvertedPixelFormat))

	comp, err := videoPixelFormat.Component(0)
	if err != nil {
		return colorspace, err
	}

	var videoDepth vship.SamplingFormat

	switch comp.Depth {
	case 8:
		videoDepth = vship.SamplingFormatUInt8
	case 9:
		videoDepth = vship.SamplingFormatUInt9
	case 10:
		videoDepth = vship.SamplingFormatUInt10
	case 12:
		videoDepth = vship.SamplingFormatUInt12
	case 14:
		videoDepth = vship.SamplingFormatUInt14
	case 16:
		videoDepth = vship.SamplingFormatUInt16
	default:
		panic("UNKNOWN PIXEL FORMAT")
	}

	colorspace.SamplingFormat = videoDepth

	colorspace.ColorRange = vship.ColorRange(video.firstFrame.ColorRange)
	colorspace.ChromaSubsamplingHeight = videoPixelFormat.Log2ChromaH()
	colorspace.ChromaSubsamplingWidth = videoPixelFormat.Log2ChromaW()
	colorspace.ChromaLocation = vship.ChromaLocation(video.firstFrame.ChromaLocation)
	colorspace.ColorFamily = vship.ColorFamilyYUV
	colorspace.ColorMatrix = vship.ColorMatrix(video.firstFrame.ColorSpace)
	colorspace.ColorTransfer = vship.ColorTransfer(video.firstFrame.TransferCharateristics)
	colorspace.ColorPrimaries = vship.ColorPrimaries(video.firstFrame.ColorPrimaries)

	return colorspace, nil
}

func computeStats(values []float64) MetricStats {
	if len(values) == 0 {
		return MetricStats{}
	}

	min := values[0]
	max := values[0]
	var sum int64

	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += int64(v)
	}

	return MetricStats{
		Min: min,
		Max: max,
		Avg: float64(sum) / float64(len(values)),
	}
}
