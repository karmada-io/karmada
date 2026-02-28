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

set -o errexit
set -o nounset
set -o pipefail

# This script sets up a control plane for Karmada.
# It creates a Kind cluster, pre-loaded with Karmada 
# component images built from the latest code.
# Note: This script works for both Linux and MacOS.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..
source "${REPO_ROOT}"/hack/util.sh

# variable define
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
HOST_IPADDRESS=${HOST_IPADDRESS:-}
BUILD_FROM_SOURCE=${BUILD_FROM_SOURCE:-"true"}
EXTRA_IMAGES_LOAD_TO_HOST_CLUSTER=${EXTRA_IMAGES_LOAD_TO_HOST_CLUSTER:-""}

CLUSTER_VERSION=${CLUSTER_VERSION:-"${DEFAULT_CLUSTER_VERSION}"}
KIND_LOG_FILE=${KIND_LOG_FILE:-"/tmp/karmada"}

#step0: prepare
# proxy setting in China mainland
if [[ -n ${CHINA_MAINLAND:-} ]]; then
  util::set_mirror_registry_for_china_mainland ${REPO_ROOT}
fi

# make sure go exists and the go version is a viable version.
util::cmd_must_exist "go"
util::verify_go_version

# make sure docker exists
util::cmd_must_exist "docker"

# install kind and kubectl
echo -n "Preparing: 'kind' existence check - "
if util::cmd_exist kind; then
  echo "passed"
else
  echo "not pass"
  # Install kind using the version defined in util.sh
  util::install_tools "sigs.k8s.io/kind" "${KIND_VERSION}"
fi

# get arch name and os name in bootstrap
BS_ARCH=$(go env GOARCH)
BS_OS=$(go env GOOS)
# check arch and os name before installing
util::install_environment_check "${BS_ARCH}" "${BS_OS}"
echo -n "Preparing: 'kubectl' existence check - "
if util::cmd_exist kubectl; then
  echo "passed"
else
  echo "not pass"
  util::install_kubectl "" "${BS_ARCH}" "${BS_OS}"
fi

#step1. create host cluster and member clusters in parallel
# host IP address: script parameter ahead of WSL2 or macOS IP
if [[ -z "${HOST_IPADDRESS}" ]]; then
  if util::is_wsl2; then
    util::get_wsl2_ipaddress # adapt for WSL2
    HOST_IPADDRESS=${WSL2_HOST_IP_ADDRESS:-}
  else
    util::get_macos_ipaddress # Adapt for macOS
    HOST_IPADDRESS=${MAC_NIC_IPADDRESS:-}
  fi
fi

#prepare for kindClusterConfig
TEMP_PATH=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH}; }' EXIT
echo -e "Preparing kindClusterConfig in path: ${TEMP_PATH}"

util::delete_necessary_resources "${MAIN_KUBECONFIG},${MEMBER_CLUSTER_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${KIND_LOG_FILE}"

if [[ -n "${HOST_IPADDRESS}" ]]; then # If bind the port of clusters(karmada-host) to the host IP
  util::verify_ip_address "${HOST_IPADDRESS}"
  cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/karmada-host.yaml "${TEMP_PATH}"/karmada-host.yaml
  sed -i'' -e "s/{{host_ipaddress}}/${HOST_IPADDRESS}/g" "${TEMP_PATH}"/karmada-host.yaml
  util::create_cluster "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}"/karmada-host.yaml
else
  util::create_cluster "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}"
fi

#step2. make images and get karmadactl
export VERSION="latest"
export REGISTRY="docker.io/karmada"
if [[ "${BUILD_FROM_SOURCE}" == "true" ]]; then
  export KARMADA_IMAGE_LABEL_VALUE="May_be_pruned_in_local_up_environment"
  export DOCKER_BUILD_ARGS="${DOCKER_BUILD_ARGS:-} --label=image.karmada.io=${KARMADA_IMAGE_LABEL_VALUE}"
  make images GOOS="linux" --directory="${REPO_ROOT}"
  #clean up dangling images
  docker image prune --force --filter "label=image.karmada.io=${KARMADA_IMAGE_LABEL_VALUE}"
fi
GO111MODULE=on go install "github.com/karmada-io/karmada/cmd/karmadactl"

#step3. wait until clusters ready
echo "Waiting for the clusters to be ready..."
util::check_clusters_ready "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

#step4. load components images to kind cluster
if [[ "${BUILD_FROM_SOURCE}" == "true" ]]; then
  # host cluster
  kind load docker-image "${REGISTRY}/karmada-controller-manager:${VERSION}" --name="${HOST_CLUSTER_NAME}"
  kind load docker-image "${REGISTRY}/karmada-scheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"
  kind load docker-image "${REGISTRY}/karmada-descheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"
  kind load docker-image "${REGISTRY}/karmada-webhook:${VERSION}" --name="${HOST_CLUSTER_NAME}"
  kind load docker-image "${REGISTRY}/karmada-scheduler-estimator:${VERSION}" --name="${HOST_CLUSTER_NAME}"
  kind load docker-image "${REGISTRY}/karmada-aggregated-apiserver:${VERSION}" --name="${HOST_CLUSTER_NAME}"
  kind load docker-image "${REGISTRY}/karmada-search:${VERSION}" --name="${HOST_CLUSTER_NAME}"
  kind load docker-image "${REGISTRY}/karmada-metrics-adapter:${VERSION}" --name="${HOST_CLUSTER_NAME}"
  for img in ${EXTRA_IMAGES_LOAD_TO_HOST_CLUSTER//,/ }; do
    kind load docker-image "$img" --name="${HOST_CLUSTER_NAME}"
  done
fi
