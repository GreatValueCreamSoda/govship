package main

import (
	"runtime"
	"sync"

	ffms "github.com/GreatValueCreamSoda/goffms2"
)

type openedVideo struct {
	video      *ffms.VideoSource
	props      *ffms.VideoProperties
	firstFrame *ffms.Frame
	err        error
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
		runtime.NumCPU()/2, ffms.SeekNormal)
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
