package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

type loggingLevel int

const (
	logError loggingLevel = iota
	logInfo
	logDebug
)

var currentLogLevel = logInfo

func logf(level loggingLevel, format string, args ...any) {
	if level > currentLogLevel {
		return
	}

	prefix := "[INFO]"
	switch level {
	case logDebug:
		prefix = "[DEBUG]"
	case logError:
		prefix = "[ERROR]"
	}

	log.Printf(prefix+" "+format, args...)
}

func parseLogLevel(s string) (loggingLevel, error) {
	switch strings.ToLower(s) {
	case "error":
		return logError, nil
	case "info":
		return logInfo, nil
	case "debug":
		return logDebug, nil
	default:
		return 0, fmt.Errorf("invalid log level: %s", s)
	}
}

func initCLI() ComparatorConfig {
	var cfg ComparatorConfig
	var metricsFlag string
	var logLevelStr string

	flag.StringVar(&cfg.VideoAPath, "a", "", "path to source video A (required)")
	flag.StringVar(&cfg.VideoBPath, "b", "", "path to source video B (required)")
	flag.IntVar(&cfg.AStartIdx, "aidx", 0, "starting frame index for video A")
	flag.IntVar(&cfg.BStartIdx, "bidx", 0, "starting frame index for video B")
	flag.IntVar(&cfg.MaxFrames, "frames", 0, "maximum number of frames to compare (0 = as many as possible)")
	flag.IntVar(&cfg.WorkerCount, "workers", 3, "number of GPU metric workers. Forced to 1 when using CVVDP with temporal weighting.")
	flag.StringVar(&metricsFlag, "metrics", "ssimu2", "comma-separated list of metrics")
	flag.StringVar(&logLevelStr, "loglevel", "info", "log level: error, info, debug")
	flag.IntVar(&cfg.ButteraugliQNorm, "butter-qnorm", 5, "Butteraugli quantization normalization value")
	flag.Float64Var(&cfg.DisplayBrightness, "display-nits", 203, "display peak brightness in nits (for CVVDP and Butteraugli Only)")
	flag.BoolVar(&cfg.CVVDPUseTemporalScore, "cvvdp-disable-temporal", false, "use temporal score (for CVVDP)")
	flag.BoolVar(&cfg.CVVDPResizeToDisplay, "cvvdp-disable-resize", false, "resize video to display resolution (for CVVDP Only)")
	flag.IntVar(&cfg.DisplayWidth, "display-width", 3840, "specify the displays horizontal resolution (for CVVDP Only)")
	flag.IntVar(&cfg.DisplayHeight, "display-height", 2160, "specify the displays horizontal resolution (for CVVDP Only)")
	flag.Float64Var(&cfg.DisplayDiagonal, "display-diagonal", 32, "specify the displays size from the top left to bottom right (for CVVDP Only)")
	flag.Float64Var(&cfg.ViewingDistance, "viewing-distance", 0.7472, "specify the distance the viewer is from the display in meters (for CVVDP Only)")
	flag.IntVar(&cfg.MonitorContrastRatio, "display-ratio", 10000, "specify the displays contrast ratio (for CVVDP Only)")
	flag.IntVar(&cfg.RoomBrightness, "room-lux", 100, "specify the rooms ambient light level in lux (for CVVDP Only)")

	flag.Parse()

	cfg.CVVDPUseTemporalScore = !cfg.CVVDPUseTemporalScore
	cfg.CVVDPResizeToDisplay = !cfg.CVVDPResizeToDisplay

	if cfg.VideoAPath == "" || cfg.VideoBPath == "" {
		fmt.Println("Error: both -a and -b are required")
		flag.Usage()
		os.Exit(1)
	}

	level, err := parseLogLevel(logLevelStr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	currentLogLevel = level

	if metricsFlag == "" {
		fmt.Println("Error: at least one metric must be specified")
		os.Exit(1)
	}

	cfg.Metrics = strings.Split(metricsFlag, ",")
	for i, m := range cfg.Metrics {
		if cfg.CVVDPUseTemporalScore && m == "cvvdp" {
			cfg.WorkerCount = 1
		}

		cfg.Metrics[i] = strings.TrimSpace(m)
	}

	return cfg
}

func main() {
	log.SetFlags(log.LstdFlags)

	cfg := initCLI()

	vc, err := NewVideoComparator(cfg)
	if err != nil {
		logf(logError, "Failed to create comparator: %v", err)
		os.Exit(1)
	}

	logf(
		logInfo,
		"Comparing %d frames starting at A:%d B:%d using %d workers",
		vc.numFrames,
		cfg.AStartIdx,
		cfg.BStartIdx,
		cfg.WorkerCount,
	)

	if err := vc.Run(context.Background()); err != nil {
		logf(logError, "Comparison failed: %v", err)
		os.Exit(1)
	}

	printSummary(vc.FinalScores(), &cfg)
}

func prettyMap[K comparable, V any](m map[K]V) string {
	if len(m) == 0 {
		return "{}"
	}

	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j])
	})

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%v=%v", k, m[k])
	}

	return b.String()
}
