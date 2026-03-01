#!/bin/bash
#
# benchmark_compare.sh - Compare benchmark results and detect regressions
#
# Usage:
#   ./scripts/benchmark_compare.sh [options]
#
# Options:
#   --baseline <file>   Baseline golden file (default: latest in golden/)
#   --current <file>    Current results file to compare
#   --threshold <pct>   Regression threshold percentage (default: 10)
#   --output <file>     Output report file (default: stdout)
#   --save-golden       Save current results as new golden file
#   --help              Show this help message
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
BASELINE_FILE=""
CURRENT_FILE=""
THRESHOLD=10
OUTPUT_FILE=""
SAVE_GOLDEN=false
GOLDEN_DIR="test/benchmarks/golden"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --baseline)
            BASELINE_FILE="$2"
            shift 2
            ;;
        --current)
            CURRENT_FILE="$2"
            shift 2
            ;;
        --threshold)
            THRESHOLD="$2"
            shift 2
            ;;
        --output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        --save-golden)
            SAVE_GOLDEN=true
            shift
            ;;
        --help)
            head -15 "$0" | tail -12
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Find latest golden file if not specified
if [[ -z "$BASELINE_FILE" ]]; then
    if [[ -d "$GOLDEN_DIR" ]]; then
        BASELINE_FILE=$(ls -t "$GOLDEN_DIR"/*.json 2>/dev/null | head -1)
        if [[ -z "$BASELINE_FILE" ]]; then
            echo -e "${YELLOW}No baseline golden file found. Run benchmarks first with --save-golden${NC}"
            exit 1
        fi
        echo -e "${BLUE}Using baseline: $BASELINE_FILE${NC}"
    else
        echo -e "${RED}Golden directory not found: $GOLDEN_DIR${NC}"
        exit 1
    fi
fi

# Check if current file is provided or run new benchmarks
if [[ -z "$CURRENT_FILE" ]]; then
    echo -e "${BLUE}Running benchmarks...${NC}"
    CURRENT_FILE="/tmp/benchmark_current_$(date +%Y%m%d_%H%M%S).json"
    go test -tags fts5 -bench=. -benchmem -json ./test/benchmarks/... 2>&1 | \
        go run ./scripts/benchmark_parse.go > "$CURRENT_FILE"
fi

# Run comparison
echo -e "${BLUE}Comparing benchmarks...${NC}"
echo -e "Threshold: ${YELLOW}${THRESHOLD}%${NC}"
echo ""

go run ./scripts/benchmark_compare_tool.go \
    --baseline "$BASELINE_FILE" \
    --current "$CURRENT_FILE" \
    --threshold "$THRESHOLD"

COMPARE_EXIT=$?

# Save as new golden if requested
if [[ "$SAVE_GOLDEN" == true ]]; then
    GOLDEN_NAME="golden_$(date +%Y%m%d_%H%M%S).json"
    cp "$CURRENT_FILE" "$GOLDEN_DIR/$GOLDEN_NAME"
    echo -e "${GREEN}Saved new golden file: $GOLDEN_DIR/$GOLDEN_NAME${NC}"
fi

exit $COMPARE_EXIT
