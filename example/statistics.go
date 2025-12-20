package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// printSummary displays a human-readable summary of all metric scores to stderr.
// This keeps stdout clean for potential future machine-readable output (e.g., distortion maps, JSON).
// It includes per-metric statistics and pairwise absolute Pearson correlations when multiple metrics exist.
func printSummary(scores map[string][]float64) {
	if len(scores) == 0 {
		fmt.Fprintln(os.Stderr, "No scores to report")
		return
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Metric summary")
	fmt.Fprintln(os.Stderr, "==============")

	// Get sorted metric names for deterministic output
	names := make([]string, 0, len(scores))
	for name := range scores {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		values := scores[name]
		if len(values) == 0 {
			continue
		}
		printMetricSummary(name, values)
	}

	if len(names) > 1 {
		printCorrelations(scores, names)
	}
}

// printMetricSummary prints statistical summary for a single metric to stderr.
func printMetricSummary(name string, values []float64) {
	n := len(values)

	// Work on a sorted copy for min/max/median
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	min := sorted[0]
	max := sorted[n-1]

	// Mean
	var sum float64
	for _, v := range values {
		sum += v
	}
	avg := sum / float64(n)

	// Median
	var median float64
	if n%2 == 1 {
		median = sorted[n/2]
	} else {
		median = (sorted[n/2-1] + sorted[n/2]) / 2.0
	}

	// Population standard deviation
	var variance float64
	for _, v := range values {
		d := v - avg
		variance += d * d
	}
	variance /= float64(n)
	stddev := math.Sqrt(variance)

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, name)
	fmt.Fprintln(os.Stderr, strings.Repeat("-", len(name)))

	fmt.Fprintf(os.Stderr, "  min     : %.6f\n", min)
	fmt.Fprintf(os.Stderr, "  max     : %.6f\n", max)
	fmt.Fprintf(os.Stderr, "  average : %.6f\n", avg)
	fmt.Fprintf(os.Stderr, "  median  : %.6f\n", median)
	fmt.Fprintf(os.Stderr, "  stddev  : %.6f\n", stddev)
}

// printCorrelations prints pairwise absolute Pearson correlations between metrics to stderr.
func printCorrelations(scores map[string][]float64, names []string) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Metric correlations")
	fmt.Fprintln(os.Stderr, "===================")

	// Calculate max name length for alignment
	maxLen := 0
	for _, name := range names {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	formatStr := fmt.Sprintf("  %%-%ds ↔ %%-%ds : %% .6f\n", maxLen, maxLen)

	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			a, b := names[i], names[j]
			x, y := scores[a], scores[b]

			if len(x) == 0 || len(y) == 0 || len(x) != len(y) {
				continue
			}

			r := pearsonCorrelation(x, y)
			fmt.Fprintf(os.Stderr, formatStr, a, b, math.Abs(r))
		}
	}
}

// pearsonCorrelation computes the Pearson correlation coefficient.
// Returns 0 if inputs are empty, mismatched, or perfectly constant.
func pearsonCorrelation(x, y []float64) float64 {
	n := len(x)
	if n == 0 || n != len(y) {
		return 0
	}

	var sumX, sumY float64
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
	}

	meanX := sumX / float64(n)
	meanY := sumY / float64(n)

	var num, denomX, denomY float64
	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		num += dx * dy
		denomX += dx * dx
		denomY += dy * dy
	}

	denom := math.Sqrt(denomX * denomY)
	if denom == 0 {
		return 0
	}

	return num / denom
}
