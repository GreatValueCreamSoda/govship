package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/spf13/pflag"
)

type LoggingLevel int

const (
	LogError LoggingLevel = iota
	LogInfo
	LogDebug
)

var currentLogLevel = LogInfo

const logPrefixWidth = 9 // Fits "[DEBUG] "

func initCLI() (ComparatorConfig, string) {
	var cfg ComparatorConfig
	var metrics string
	var logLevelStr string
	var outputPath string
	var encoderSettings string
	var printHelpMessage bool

	// This is NOT the correct usage of Pflag. I simply do not care right now.

	RequiredFlags := pflag.NewFlagSet("Required", pflag.ContinueOnError)
	RequiredFlags.ParseErrorsAllowlist.UnknownFlags = true
	RequiredFlags.SortFlags = false
	RequiredFlags.SetOutput(io.Discard)
	RequiredFlags.StringVarP(&cfg.VideoAPath, "a", "a", "",
		"Path to source/reference video. Video B will be compared to this")
	RequiredFlags.StringVarP(&cfg.VideoBPath, "b", "b", "",
		"Path to distorted/encoded video. This will be compared to video A")
	RequiredFlags.Parse(os.Args[1:])

	GeneralFlags := pflag.NewFlagSet("General", pflag.ContinueOnError)
	GeneralFlags.ParseErrorsAllowlist.UnknownFlags = true
	GeneralFlags.SortFlags = false
	GeneralFlags.SetOutput(io.Discard)
	GeneralFlags.BoolVar(&printHelpMessage, "help", false,
		"prints this help message")
	GeneralFlags.IntVar(&cfg.AStartIdx, "aidx", 0,
		"Starting frame index for video A")
	GeneralFlags.IntVar(&cfg.BStartIdx, "bidx", 0,
		"Starting frame index for video B")
	GeneralFlags.IntVar(&cfg.MaxFrames, "frames", 0,
		"Maximum number of frames to compare (0 = all frames)")
	GeneralFlags.IntVar(&cfg.WorkerCount, "workers", 3,
		"Number of parallel GPU workers")
	GeneralFlags.StringVar(&metrics, "metrics", "ssimu2",
		"Comma-separated list of metrics [ssimu2, butteraugli, cvvdp]")
	GeneralFlags.StringVar(&logLevelStr, "loglevel", "info",
		"Log level: error, info, debug")
	GeneralFlags.Float64Var(&cfg.DisplayBrightness, "display-nits", 203,
		"Display peak brightness in nits. Used for CVVDP and Butteraugli")
	GeneralFlags.Parse(os.Args[1:])

	ButterFlags := pflag.NewFlagSet("Butteraugli", pflag.ContinueOnError)
	ButterFlags.ParseErrorsAllowlist.UnknownFlags = true
	ButterFlags.SortFlags = false
	ButterFlags.SetOutput(io.Discard)
	ButterFlags.IntVar(&cfg.ButteraugliQNorm, "butter-qnorm", 5,
		"Butteraugli quantization normalization factor")
	ButterFlags.Float64Var(&cfg.ButteraugliMaxDistortionClipping,
		"butteraugli-clipping", 15,
		"Save Butterauglis distortion map as a video")
	ButterFlags.StringVar(&cfg.ButteraugliDistMapVideo, "butteraugli-video",
		"", "Save Butterauglis distortion map as a video")
	ButterFlags.Parse(os.Args[1:])

	CvvdpFlags := pflag.NewFlagSet("CVVDP", pflag.ContinueOnError)
	CvvdpFlags.ParseErrorsAllowlist.UnknownFlags = true
	CvvdpFlags.SortFlags = false
	CvvdpFlags.SetOutput(io.Discard)
	CvvdpFlags.BoolVar(&cfg.CVVDPUseTemporalScore, "disable-temporal", false,
		"Disable temporal pooling for CVVDP (use frame-by-frame scores)")
	CvvdpFlags.BoolVar(&cfg.CVVDPResizeToDisplay, "disable-resize", false,
		"Disable resizing videos to display resolution")
	CvvdpFlags.IntVar(&cfg.DisplayWidth, "display-width", 3840,
		"Display horizontal resolution in pixels")
	CvvdpFlags.IntVar(&cfg.DisplayHeight, "display-height", 2160,
		"Display vertical resolution in pixels")
	CvvdpFlags.Float64Var(&cfg.DisplayDiagonal, "display-diagonal", 32,
		"Display diagonal size in inches")
	CvvdpFlags.Float64Var(&cfg.ViewingDistance, "viewing-distance", 0.7472,
		"Viewing distance in meters")
	CvvdpFlags.IntVar(&cfg.MonitorContrastRatio, "display-ratio", 10000,
		"Display contrast ratio")
	CvvdpFlags.IntVar(&cfg.RoomBrightness, "room-lux", 100,
		"Ambient room illumination in lux")
	CvvdpFlags.Float64Var(&cfg.CVVDPMaxDistortionClipping, "cvvdp-clipping",
		0.75, "Save Butterauglis distortion map as a video")
	CvvdpFlags.StringVar(&cfg.CVVDPDistMapVideo, "cvvdp-video",
		"", "Save CVVDPs distortion map as a video")
	CvvdpFlags.Parse(os.Args[1:])

	OutputFlags := pflag.NewFlagSet("Output", pflag.ContinueOnError)
	OutputFlags.ParseErrorsAllowlist.UnknownFlags = true
	OutputFlags.SortFlags = false
	OutputFlags.SetOutput(io.Discard)
	OutputFlags.StringVarP(&outputPath, "output", "o", "",
		"Save per-frame JSON results to file")
	OutputFlags.StringVar(&encoderSettings, "distortion-encoder-settings",
		"-c:v libx264 -preset fast -crf 18", "FFmpeg encoder settings for "+
			"distortion map video")
	OutputFlags.Parse(os.Args[1:])

	flagSets := []*pflag.FlagSet{RequiredFlags, GeneralFlags, ButterFlags,
		CvvdpFlags, OutputFlags}

	if printHelpMessage {
		printHelpMessages(flagSets)
		os.Exit(1)
	}

	cfg.CVVDPUseTemporalScore = !cfg.CVVDPUseTemporalScore
	cfg.CVVDPResizeToDisplay = !cfg.CVVDPResizeToDisplay

	if cfg.VideoAPath == "" || cfg.VideoBPath == "" {
		fmt.Fprintln(os.Stderr, "Error: both -a and -b are required")
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

	cfg.DistortionMapEncoderSettings = strings.Split(encoderSettings, " ")

	if outputPath != "" &&
		strings.HasSuffix(outputPath, string(os.PathSeparator)) {
		fmt.Fprintln(os.Stderr, "Error: -output cannot be a directory")
		os.Exit(1)
	}

	return cfg, outputPath
}

func printHelpMessages(flagSets []*pflag.FlagSet) {
	var longestFlagName int = 0
	var longestHelpText int = 0

	for _, i := range flagSets {
		i.VisitAll(func(f *pflag.Flag) {
			if len(f.Name) > longestFlagName {
				longestFlagName = len(f.Name) + 3
			}
			if len(f.Usage) > longestHelpText {
				longestHelpText = len(f.Usage) + 3
			}

		})
	}

	printFlag := func(f *pflag.Flag) {
		var defaultText string
		if f.DefValue != "" {
			defaultText = fmt.Sprintf("%s(Default: %s)", strings.Repeat(" ",
				longestHelpText-len(f.Usage)), f.DefValue)
		}

		fmt.Fprintf(os.Stderr, "\t--%s%s %s%s\n", f.Name, strings.Repeat(" ",
			longestFlagName-len(f.Name)), f.Usage, defaultText)

	}

	for _, i := range flagSets {
		fmt.Fprintln(os.Stderr, i.Name())
		i.VisitAll(printFlag)
		fmt.Fprintln(os.Stderr, "")
	}
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

// Helper to print only selected flags in a clean way
func printFlagsByCategory(fs *pflag.FlagSet, names []string) {
	for _, name := range names {
		if f := fs.Lookup(name); f != nil {
			// Format similar to pflag's default but without the leading tab
			var line strings.Builder
			if shorthand := f.Shorthand; shorthand != "" {
				line.WriteString(fmt.Sprintf("  -%s, --%s", shorthand, f.Name))
			} else {
				line.WriteString(fmt.Sprintf("  --%s", f.Name))
			}

			// Add value name if any
			if f.Value.Type() != "bool" {
				line.WriteString(" " + f.Value.Type())
			}

			// Pad to align descriptions
			padding := 30 - line.Len()
			if padding < 1 {
				padding = 1
			}
			line.WriteString(strings.Repeat(" ", padding))
			line.WriteString(f.Usage)

			// Print default value if not empty and not bool
			if def := f.DefValue; def != "" && f.Value.Type() != "bool" {
				line.WriteString(fmt.Sprintf(" (default: %s)", def))
			}

			fmt.Fprintln(os.Stderr, line.String())
		}
	}
}

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
