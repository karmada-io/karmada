#!/usr/bin/env bash
# Copyright 2024 The Karmada Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script extracts command-line flags from Karmada component binaries.
# It generates JSON output that can be used for documentation and change detection.
#
# Usage:
#   ./hack/extract-flags.sh [--binary-dir=./bin] [--output=flags.json]
#
# Requirements:
#   - Built Karmada binaries in the specified directory

set -euo pipefail

# Configuration
BINARY_DIR="${BINARY_DIR:-_output/bin/linux/amd64}"
OUTPUT_FILE="${OUTPUT_FILE:-}"
VERSION="${VERSION:-$(git describe --tags --always 2>/dev/null || echo 'unknown')}"

# Karmada component binaries to extract flags from
COMPONENTS=(
    "karmada-controller-manager"
    "karmada-scheduler"
    "karmada-webhook"
    "karmada-search"
    "karmada-aggregated-apiserver"
    "karmada-descheduler"
    "karmada-metrics-adapter"
    "karmada-scheduler-estimator"
    "karmada-agent"
)

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --binary-dir=*)
            BINARY_DIR="${1#*=}"
            ;;
        --output=*)
            OUTPUT_FILE="${1#*=}"
            ;;
        --version=*)
            VERSION="${1#*=}"
            ;;
        --help)
            echo "Usage: $0 [--binary-dir=./bin] [--output=flags.json] [--version=v1.x.x]"
            echo ""
            echo "Extracts command-line flags from Karmada component binaries."
            exit 0
            ;;
        *)
            echo "Error: Unknown option $1" >&2
            exit 1
            ;;
    esac
    shift
done

# Validate binary directory
if [[ ! -d "$BINARY_DIR" ]]; then
    echo "Error: Binary directory '$BINARY_DIR' not found."
    echo "Please build Karmada first with: make all"
    exit 1
fi

# Function to extract flags from a single binary (outputs to stdout, status to stderr)
extract_flags_from_binary() {
    local binary="$1"
    local binary_path="${BINARY_DIR}/${binary}"
    
    if [[ ! -x "$binary_path" ]]; then
        return
    fi
    
    # Run --help and extract unique flag names.
    # pipefail is temporarily disabled to prevent the script from exiting if grep finds no matches.
    set +o pipefail
    "$binary_path" --help 2>&1 | grep -oE -- '--[a-zA-Z0-9_-]+' | sed 's/^--//' | sort -u
    set -o pipefail
}



# Generate JSON output for a single component
# Optimized: runs binary once, counts from stored result
generate_component_json() {
    local component="$1"
    local flags
    local flag_count
    
    # Extract flags once and store in variable
    flags=$(extract_flags_from_binary "$component")
    
    # Count flags from the stored variable
    if [[ -n "$flags" ]]; then
        flag_count=$(echo "$flags" | wc -l | tr -d ' ')
    else
        flag_count=0
    fi
    
    echo "    {"
    echo "      \"name\": \"$component\","
    echo "      \"flagCount\": $flag_count,"
    echo "      \"flags\": ["
    
    local first=true
    while IFS= read -r flag_name; do
        if [[ -n "$flag_name" ]]; then
            if [[ "$first" == "true" ]]; then
                first=false
            else
                echo ","
            fi
            echo -n "        \"$flag_name\""
        fi
    done <<< "$flags"
    
    echo ""
    echo "      ]"
    echo -n "    }"
}

# Main JSON generation
generate_json() {
    echo "{"
    echo "  \"version\": \"$VERSION\","
    echo "  \"generatedAt\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
    echo "  \"components\": ["
    
    local first=true
    for component in "${COMPONENTS[@]}"; do
        local binary_path="${BINARY_DIR}/${component}"
        
        if [[ ! -x "$binary_path" ]]; then
            echo "  Skipping $component (not found)" >&2
            continue
        fi
        
        echo "  Extracting flags from $component..." >&2
        
        if [[ "$first" == "true" ]]; then
            first=false
        else
            echo "    ,"
        fi
        
        generate_component_json "$component"
    done
    
    echo ""
    echo "  ]"
    echo "}"
}

# Main
echo "Karmada Flag Extractor" >&2
echo "======================" >&2
echo "Binary directory: $BINARY_DIR" >&2
echo "Version: $VERSION" >&2
echo "" >&2

if [[ -n "$OUTPUT_FILE" ]]; then
    generate_json > "$OUTPUT_FILE"
    echo "" >&2
    echo "Output written to: $OUTPUT_FILE" >&2
else
    generate_json
fi

echo "" >&2
echo "Done!" >&2
