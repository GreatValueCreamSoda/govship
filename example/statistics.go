package main

import (
	"fmt"
	"math"
	"sort"
)

func printSummary(scores map[string][]float64) {
	fmt.Println()
	fmt.Println("Metric summary")
	fmt.Println("--------------")

	for name, values := range scores {
		if len(values) == 0 {
			continue
		}
		printMetricSummary(name, values)
	}
}

func printMetricSummary(name string, values []float64) {
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	min := sorted[0]
	max := sorted[n-1]

	var sum float64
	for _, v := range sorted {
		sum += v
	}
	avg := float64(sum) / float64(n)

	stats := map[string]func() string{
		"count": func() string {
			return fmt.Sprintf("%d", n)
		},
		"min": func() string {
			return fmt.Sprintf("%f", min)
		},
		"max": func() string {
			return fmt.Sprintf("%f", max)
		},
		"average": func() string {
			return fmt.Sprintf("%.3f", avg)
		},
		"median": func() string {
			if n%2 == 0 {
				return fmt.Sprintf(
					"%.3f",
					float64(sorted[n/2-1]+sorted[n/2])/2.0,
				)
			}
			return fmt.Sprintf("%.3f", float64(sorted[n/2]))
		},
		"stddev": func() string {
			var variance float64
			for _, v := range sorted {
				d := float64(v) - avg
				variance += d * d
			}
			variance /= float64(n)
			return fmt.Sprintf("%.3f", math.Sqrt(variance))
		},
		"p5": func() string {
			i := int(math.Round(0.05 * float64(n-1)))
			return fmt.Sprintf("%.3f", float64(sorted[i]))
		},
		"p95": func() string {
			i := int(math.Round(0.95 * float64(n-1)))
			return fmt.Sprintf("%.3f", float64(sorted[i]))
		},
	}

	maxLabel := 0
	for label := range stats {
		if len(label) > maxLabel {
			maxLabel = len(label)
		}
	}

	fmt.Println(name)
	for label, fn := range stats {
		fmt.Printf(
			"  %*s : %10s\n",
			maxLabel,
			label,
			fn(),
		)
	}
	fmt.Println()
}
