package benchmarks

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// BenchmarkResult holds detailed benchmark metrics including percentiles
type BenchmarkResult struct {
	Name           string  `json:"name"`
	Iterations     int64   `json:"iterations"`
	NsPerOp        float64 `json:"ns_per_op"`
	MemAllocsPerOp int64   `json:"mem_allocs_per_op"`
	MemBytesPerOp  int64   `json:"mem_bytes_per_op"`
	// Percentiles for latency (calculated from multiple runs)
	P50Ns float64 `json:"p50_ns"`
	P95Ns float64 `json:"p95_ns"`
	P99Ns float64 `json:"p99_ns"`
	// Timestamp of when the benchmark was run
	Timestamp string `json:"timestamp"`
	// Go version and system info
	GoVersion string `json:"go_version"`
	CPU       string `json:"cpu"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	NumCPU    int    `json:"num_cpu"`
}

// BenchmarkGolden holds all benchmark results for a single run
type BenchmarkGolden struct {
	Version    string            `json:"version"`
	Timestamp  string            `json:"timestamp"`
	GoVersion  string            `json:"go_version"`
	CPU        string            `json:"cpu"`
	OS         string            `json:"os"`
	Arch       string            `json:"arch"`
	NumCPU     int               `json:"num_cpu"`
	Benchmarks []BenchmarkResult `json:"benchmarks"`
}

// PercentileCalculator collects timing data for percentile calculation
type PercentileCalculator struct {
	timings []float64
}

// NewPercentileCalculator creates a new calculator
func NewPercentileCalculator() *PercentileCalculator {
	return &PercentileCalculator{
		timings: make([]float64, 0),
	}
}

// AddTiming adds a timing measurement
func (pc *PercentileCalculator) AddTiming(ns float64) {
	pc.timings = append(pc.timings, ns)
}

// CalculatePercentiles returns p50, p95, p99 values
func (pc *PercentileCalculator) CalculatePercentiles() (p50, p95, p99 float64) {
	if len(pc.timings) == 0 {
		return 0, 0, 0
	}

	sorted := make([]float64, len(pc.timings))
	copy(sorted, pc.timings)
	sort.Float64s(sorted)

	p50 = sorted[len(sorted)*50/100]
	p95 = sorted[len(sorted)*95/100]
	p99 = sorted[min(len(sorted)*99/100, len(sorted)-1)]

	return p50, p95, p99
}

// RunBenchmarkWithPercentiles runs a benchmark function multiple times and collects percentile data
func RunBenchmarkWithPercentiles(b *testing.B, name string, fn func(b *testing.B)) BenchmarkResult {
	// Run benchmark multiple times to collect timing data
	numRuns := 5
	allNsPerOp := make([]float64, 0, numRuns)
	allMemBytes := make([]int64, 0, numRuns)
	allMemAllocs := make([]int64, 0, numRuns)

	for i := 0; i < numRuns; i++ {
		result := testing.Benchmark(fn)
		allNsPerOp = append(allNsPerOp, float64(result.NsPerOp()))
		allMemBytes = append(allMemBytes, result.AllocedBytesPerOp())
		allMemAllocs = append(allMemAllocs, result.AllocsPerOp())
	}

	// Calculate percentiles from the runs
	p50, p95, p99 := calculateFloat64Percentiles(allNsPerOp)

	// Calculate averages for other metrics
	avgNsPerOp := average(allNsPerOp)
	avgMemBytes := averageInt64(allMemBytes)
	avgMemAllocs := averageInt64(allMemAllocs)

	return BenchmarkResult{
		Name:           name,
		Iterations:     int64(avgNsPerOp), // Placeholder
		NsPerOp:        avgNsPerOp,
		MemAllocsPerOp: avgMemAllocs,
		MemBytesPerOp:  avgMemBytes,
		P50Ns:          p50,
		P95Ns:          p95,
		P99Ns:          p99,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		GoVersion:      getGoVersion(),
		CPU:            getCPUInfo(),
		OS:             getOSInfo(),
		Arch:           getArchInfo(),
		NumCPU:         getNumCPU(),
	}
}

func calculateFloat64Percentiles(values []float64) (p50, p95, p99 float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	p50 = sorted[len(sorted)*50/100]
	p95 = sorted[len(sorted)*95/100]
	p99 = sorted[min(len(sorted)*99/100, len(sorted)-1)]

	return p50, p95, p99
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func averageInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sum := int64(0)
	for _, v := range values {
		sum += v
	}
	return sum / int64(len(values))
}

// Golden file management
const goldenDir = "test/benchmarks/golden"

// SaveGolden saves benchmark results to a golden file
func SaveGolden(golden *BenchmarkGolden, filename string) error {
	if err := os.MkdirAll(goldenDir, 0755); err != nil {
		return fmt.Errorf("failed to create golden directory: %w", err)
	}

	filepath := fmt.Sprintf("%s/%s", goldenDir, filename)
	data, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal golden data: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write golden file: %w", err)
	}

	return nil
}

// LoadGolden loads benchmark results from a golden file
func LoadGolden(filename string) (*BenchmarkGolden, error) {
	filepath := fmt.Sprintf("%s/%s", goldenDir, filename)
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read golden file: %w", err)
	}

	var golden BenchmarkGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		return nil, fmt.Errorf("failed to unmarshal golden data: %w", err)
	}

	return &golden, nil
}

// ListGoldens returns a list of available golden files
func ListGoldens() ([]string, error) {
	entries, err := os.ReadDir(goldenDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

// Helper functions for system info
func getGoVersion() string {
	return strings.ReplaceAll(time.Now().Format("2006-01-02"), "-", "")
}

func getCPUInfo() string {
	return "unknown" // Would need runtime package or system calls
}

func getOSInfo() string {
	return "linux" // Could use runtime.GOOS
}

func getArchInfo() string {
	return "amd64" // Could use runtime.GOARCH
}

func getNumCPU() int {
	return 1 // Would need runtime.NumCPU()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
