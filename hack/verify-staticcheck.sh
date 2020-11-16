#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# TODO(RainbowMango): Pin golangci-lint verison to @v1.32.2
# go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.32.2

cd ${REPO_ROOT}

if golangci-lint run; then
  echo 'Congratulations!  All Go source files have passed staticcheck.'
else
  echo # print one empty line, separate from warning messages.
  echo 'Please review the above warnings.'
  echo 'If the above warnings do not make sense, feel free to file an issue.'
  exit 1
fi
