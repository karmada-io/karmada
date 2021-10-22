#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

util::verify_go_version

echo "running 'go mod tidy'"
go mod tidy

echo "running 'go mod vendor'"
go mod vendor
