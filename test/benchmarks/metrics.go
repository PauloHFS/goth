package benchmarks

import (
	"runtime"
	"slices"
	"time"
)

type Metrics struct {
	Durations    []time.Duration
	AllocsPerOp  int64
	BytesPerOp   int64
	RSSBytes     int64
	BusyErrors   int64
	TotalRetries int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		Durations: make([]time.Duration, 0),
	}
}

func (m *Metrics) Record(d time.Duration) {
	m.Durations = append(m.Durations, d)
}

func (m *Metrics) RecordMemory() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	m.RSSBytes = int64(mem.Sys)
}

func (m *Metrics) P50() time.Duration {
	return m.percentile(0.50)
}

func (m *Metrics) P90() time.Duration {
	return m.percentile(0.90)
}

func (m *Metrics) P95() time.Duration {
	return m.percentile(0.95)
}

func (m *Metrics) P99() time.Duration {
	return m.percentile(0.99)
}

func (m *Metrics) percentile(p float64) time.Duration {
	if len(m.Durations) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(m.Durations))
	copy(sorted, m.Durations)
	slices.Sort(sorted)
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func (m *Metrics) Mean() time.Duration {
	if len(m.Durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range m.Durations {
		total += d
	}
	return total / time.Duration(len(m.Durations))
}

func (m *Metrics) Min() time.Duration {
	if len(m.Durations) == 0 {
		return 0
	}
	min := m.Durations[0]
	for _, d := range m.Durations {
		if d < min {
			min = d
		}
	}
	return min
}

func (m *Metrics) Max() time.Duration {
	if len(m.Durations) == 0 {
		return 0
	}
	max := m.Durations[0]
	for _, d := range m.Durations {
		if d > max {
			max = d
		}
	}
	return max
}

func (m *Metrics) OpsPerSecond() float64 {
	if len(m.Durations) == 0 {
		return 0
	}
	totalSeconds := m.Mean().Seconds()
	if totalSeconds == 0 {
		return 0
	}
	return 1 / totalSeconds
}
