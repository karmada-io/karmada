#!/usr/bin/env bash
# Copyright 2021 The Karmada Authors.
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

# This script holds docker related functions.
# You can set the platform to build with BUILD_PLATFORMS, with format: `<os>/<arch>`
# When `OUTPUT_TYPE=docker` is set, `BUILD_PLATFORMS` cannot be set with multi platforms.
# See: https://github.com/docker/buildx/issues/59
#
# Usage:
#   hack/docker.sh <target>
# Args:
#   $1:              target to build
# Environments:
#   BUILD_PLATFORMS:  platforms to build. You can set one or more platforms separated by comma.
#                     e.g.: linux/amd64,linux/arm64
#   OUTPUT_TYPE       Destination to save image(`docker`/`registry`/`local,dest=path`, default is `docker`).
#   REGISTRY          image registry
#   VERSION           image version
#   DOCKER_BUILD_ARGS additional arguments to the docker build command
#   SIGN_IMAGE        enabled sign image with cosign, disabled by default.
# Examples:
#   hack/docker.sh karmada-aggregated-apiserver
#   BUILD_PLATFORMS=linux/amd64 hack/docker.sh karmada-aggregated-apiserver
#   OUTPUT_TYPE=registry BUILD_PLATFORMS=linux/amd64,linux/arm64 hack/docker.sh karmada-aggregated-apiserver
#   DOCKER_BUILD_ARGS="--build-arg https_proxy=${https_proxy}" hack/docker.sh karmada-aggregated-apiserver"
#   SIGN_IMAGE="1"

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}/hack/util.sh"

REGISTRY=${REGISTRY:-"docker.io/karmada"}
VERSION=${VERSION:="unknown"}
DOCKER_BUILD_ARGS=${DOCKER_BUILD_ARGS:-}
SIGN_IMAGE=${SIGN_IMAGE:-"0"}

function build_images() {
  local -r target=$1
  local -r output_type=${OUTPUT_TYPE:-docker}
  local platforms="${BUILD_PLATFORMS:-"$(util:host_platform)"}"

  # Preferentially use `docker build`. If we are building multi platform,
  # or cross building, change to `docker buildx build`
  cross=$(isCross "${platforms}")
  if [[ "${cross}" == "true" ]]; then
    build_cross_image "${output_type}" "${target}" "${platforms}"
  else
    build_local_image "${output_type}" "${target}" "${platforms}"
  fi
}

function build_local_image() {
  local -r output_type=$1
  local -r target=$2
  local -r platform=$3

  local -r image_name="${REGISTRY}/${target}:${VERSION}"

  echo "Building image for ${platform}: ${image_name}"
  set -x
  docker build --build-arg BINARY="${target}" \
          ${DOCKER_BUILD_ARGS} \
          --tag "${image_name}" \
          --file "${REPO_ROOT}/cluster/images/Dockerfile" \
          "${REPO_ROOT}/_output/bin/${platform}"
  set +x

  if [[ "$output_type" == "registry" ]]; then
    docker push "${image_name}"
    signImage ${image_name}
  fi
}

function build_cross_image() {
  local -r output_type=$1
  local -r target=$2
  local -r platforms=$3

  local -r image_name="${REGISTRY}/${target}:${VERSION}"

  echo "Cross building image for ${platforms}: ${image_name}"
  set -x
  docker buildx build --output=type="${output_type}" \
          --platform "${platforms}" \
          --build-arg BINARY="${target}" \
          ${DOCKER_BUILD_ARGS} \
          --tag "${image_name}" \
          --file "${REPO_ROOT}/cluster/images/buildx.Dockerfile" \
          "${REPO_ROOT}/_output/bin"
  signImage ${image_name}
  set +x
}

function signImage(){
  if [ $SIGN_IMAGE = "1" ];then
    local -r target=$1
    echo "Signing image: "${target}
    cosign sign --yes ${target}
  fi
}

function isCross() {
  local platforms=$1

  IFS="," read -ra platform_array <<< "${platforms}"
  if [[ ${#platform_array[@]} -ne 1 ]]; then
    echo true
    return
  fi

  local -r arch=${platforms##*/}
  if [[ "$arch" == $(go env GOHOSTARCH) ]]; then
    echo false
  else
    echo true
  fi
}

build_images $@
