package perf

import (
	"sync"
	"time"
)

// Timer measures operation latency.
type Timer struct {
	mu      sync.Mutex
	starts  map[string]time.Time
	results map[string][]time.Duration
}

// NewTimer creates a new Timer.
func NewTimer() *Timer {
	return &Timer{
		starts:  make(map[string]time.Time),
		results: make(map[string][]time.Duration),
	}
}

// Start records the start time for an operation.
func (t *Timer) Start(op string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.starts[op] = time.Now()
}

// Stop records the elapsed time for an operation and returns the duration.
func (t *Timer) Stop(op string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	start, ok := t.starts[op]
	if !ok {
		return 0
	}
	d := time.Since(start)
	t.results[op] = append(t.results[op], d)
	delete(t.starts, op)
	return d
}

// OperationStats holds aggregate statistics for a single operation.
type OperationStats struct {
	Operation    string  `json:"operation"`
	Count        int     `json:"count"`
	AvgMs        float64 `json:"avg_ms"`
	MinMs        float64 `json:"min_ms"`
	MaxMs        float64 `json:"max_ms"`
	OpsPerSecond float64 `json:"ops_per_second"`
}

// PerfReport is a map of operation name to aggregate stats.
type PerfReport map[string]OperationStats

// Report computes aggregate stats for all recorded operations.
func (t *Timer) Report() PerfReport {
	t.mu.Lock()
	defer t.mu.Unlock()

	report := make(PerfReport, len(t.results))
	for op, durations := range t.results {
		if len(durations) == 0 {
			continue
		}
		var total time.Duration
		minD := durations[0]
		maxD := durations[0]
		for _, d := range durations {
			total += d
			if d < minD {
				minD = d
			}
			if d > maxD {
				maxD = d
			}
		}
		avgMs := float64(total.Nanoseconds()) / float64(len(durations)) / 1e6
		minMs := float64(minD.Nanoseconds()) / 1e6
		maxMs := float64(maxD.Nanoseconds()) / 1e6
		opsPerSec := 0.0
		if avgMs > 0 {
			opsPerSec = 1000.0 / avgMs
		}
		report[op] = OperationStats{
			Operation:    op,
			Count:        len(durations),
			AvgMs:        avgMs,
			MinMs:        minMs,
			MaxMs:        maxMs,
			OpsPerSecond: opsPerSec,
		}
	}
	return report
}

// BenchResult holds the result of a benchmark run.
type BenchResult struct {
	Iterations   int     `json:"iterations"`
	TotalMs      float64 `json:"total_ms"`
	AvgMs        float64 `json:"avg_ms"`
	MinMs        float64 `json:"min_ms"`
	MaxMs        float64 `json:"max_ms"`
	OpsPerSecond float64 `json:"ops_per_second"`
}

// Benchmark runs fn n times and measures latency.
func Benchmark(fn func(), n int) BenchResult {
	if n <= 0 {
		n = 1
	}
	durations := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		fn()
		durations[i] = time.Since(start)
	}

	var total time.Duration
	minD := durations[0]
	maxD := durations[0]
	for _, d := range durations {
		total += d
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}

	totalMs := float64(total.Nanoseconds()) / 1e6
	avgMs := totalMs / float64(n)
	minMs := float64(minD.Nanoseconds()) / 1e6
	maxMs := float64(maxD.Nanoseconds()) / 1e6
	opsPerSec := 0.0
	if avgMs > 0 {
		opsPerSec = 1000.0 / avgMs
	}

	return BenchResult{
		Iterations:   n,
		TotalMs:      totalMs,
		AvgMs:        avgMs,
		MinMs:        minMs,
		MaxMs:        maxMs,
		OpsPerSecond: opsPerSec,
	}
}
