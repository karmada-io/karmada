#!/usr/bin/env bash
# Copyright 2023 The Karmada Authors.
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
cd "${REPO_ROOT}"

if [[ "$(which addlicense)" == "" ]]; then
	go install github.com/google/addlicense@v1.1.1
fi
ADDLICENSE_BIN=$(which addlicense)

# verify presence of license headers and exit with non-zero code if missing
missing_license_header_files="$($ADDLICENSE_BIN \
  -check \
  -ignore "vendor/**" \
  -ignore "_output/**" \
  -ignore "samples/**" \
  -ignore "docs/**" \
  -ignore ".github/**" \
  -ignore "third_party/**" \
  -ignore "**/*.md" \
  -ignore "**/*.yaml" \
  -ignore "**/*.yml" \
  -ignore "**/*.json" \
  -ignore ".idea/**" \
  .)" || true

if [[ "$missing_license_header_files" ]]; then
  echo "Files with no license header detected:"
  echo "$missing_license_header_files"
  echo "Please add all missing license headers."
  exit 1
fi

echo "Congratulations! All files have passed license header check."
