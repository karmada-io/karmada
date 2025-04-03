#!/usr/bin/env bash
# Copyright 2020 The Karmada Authors.
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


set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
GOLANGCI_LINT_VER="v2.0.2"

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
  echo 'Tips: The golangci-lint might help you fix some issues, try with the command "golangci-lint run --fix".'
  echo 'If the above warnings do not make sense, feel free to file an issue.'
  exit 1
fi
