package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func printSummary(scores map[string][]float64, cfg *ComparatorConfig) {
	if len(scores) == 0 {
		fmt.Println("No scores to report")
		return
	}

	fmt.Println()
	fmt.Println("Metric summary")
	fmt.Println("==============")

	// stable order so output is deterministic
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
		printMetricSummary(name, values, cfg)
	}
}

func printMetricSummary(name string, values []float64, cfg *ComparatorConfig) {
	if name == "cvvdp" && cfg.CVVDPUseTemporalScore {
		fmt.Println()
		fmt.Println(name)
		fmt.Println(strings.Repeat("-", len(name)))
		fmt.Printf("Score: %.6f\n", values[len(values)-1])
		return
	}

	n := len(values)

	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	min := sorted[0]
	max := sorted[n-1]

	var sum float64
	for _, v := range sorted {
		sum += v
	}
	avg := sum / float64(n)

	median := func() float64 {
		if n%2 == 0 {
			return (sorted[n/2-1] + sorted[n/2]) / 2.0
		}
		return sorted[n/2]
	}()

	var variance float64
	for _, v := range sorted {
		d := v - avg
		variance += d * d
	}
	variance /= float64(n)
	stddev := math.Sqrt(variance)

	fmt.Println()
	fmt.Println(name)
	fmt.Println(strings.Repeat("-", len(name)))

	fmt.Printf("  min     : %.6f\n", min)
	fmt.Printf("  max     : %.6f\n", max)
	fmt.Printf("  average : %.6f\n", avg)
	fmt.Printf("  median  : %.6f\n", median)
	fmt.Printf("  stddev  : %.6f\n", stddev)
}
