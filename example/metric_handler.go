package main

// MetricHandler is the interface that every metric must implement
type MetricHandler interface {
	Name() string
	Close()
	Compute(a, b *frame) (map[string]float64, error)
}
