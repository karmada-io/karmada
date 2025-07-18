#!/usr/bin/env bash
# Copyright 2024 The Karmada Authors.
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

# This script sets up a development environment for installing Karmada locally.
# It creates multiple Kind clusters, including a host cluster pre-loaded with
# Karmada component images built from the latest code. The remaining clusters
# will serve as member clusters and will be registered to the Karmada control
# plane using the installation tool.
# Note: This script works for both Linux and MacOS.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# variable define
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}
MEMBER_TMP_CONFIG_PREFIX="member-tmp"
MEMBER_CLUSTER_1_TMP_CONFIG="${KUBECONFIG_PATH}/${MEMBER_TMP_CONFIG_PREFIX}-${MEMBER_CLUSTER_1_NAME}.config"
MEMBER_CLUSTER_2_TMP_CONFIG="${KUBECONFIG_PATH}/${MEMBER_TMP_CONFIG_PREFIX}-${MEMBER_CLUSTER_2_NAME}.config"
PULL_MODE_CLUSTER_TMP_CONFIG="${KUBECONFIG_PATH}/${MEMBER_TMP_CONFIG_PREFIX}-${PULL_MODE_CLUSTER_NAME}.config"
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
# host IP address: script parameter ahead of macOS IP
if [[ -z "${HOST_IPADDRESS}" ]]; then
  util::get_macos_ipaddress # Adapt for macOS
  HOST_IPADDRESS=${MAC_NIC_IPADDRESS:-}
fi
#prepare for kindClusterConfig
TEMP_PATH=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH}; }' EXIT
echo -e "Preparing kindClusterConfig in path: ${TEMP_PATH}"
cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/member1.yaml "${TEMP_PATH}"/member1.yaml
cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/member2.yaml "${TEMP_PATH}"/member2.yaml
cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/member3.yaml "${TEMP_PATH}"/member3.yaml

util::delete_necessary_resources "${MAIN_KUBECONFIG},${MEMBER_CLUSTER_KUBECONFIG}" "${HOST_CLUSTER_NAME},${MEMBER_CLUSTER_1_NAME},${MEMBER_CLUSTER_2_NAME},${PULL_MODE_CLUSTER_NAME}" "${KIND_LOG_FILE}"

if [[ -n "${HOST_IPADDRESS}" ]]; then # If bind the port of clusters(karmada-host, member1 and member2) to the host IP
  util::verify_ip_address "${HOST_IPADDRESS}"
  cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/karmada-host.yaml "${TEMP_PATH}"/karmada-host.yaml
  sed -i'' -e "s/{{host_ipaddress}}/${HOST_IPADDRESS}/g" "${TEMP_PATH}"/karmada-host.yaml
  sed -i'' -e 's/networking:/&\'$'\n''  apiServerAddress: "'${HOST_IPADDRESS}'"/' "${TEMP_PATH}"/member1.yaml
  sed -i'' -e 's/networking:/&\'$'\n''  apiServerAddress: "'${HOST_IPADDRESS}'"/' "${TEMP_PATH}"/member2.yaml
  sed -i'' -e 's/networking:/&\'$'\n''  apiServerAddress: "'${HOST_IPADDRESS}'"/' "${TEMP_PATH}"/member3.yaml
  util::create_cluster "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}"/karmada-host.yaml
else
  util::create_cluster "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}"
fi
util::create_cluster "${MEMBER_CLUSTER_1_NAME}" "${MEMBER_CLUSTER_1_TMP_CONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}"/member1.yaml
util::create_cluster "${MEMBER_CLUSTER_2_NAME}" "${MEMBER_CLUSTER_2_TMP_CONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}"/member2.yaml
util::create_cluster "${PULL_MODE_CLUSTER_NAME}" "${PULL_MODE_CLUSTER_TMP_CONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}"/member3.yaml

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
util::check_clusters_ready "${MEMBER_CLUSTER_1_TMP_CONFIG}" "${MEMBER_CLUSTER_1_NAME}"
util::check_clusters_ready "${MEMBER_CLUSTER_2_TMP_CONFIG}" "${MEMBER_CLUSTER_2_NAME}"
util::check_clusters_ready "${PULL_MODE_CLUSTER_TMP_CONFIG}" "${PULL_MODE_CLUSTER_NAME}"

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
  # pull mode member cluster
  kind load docker-image "${REGISTRY}/karmada-agent:${VERSION}" --name="${PULL_MODE_CLUSTER_NAME}"
fi

#step5. connecting networks between karmada-host, member1 and member2 clusters
echo "connecting cluster networks..."
util::add_routes "${MEMBER_CLUSTER_1_NAME}" "${MEMBER_CLUSTER_2_TMP_CONFIG}" "${MEMBER_CLUSTER_2_NAME}"
util::add_routes "${MEMBER_CLUSTER_2_NAME}" "${MEMBER_CLUSTER_1_TMP_CONFIG}" "${MEMBER_CLUSTER_1_NAME}"

util::add_routes "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_1_TMP_CONFIG}" "${MEMBER_CLUSTER_1_NAME}"
util::add_routes "${MEMBER_CLUSTER_1_NAME}" "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

util::add_routes "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_2_TMP_CONFIG}" "${MEMBER_CLUSTER_2_NAME}"
util::add_routes "${MEMBER_CLUSTER_2_NAME}" "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"
echo "cluster networks connected"

#step6. merge temporary kubeconfig of member clusters by kubectl
export KUBECONFIG=$(find ${KUBECONFIG_PATH} -maxdepth 1 -type f | grep ${MEMBER_TMP_CONFIG_PREFIX} | tr '\n' ':')
kubectl config view --flatten > ${MEMBER_CLUSTER_KUBECONFIG}
rm $(find ${KUBECONFIG_PATH} -maxdepth 1 -type f | grep ${MEMBER_TMP_CONFIG_PREFIX})
