#!/usr/bin/env bash

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
