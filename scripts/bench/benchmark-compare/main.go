package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	Name           string  `json:"name"`
	Iterations     int64   `json:"iterations"`
	NsPerOp        float64 `json:"ns_per_op"`
	MemAllocsPerOp int64   `json:"mem_allocs_per_op"`
	MemBytesPerOp  int64   `json:"mem_bytes_per_op"`
	P50Ns          float64 `json:"p50_ns,omitempty"`
	P95Ns          float64 `json:"p95_ns,omitempty"`
	P99Ns          float64 `json:"p99_ns,omitempty"`
	Timestamp      string  `json:"timestamp"`
}

// HardwareInfo captures hardware fingerprint
type HardwareInfo struct {
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	NumCPU      int    `json:"num_cpu"`
	Model       string `json:"model,omitempty"`
	Fingerprint string `json:"fingerprint"`
}

// BenchmarkGolden holds all benchmark results
type BenchmarkGolden struct {
	Version    string            `json:"version"`
	Timestamp  string            `json:"timestamp"`
	GoVersion  string            `json:"go_version"`
	Hardware   HardwareInfo      `json:"hardware"`
	Reference  float64           `json:"reference_benchmark_ns"`
	Benchmarks []BenchmarkResult `json:"benchmarks"`
}

// ComparisonResult holds the comparison between two benchmark results
type ComparisonResult struct {
	Name           string
	BaselineNs     float64
	CurrentNs      float64
	NormalizedNs   float64
	DiffPercent    float64
	NormalizedDiff float64
	IsRegression   bool
	IsImprovement  bool
	BaselineMem    int64
	CurrentMem     int64
	MemDiffPercent float64
	SameHardware   bool
	Excluded       bool
	ExcludeReason  string
}

// Patterns for benchmarks to exclude from comparison (too variable or hardware-specific)
var unstablePatterns = []string{
	"Bcrypt",          // Bcrypt timing varies wildly based on CPU features
	"PasswordHashing", // Same as above
}

// Patterns that indicate micro-benchmarks (cache-sensitive, not meaningful for comparison)
var microBenchmarkPatterns = []string{
	"CacheHit",
	"CacheMiss",
}

func main() {
	baselineFile := flag.String("baseline", "", "Baseline golden file")
	currentFile := flag.String("current", "", "Current results file")
	threshold := flag.Float64("threshold", 25, "Regression threshold percentage")
	outputFile := flag.String("output", "", "Output report file")
	showAll := flag.Bool("all", false, "Show all benchmarks, not just regressions")
	normalize := flag.Bool("normalize", true, "Normalize results based on reference benchmark")
	excludePattern := flag.String("exclude", "", "Regex pattern to exclude benchmarks from comparison")
	skipUnstable := flag.Bool("skip-unstable", true, "Skip benchmarks known to be unstable across hardware")
	flag.Parse()

	if *baselineFile == "" || *currentFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --baseline and --current are required")
		flag.Usage()
		os.Exit(1)
	}

	// Load baseline
	baseline, err := loadGolden(*baselineFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading baseline: %v\n", err)
		os.Exit(1)
	}

	// Load current
	current, err := loadGolden(*currentFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading current: %v\n", err)
		os.Exit(1)
	}

	// Compare
	comparisons := compareBenchmarks(baseline, current, *threshold, *normalize, *excludePattern, *skipUnstable)

	// Sort by regression severity
	sort.Slice(comparisons, func(i, j int) bool {
		return comparisons[i].NormalizedDiff > comparisons[j].NormalizedDiff
	})

	// Generate report
	report := generateReport(comparisons, *threshold, *showAll, baseline, current)

	// Output
	if *outputFile != "" {
		// Use 0600 for file permissions (owner rw only) - benchmark reports may contain sensitive performance data
		if err := os.WriteFile(*outputFile, []byte(report), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Report saved to: %s\n", *outputFile)
	} else {
		fmt.Println(report)
	}

	// Exit with error if regressions found
	hasRegressions := false
	for _, c := range comparisons {
		if c.IsRegression {
			hasRegressions = true
			break
		}
	}
	if hasRegressions {
		os.Exit(1)
	}
}

func loadGolden(filename string) (*BenchmarkGolden, error) {
	// Prevent path traversal attacks by cleaning the path
	cleanPath := filepath.Clean(filename)
	if !filepath.IsLocal(cleanPath) {
		return nil, fmt.Errorf("invalid golden file path: must be within current directory")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, err
	}

	var golden BenchmarkGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		return nil, err
	}

	return &golden, nil
}

