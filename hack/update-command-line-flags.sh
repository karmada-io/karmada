#!/usr/bin/env bash
# Copyright 2026 The Karmada Authors.
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

# Regenerate component command-line flag reference Markdown:
#   - docs/command-line-flags/   (karmada-controller-manager, karmada-scheduler, ...)
#
# Usage:
#   hack/update-command-line-flags.sh

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# shellcheck source=hack/util.sh
source "${SCRIPT_ROOT}/hack/util.sh"

util::verify_go_version

COMPONENT_OUT="${SCRIPT_ROOT}/docs/command-line-flags"
mkdir -p "${COMPONENT_OUT}"

cd "${SCRIPT_ROOT}"
go run ./hack/tools/gencomponentdocs/ "${COMPONENT_OUT}" all
