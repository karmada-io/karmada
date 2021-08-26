#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# TODO(RainbowMango): checks if the Go version is greater than the minimum go version that we requires.

echo "vendor: running 'go mod vendor'"
go mod vendor
