#!/usr/bin/env bash

#   Copyright The Karmada Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

# This script holds docker related functions.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
REGISTRY=${REGISTRY:-"swr.ap-southeast-1.myhuaweicloud.com/karmada"}
VERSION=${VERSION:="unknown"}

function build_images() {
  local target="$1"
  docker build -t ${REGISTRY}/${target}:${VERSION} -f ${REPO_ROOT}/cluster/images/${target}/Dockerfile ${REPO_ROOT}
}

build_images $@
