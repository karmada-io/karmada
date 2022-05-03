#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script holds docker related functions.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}/hack/util.sh"

REGISTRY=${REGISTRY:-"swr.ap-southeast-1.myhuaweicloud.com/karmada"}
VERSION=${VERSION:="unknown"}
GO_LDFLAGS=$(util::get_GO_LDFLAGS)

function build_images() {
  local target="$1"
  docker build --build-arg GO_LDFLAGS="${GO_LDFLAGS}" -t ${REGISTRY}/${target}:${VERSION} -f ${REPO_ROOT}/cluster/images/${target}/Dockerfile ${REPO_ROOT}
}

build_images $@
