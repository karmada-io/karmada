#!/usr/bin/env bash
# Copyright 2022 The Karmada Authors.
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
source "${REPO_ROOT}/hack/util.sh"

binary=$1
os=$2
arch=$3

release_dir="${REPO_ROOT}/_output/release"
mkdir -p "${release_dir}"

tar_file="${binary}-${os}-${arch}.tgz"
echo "!!! Packaging ${tar_file}"
tar czf "${release_dir}/${tar_file}" "${REPO_ROOT}/LICENSE" -C "${REPO_ROOT}/_output/bin/${os}/${arch}" "${binary}"

cd "${release_dir}"
sha256sum "${tar_file}" > "${tar_file}.sha256"
