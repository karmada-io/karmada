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

# - download golangci-lint if needed (replace existing if incorrect version)
# - run golangci-lint

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
GOLANGCI_LINT_VER="1.59.0"
GOLANGCI_LINT="./bin/golangci-lint"

cd "${REPO_ROOT}"
mkdir -p bin

if ! (test -f "${GOLANGCI_LINT}" && ${GOLANGCI_LINT} --version | grep " ${GOLANGCI_LINT_VER} " >/dev/null); then
  rm -f ${GOLANGCI_LINT} && echo "Installing ${GOLANGCI_LINT} ${GOLANGCI_LINT_VER}"
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b bin -d "v${GOLANGCI_LINT_VER}"
fi

if ${GOLANGCI_LINT} run; then
  echo 'Congratulations!  All Go source files have passed staticcheck.'
else
  echo # print one empty line, separate from warning messages.
  echo 'Please review the above warnings.'
  echo "Tips: The ${GOLANGCI_LINT} might help you fix some issues, try with the command "${GOLANGCI_LINT} run --fix"."
  echo 'If the above warnings do not make sense, feel free to file an issue.'
  exit 1
fi