func compareBenchmarks(baseline, current *BenchmarkGolden, threshold float64, normalize bool, excludePattern string, skipUnstable bool) []ComparisonResult {
	var results []ComparisonResult

	// Create map for quick lookup
	baselineMap := make(map[string]BenchmarkResult)
	for _, b := range baseline.Benchmarks {
		baselineMap[b.Name] = b
	}

	// Check if hardware is the same
	sameHardware := baseline.Hardware.Fingerprint == current.Hardware.Fingerprint

	// Calculate normalization factor if needed
	normFactor := 1.0
	if normalize && !sameHardware && baseline.Reference > 0 && current.Reference > 0 {
		// Normalize current results to baseline hardware
		normFactor = baseline.Reference / current.Reference
	}

	// If hardware is very different, increase threshold
	effectiveThreshold := threshold
	if !sameHardware {
		// For different hardware, use a more lenient threshold
		// because normalization is imperfect
		effectiveThreshold = math.Max(threshold, 30)
	}

	for _, c := range current.Benchmarks {
		// Skip excluded benchmarks
		if excludePattern != "" && strings.Contains(c.Name, excludePattern) {
			continue
		}

		// Check if benchmark should be skipped due to instability
		excluded := false
		excludeReason := ""
		if skipUnstable && !sameHardware {
			for _, pattern := range unstablePatterns {
				if strings.Contains(c.Name, pattern) {
					excluded = true
					excludeReason = "Unstable across hardware (crypto timing)"
					break
				}
			}
			// Also skip micro-benchmarks that are cache-sensitive
			if !excluded {
				for _, pattern := range microBenchmarkPatterns {
					if strings.Contains(c.Name, pattern) {
						excluded = true
						excludeReason = "Micro-benchmark (cache-sensitive)"
						break
					}
				}
			}
		}

		if baseline, ok := baselineMap[c.Name]; ok {
			// Raw difference
			diffPercent := ((c.NsPerOp - baseline.NsPerOp) / baseline.NsPerOp) * 100

			// Normalized difference
			normalizedNs := c.NsPerOp * normFactor
			normalizedDiff := ((normalizedNs - baseline.NsPerOp) / baseline.NsPerOp) * 100

			// Memory difference
			memDiffPercent := float64(0)
			if baseline.MemBytesPerOp > 0 {
				memDiffPercent = ((float64(c.MemBytesPerOp) - float64(baseline.MemBytesPerOp)) / float64(baseline.MemBytesPerOp)) * 100
			}

			// For unstable benchmarks, don't mark as regression
			isRegression := false
			isImprovement := false
			if !excluded {
				isRegression = normalizedDiff > effectiveThreshold
				isImprovement = normalizedDiff < -effectiveThreshold
			}

			results = append(results, ComparisonResult{
				Name:           c.Name,
				BaselineNs:     baseline.NsPerOp,
				CurrentNs:      c.NsPerOp,
				NormalizedNs:   normalizedNs,
				DiffPercent:    diffPercent,
				NormalizedDiff: normalizedDiff,
				IsRegression:   isRegression,
				IsImprovement:  isImprovement,
				BaselineMem:    baseline.MemBytesPerOp,
				CurrentMem:     c.MemBytesPerOp,
				MemDiffPercent: memDiffPercent,
				SameHardware:   sameHardware,
				Excluded:       excluded,
				ExcludeReason:  excludeReason,
			})
		}
	}

	return results
}

