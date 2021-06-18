#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
GOLANGCI_LINT_VER="v1.40.1"

cd "${REPO_ROOT}"
source "hack/util.sh"

# directly download binary, details refer to https://golangci-lint.run/usage/install
command -v golangci-lint &>/dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANGCI_LINT_VER}

function lint() {
  local ret=0

  golangci-lint run || ret=1

  # to test with specified build tags in hack, default skip them
  # typecheck works bad with build-tags, so skip it
  sed -i "/typecheck/d" ./.golangci.yml
  golangci-lint run --build-tags=tools --disable=typecheck ./hack/... || ret=1

  return $ret
}

if lint; then
  echo 'Congratulations!  All Go source files have passed staticcheck.'
else
  echo # print one empty line, separate from warning messages.
  echo 'Please review the above warnings.'
  echo 'If the above warnings do not make sense, feel free to file an issue.'
  exit 1
fi
