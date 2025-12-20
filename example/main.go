package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

type LoggingLevel int

const (
	LogError LoggingLevel = iota
	LogInfo
	LogDebug
)

var currentLogLevel = LogInfo

const logPrefixWidth = 9 // Fits "[DEBUG] "

func logf(level LoggingLevel, format string, args ...any) {
	if level > currentLogLevel {
		return
	}

	prefix := "[INFO] "
	switch level {
	case LogDebug:
		prefix = "[DEBUG]"
	case LogError:
		prefix = "[ERROR]"
	}

	padded := fmt.Sprintf("%-*s", logPrefixWidth, prefix)

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	log.Printf("%s%s", padded, msg)
}

func parseLogLevel(s string) (LoggingLevel, error) {
	switch strings.ToLower(s) {
	case "error":
		return LogError, nil
	case "info":
		return LogInfo, nil
	case "debug":
		return LogDebug, nil
	default:
		return 0, fmt.Errorf("invalid log level: %q", s)
	}
}

// initCLI parses all flags and returns the config + output path
func initCLI() (ComparatorConfig, string) {
	var cfg ComparatorConfig
	var metrics string
	var logLevelStr string
	var outputPath string

	flag.StringVar(
		&cfg.VideoAPath, "a", "", "path to source video A (required)")

	flag.StringVar(
		&cfg.VideoBPath, "b", "", "path to source video B (required)")

	flag.IntVar(
		&cfg.AStartIdx, "aidx", 0, "starting frame index for video A")

	flag.IntVar(
		&cfg.BStartIdx, "bidx", 0, "starting frame index for video B")

	flag.IntVar(
		&cfg.MaxFrames, "frames", 0, "maximum number of frames to "+
			"compare (0 = all)")

	flag.IntVar(
		&cfg.WorkerCount, "workers", 3,
		"number of GPU workers (forced to 1 for CVVDP temporal)")

	flag.StringVar(
		&metrics, "metrics", "ssimu2", "comma-separated list of metrics")

	flag.StringVar(
		&logLevelStr, "loglevel", "info", "log level: error, info, debug")

	flag.IntVar(
		&cfg.ButteraugliQNorm, "butter-qnorm", 5,
		"Butteraugli quantization normalization")

	flag.Float64Var(
		&cfg.DisplayBrightness, "display-nits", 203,
		"display peak brightness in nits")

	flag.BoolVar(
		&cfg.CVVDPUseTemporalScore, "cvvdp-disable-temporal", false,
		"disable temporal weighting for CVVDP")

	flag.BoolVar(
		&cfg.CVVDPResizeToDisplay, "cvvdp-disable-resize", false,
		"disable resizing to display resolution for CVVDP")

	flag.IntVar(
		&cfg.DisplayWidth, "display-width", 3840,
		"display horizontal resolution (CVVDP)")

	flag.IntVar(
		&cfg.DisplayHeight, "display-height", 2160,
		"display vertical resolution (CVVDP)")

	flag.Float64Var(
		&cfg.DisplayDiagonal, "display-diagonal", 32,
		"display diagonal size in inches (CVVDP)")

	flag.Float64Var(
		&cfg.ViewingDistance, "viewing-distance", 0.7472,
		"viewing distance in meters (CVVDP)")

	flag.IntVar(
		&cfg.MonitorContrastRatio, "display-ratio", 10000,
		"display contrast ratio (CVVDP)")

	flag.IntVar(
		&cfg.RoomBrightness, "room-lux", 100,
		"ambient room brightness in lux (CVVDP)")

	flag.StringVar(
		&outputPath, "output", "", "path to save per-frame JSON results")

	flag.StringVar(&outputPath, "o", "", "alias for -output")

	flag.BoolVar(
		&cfg.outputDistortionMapToStdout, "distmap", false,
		"output the given metrics distortion map to stdout")

	flag.Parse()

	if cfg.outputDistortionMapToStdout {
		cfg.WorkerCount = 1
	}

	cfg.CVVDPUseTemporalScore = !cfg.CVVDPUseTemporalScore
	cfg.CVVDPResizeToDisplay = !cfg.CVVDPResizeToDisplay

	if cfg.VideoAPath == "" || cfg.VideoBPath == "" {
		fmt.Fprintln(os.Stderr, "Error: both -a and -b are required")
		flag.Usage()
		os.Exit(1)
	}

	level, err := parseLogLevel(logLevelStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	currentLogLevel = level

	if metrics == "" {
		fmt.Fprintln(os.Stderr, "Error: at least one metric must be "+
			"specified via -metrics")
		os.Exit(1)
	}

	cfg.Metrics = strings.Split(metrics, ",")
	for i, m := range cfg.Metrics {
		cfg.Metrics[i] = strings.TrimSpace(m)
		if strings.ToLower(m) == "cvvdp" && cfg.CVVDPUseTemporalScore {
			cfg.WorkerCount = 1
		}
	}

	if outputPath != "" &&
		strings.HasSuffix(outputPath, string(os.PathSeparator)) {
		fmt.Fprintln(os.Stderr, "Error: -output cannot be a directory")
		os.Exit(1)
	}

	return cfg, outputPath
}

func main() {
	log.SetFlags(log.LstdFlags)

	cfg, outputPath := initCLI()

	vc, err := NewVideoComparator(cfg)
	if err != nil {
		logf(LogError, "Failed to create comparator: %v", err)
		os.Exit(1)
	}

	logf(LogInfo, "Comparing %d frames (A start: %d, B start: %d) with %d"+
		" workers", vc.numFrames, cfg.AStartIdx, cfg.BStartIdx,
		cfg.WorkerCount)

	if err := vc.Run(context.Background()); err != nil {
		logf(LogError, "Comparison failed: %v", err)
		os.Exit(1)
	}

	scores := vc.FinalScores()
	printSummary(scores)

	if outputPath != "" {
		if err := saveScoresToJSON(scores, outputPath); err != nil {
			logf(LogError, "Failed to save results to %s: %v", outputPath, err)
			os.Exit(1)
		}
		logf(LogInfo, "Per-frame scores saved to %s", outputPath)
	}
}

func saveScoresToJSON(scores map[string][]float64, path string) error {
	data, err := json.MarshalIndent(scores, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
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

	var sb strings.Builder
	sb.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%v=%v", k, m[k])
	}
	sb.WriteString("}")
	return sb.String()
}
