package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ParseBenchmarkResult represents a single benchmark result
type ParseBenchmarkResult struct {
	Name           string  `json:"name"`
	Iterations     int64   `json:"iterations"`
	NsPerOp        float64 `json:"ns_per_op"`
	MemAllocsPerOp int64   `json:"mem_allocs_per_op"`
	MemBytesPerOp  int64   `json:"mem_bytes_per_op"`
	Timestamp      string  `json:"timestamp"`
}

// HardwareInfo captures hardware fingerprint for normalization
type HardwareInfo struct {
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	NumCPU      int    `json:"num_cpu"`
	Model       string `json:"model,omitempty"`
	Fingerprint string `json:"fingerprint"`
}

// ParseBenchmarkGolden holds all benchmark results
type ParseBenchmarkGolden struct {
	Version    string                 `json:"version"`
	Timestamp  string                 `json:"timestamp"`
	GoVersion  string                 `json:"go_version"`
	Hardware   HardwareInfo           `json:"hardware"`
	Reference  float64                `json:"reference_benchmark_ns"` // FTS5 as reference
	Benchmarks []ParseBenchmarkResult `json:"benchmarks"`
}

func getHardwareInfo() HardwareInfo {
	// Try to get CPU model from /proc/cpuinfo (Linux)
	model := ""
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					model = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}

	// Create hardware fingerprint
	hw := HardwareInfo{
		GOOS:   runtime.GOOS,
		GOARCH: runtime.GOARCH,
		NumCPU: runtime.NumCPU(),
		Model:  model,
	}

	// Generate fingerprint hash (first 8 bytes)
	fingerprint := fmt.Sprintf("%s-%s-%d-%s", hw.GOOS, hw.GOARCH, hw.NumCPU, hw.Model)
	hash := sha256.Sum256([]byte(fingerprint))
	hw.Fingerprint = hex.EncodeToString(hash[:8])

	return hw
}

func main() {
	golden := ParseBenchmarkGolden{
		Version:    "2.0",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		GoVersion:  runtime.Version(),
		Hardware:   getHardwareInfo(),
		Benchmarks: make([]ParseBenchmarkResult, 0),
	}

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()

		// Parse go test -bench output format
		if strings.Contains(line, "ns/op") && strings.HasPrefix(strings.TrimSpace(line), "Benchmark") {
			parts := strings.Fields(line)
			if len(parts) >= 8 {
				name := parts[0]
				// Remove the -XX suffix (GOMAXPROCS)
				name = strings.TrimRightFunc(name, func(r rune) bool {
					return (r >= '0' && r <= '9') || r == '-'
				})

				iterations, _ := strconv.ParseInt(parts[1], 10, 64)

				nsPerOp, err := strconv.ParseFloat(parts[2], 64)
				if err != nil {
					continue
				}

				memBytes, _ := strconv.ParseInt(parts[4], 10, 64)
				memAllocs, _ := strconv.ParseInt(parts[6], 10, 64)

				golden.Benchmarks = append(golden.Benchmarks, ParseBenchmarkResult{
					Name:           name,
					Iterations:     iterations,
					NsPerOp:        nsPerOp,
					MemAllocsPerOp: memAllocs,
					MemBytesPerOp:  memBytes,
					Timestamp:      time.Now().UTC().Format(time.RFC3339),
				})

				// Capture FTS5 search as reference benchmark for normalization
				if strings.Contains(name, "BenchmarkFTS5Search") {
					golden.Reference = nsPerOp
				}
			}
		}
	}

	// If no reference found, use a default
	if golden.Reference == 0 {
		golden.Reference = 4000.0 // Default ~4000ns for FTS5 search
	}

	// Output as JSON
	data, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}
