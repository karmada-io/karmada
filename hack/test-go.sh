#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script runs go test.
#
# Usage:
#   hack/test-go.sh [OPTIONS]
# Options:
#   -failfast:    Do not start new tests after the first test failure.
#                 Different from `failfast` flag in `go test`, it will stop all the tests.
#
# Examples:
#   hack/test-go.sh
#   hack/test-go.sh -failfast

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd $REPO_ROOT

pid=$$
failfast=

for o in "$@"; do
  case $o in
  -failfast|--failfast)
    failfast=true
    ;;
  esac
done

go_test() {
  pkgs=$1
  go test -race -v "${pkgs}" \
    | grep -v '\[no test files\]$'\
    | (
        while read -r line; do
          echo "$line"
          if [[ "$failfast" = "true" && "$line" = "--- FAIL: "* ]]; then
            kill "$pid"
            break
          fi
        done
      )
}

go_test ./pkg/... \
        ./cmd/... \
        ./examples/...
