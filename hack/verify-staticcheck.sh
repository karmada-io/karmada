#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
GOLANGCI_LINT_VER="v1.52.2"

cd "${REPO_ROOT}"
source "hack/util.sh"

if util::cmd_exist golangci-lint ; then
  echo "Using golangci-lint version:"
  golangci-lint version
else
  echo "Installing golangci-lint ${GOLANGCI_LINT_VER}"
  # https://golangci-lint.run/usage/install/#other-ci
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANGCI_LINT_VER}
fi

if golangci-lint run; then
  echo 'Congratulations!  All Go source files have passed staticcheck.'
else
  echo # print one empty line, separate from warning messages.
  echo 'Please review the above warnings.'
  echo 'If the above warnings do not make sense, feel free to file an issue.'
  exit 1
fi