func generateReport(comparisons []ComparisonResult, threshold float64, showAll bool, baseline, current *BenchmarkGolden) string {
	var sb strings.Builder

	// Check if hardware is the same
	sameHardware := baseline.Hardware.Fingerprint == current.Hardware.Fingerprint

	sb.WriteString("================================================================================\n")
	sb.WriteString("                        BENCHMARK COMPARISON REPORT\n")
	sb.WriteString("================================================================================\n\n")

	// Hardware info
	sb.WriteString(fmt.Sprintf("Baseline Hardware: %s (%s)\n", baseline.Hardware.Model, baseline.Hardware.Fingerprint[:min(16, len(baseline.Hardware.Fingerprint))]))
	sb.WriteString(fmt.Sprintf("Current Hardware:  %s (%s)\n", current.Hardware.Model, current.Hardware.Fingerprint[:min(16, len(current.Hardware.Fingerprint))]))
	if baseline.Hardware.Fingerprint != current.Hardware.Fingerprint {
		normFactor := baseline.Reference / current.Reference
		sb.WriteString("⚠️  HARDWARE MISMATCH detected\n")
		sb.WriteString("   Results are normalized, but comparison may be imprecise\n")
		sb.WriteString(fmt.Sprintf("   Normalization factor: %.2fx (based on FTS5 reference)\n", normFactor))
		sb.WriteString("   Benchmarks marked as 'unstable' are excluded from regression check\n\n")
	} else {
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Baseline:  %s\n", baseline.Timestamp))
	sb.WriteString(fmt.Sprintf("Current:   %s\n", current.Timestamp))
	sb.WriteString(fmt.Sprintf("Threshold: %.1f%%\n\n", threshold))

	// Summary
	regressions := 0
	improvements := 0
	stable := 0
	excluded := 0
	for _, c := range comparisons {
		if c.Excluded {
			excluded++
		} else if c.IsRegression {
			regressions++
		} else if c.IsImprovement {
			improvements++
		} else {
			stable++
		}
	}

	sb.WriteString("SUMMARY\n")
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("Total Benchmarks: %d\n", len(comparisons)))
	sb.WriteString(fmt.Sprintf("  Regressions:    %d\n", regressions))
	sb.WriteString(fmt.Sprintf("  Improvements:   %d\n", improvements))
	sb.WriteString(fmt.Sprintf("  Stable:         %d\n", stable))
	if excluded > 0 {
		sb.WriteString(fmt.Sprintf("  Excluded:       %d (unstable across hardware)\n", excluded))
	}
	sb.WriteString("\n")

	// Regressions
	if regressions > 0 {
		sb.WriteString("\n")
		sb.WriteString("REGRESSIONS ⚠️\n")
		sb.WriteString("================================================================================\n")
		for _, c := range comparisons {
			if c.IsRegression {
				sb.WriteString(fmt.Sprintf("\n%s\n", c.Name))
				sb.WriteString(fmt.Sprintf("  Baseline:   %.2f ns/op\n", c.BaselineNs))
				sb.WriteString(fmt.Sprintf("  Current:    %.2f ns/op (%.2f ns/op normalized)\n", c.CurrentNs, c.NormalizedNs))
				sb.WriteString(fmt.Sprintf("  Diff:       +%.2f%% (normalized: +%.2f%%) %s\n", c.DiffPercent, c.NormalizedDiff, formatPercentChange(c.NormalizedDiff)))
				if c.MemDiffPercent != 0 {
					sb.WriteString(fmt.Sprintf("  Memory:     %d → %d B/op (%.2f%%)\n", c.BaselineMem, c.CurrentMem, c.MemDiffPercent))
				}
			}
		}
	}

	// Show all if requested
	if showAll {
		sb.WriteString("\n")
		sb.WriteString("ALL BENCHMARKS\n")
		sb.WriteString("================================================================================\n")
		sb.WriteString(fmt.Sprintf("\n%-55s %12s %12s %10s\n", "Name", "Baseline", "Current", "Change"))
		sb.WriteString(strings.Repeat("-", 95) + "\n")

		for _, c := range comparisons {
			status := "✓"
			if c.IsRegression {
				status = "⚠️"
			} else if c.IsImprovement {
				status = "↑"
			}
			sb.WriteString(fmt.Sprintf("%-55s %12.2f %12.2f %9.2f%% %s\n",
				truncate(c.Name, 55), c.BaselineNs, c.NormalizedNs, c.NormalizedDiff, status))
		}
	}

	// Improvements
	if improvements > 0 {
		sb.WriteString("\n")
		sb.WriteString("IMPROVEMENTS ↑\n")
		sb.WriteString("--------------------------------------------------------------------------------\n")
		for _, c := range comparisons {
			if c.IsImprovement {
				sb.WriteString(fmt.Sprintf("  %s: %.2f → %.2f ns/op (%.2f%% faster)\n",
					c.Name, c.BaselineNs, c.NormalizedNs, -c.NormalizedDiff))
			}
		}
	}

	// Excluded benchmarks
	excludedCount := 0
	for _, c := range comparisons {
		if c.Excluded {
			excludedCount++
		}
	}
	if excludedCount > 0 {
		sb.WriteString("\n")
		sb.WriteString("EXCLUDED (Unstable across hardware) ℹ️\n")
		sb.WriteString("--------------------------------------------------------------------------------\n")
		for _, c := range comparisons {
			if c.Excluded {
				sb.WriteString(fmt.Sprintf("  %s (%s)\n", c.Name, c.ExcludeReason))
			}
		}
	}

	sb.WriteString("\n================================================================================\n")

	// Always report hardware mismatch if detected
	if regressions > 0 && !sameHardware {
		sb.WriteString(fmt.Sprintf("RESULT: FAILED - %d regression(s) detected (HARDWARE MISMATCH - results may be imprecise)\n", regressions))
		sb.WriteString("         Consider updating baseline on CI hardware for accurate comparison\n")
	} else if regressions > 0 {
		sb.WriteString(fmt.Sprintf("RESULT: FAILED - %d regression(s) detected (threshold: %.1f%%)\n", regressions, threshold))
	} else if excludedCount > 0 && !sameHardware {
		sb.WriteString(fmt.Sprintf("RESULT: WARNING - Hardware mismatch, %d benchmark(s) excluded from comparison\n", excludedCount))
		sb.WriteString("          Consider updating baseline on CI hardware for accurate comparison\n")
	} else {
		sb.WriteString("RESULT: PASSED - No regressions detected\n")
	}
	sb.WriteString("================================================================================\n")

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatPercentChange(pct float64) string {
	if pct > 15 {
		return "⚠️"
	} else if pct < -15 {
		return "↑"
	}
	return "✓"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
