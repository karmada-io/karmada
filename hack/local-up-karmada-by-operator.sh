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

# This script is used in workflow to validate karmada installation by operator.
# It starts a local karmada control plane based on current codebase via karmada-operator and with a certain number of clusters joined.
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh
# 1. used by developer to setup develop environment quickly.
# 2. used by e2e testing to test if the operator installs correctly.

function usage() {
    echo "Usage:"
    echo "    hack/local-up-karmada-by-operator.sh [-h]"
    echo "    h: print help information"
}

function getCrdsDir() {
  local path=$1
  local url=$2
  local key=$(echo "$url" | xargs)  # Trim whitespace using xargs
  local hash=$(echo -n "$key" | sha256sum | awk '{print $1}')  # Calculate SHA256 hash
  local hashedKey=${hash:0:64}  # Take the first 64 characters of the hash
  echo "${path}/cache/${hashedKey}"
}

while getopts 'h' OPT; do
    case $OPT in
        h)
          usage
          exit 0
          ;;
        ?)
          usage
          exit 1
          ;;
    esac
done

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# variable define
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
KARMADA_APISERVER_CLUSTER_NAME=${KARMADA_APISERVER_CLUSTER_NAME:-"karmada-apiserver"}
MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}
MEMBER_TMP_CONFIG_PREFIX="member-tmp"
MEMBER_CLUSTER_1_TMP_CONFIG="${KUBECONFIG_PATH}/${MEMBER_TMP_CONFIG_PREFIX}-${MEMBER_CLUSTER_1_NAME}.config"
MEMBER_CLUSTER_2_TMP_CONFIG="${KUBECONFIG_PATH}/${MEMBER_TMP_CONFIG_PREFIX}-${MEMBER_CLUSTER_2_NAME}.config"
PULL_MODE_CLUSTER_TMP_CONFIG="${KUBECONFIG_PATH}/${MEMBER_TMP_CONFIG_PREFIX}-${PULL_MODE_CLUSTER_NAME}.config"
CLUSTER_VERSION=${CLUSTER_VERSION:-"${DEFAULT_CLUSTER_VERSION}"}
KIND_LOG_FILE=${KIND_LOG_FILE:-"/tmp/karmada"}
KARMADA_SYSTEM_NAMESPACE="karmada-system"
KARMADA_INSTANCE_NAME=${KARMADA_INSTANCE_NAME:-"karmada-demo"}
KARMADA_INSTANCE_NAMESPACE=${KARMADA_INSTANCE_NAMESPACE:-"test"}

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
kind_version=v0.25.0
echo -n "Preparing: 'kind' existence check - "
if util::cmd_exist kind; then
  echo "passed"
else
  echo "not pass"
  util::install_tools "sigs.k8s.io/kind" $kind_version
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
#prepare for kindClusterConfig
util::delete_necessary_resources "${MAIN_KUBECONFIG},${MEMBER_CLUSTER_KUBECONFIG}" "${HOST_CLUSTER_NAME},${MEMBER_CLUSTER_1_NAME},${MEMBER_CLUSTER_2_NAME},${PULL_MODE_CLUSTER_NAME}" "${KIND_LOG_FILE}"

util::create_cluster "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}"
util::create_cluster "${MEMBER_CLUSTER_1_NAME}" "${MEMBER_CLUSTER_1_TMP_CONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${REPO_ROOT}"/artifacts/kindClusterConfig/member1.yaml
util::create_cluster "${MEMBER_CLUSTER_2_NAME}" "${MEMBER_CLUSTER_2_TMP_CONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${REPO_ROOT}"/artifacts/kindClusterConfig/member2.yaml
util::create_cluster "${PULL_MODE_CLUSTER_NAME}" "${PULL_MODE_CLUSTER_TMP_CONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${REPO_ROOT}"/artifacts/kindClusterConfig/member3.yaml

#step2. make images and get karmadactl
export VERSION="latest"
export REGISTRY="docker.io/karmada"
export KARMADA_IMAGE_LABEL_VALUE="May_be_pruned_in_local-up-karmada_by_operator.sh"
export DOCKER_BUILD_ARGS="${DOCKER_BUILD_ARGS:-} --label=image.karmada.io=${KARMADA_IMAGE_LABEL_VALUE}"
make images GOOS="linux" --directory="${REPO_ROOT}"
#clean up dangling images
docker image prune --force --filter "label=image.karmada.io=${KARMADA_IMAGE_LABEL_VALUE}"

GO111MODULE=on go install "github.com/karmada-io/karmada/cmd/karmadactl"
GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
KARMADACTL_BIN="${GOPATH}/bin/karmadactl"

#step3. wait until the host cluster ready
echo "Waiting for the host clusters to be ready..."
util::check_clusters_ready "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

# load components images to host cluster
kind load docker-image "${REGISTRY}/karmada-controller-manager:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-scheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-descheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-webhook:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-aggregated-apiserver:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-search:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-metrics-adapter:${VERSION}" --name="${HOST_CLUSTER_NAME}"

#step4. deploy karmada-operator
"${REPO_ROOT}"/hack/deploy-karmada-operator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

#step5. install karmada instance by karmada-operator
# prepare the local crds
echo "Prepare the local crds"
cd  ${REPO_ROOT}/charts/karmada/
cp -r _crds crds
tar -zcvf ../../crds.tar.gz crds
cd -

# copy the local crds.tar.gz file to the specified path of the karmada-operator, so that the karmada-operator will skip the step of downloading CRDs.
CRDTARBALL_URL="http://local"
DATA_DIR="/var/lib/karmada"
CRD_CACHE_DIR=$(getCrdsDir "${DATA_DIR}" "${CRDTARBALL_URL}")
OPERATOR_POD_NAME=$(kubectl --kubeconfig="${MAIN_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" get pods -n ${KARMADA_SYSTEM_NAMESPACE} -l karmada-app=karmada-operator -o custom-columns=NAME:.metadata.name --no-headers)
kubectl --kubeconfig="${MAIN_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" exec -i ${OPERATOR_POD_NAME} -n ${KARMADA_SYSTEM_NAMESPACE} -- mkdir -p ${CRD_CACHE_DIR}
kubectl --kubeconfig="${MAIN_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" cp ${REPO_ROOT}/crds.tar.gz ${KARMADA_SYSTEM_NAMESPACE}/${OPERATOR_POD_NAME}:${CRD_CACHE_DIR}

# install karmada instance
"${REPO_ROOT}"/hack/deploy-karmada-by-operator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${KARMADA_APISERVER_CLUSTER_NAME}" "${VERSION}" true "${CRDTARBALL_URL}"

#step6. join member clusters
# wait until member clusters ready
util::check_clusters_ready "${MEMBER_CLUSTER_1_TMP_CONFIG}" "${MEMBER_CLUSTER_1_NAME}"
util::check_clusters_ready "${MEMBER_CLUSTER_2_TMP_CONFIG}" "${MEMBER_CLUSTER_2_NAME}"
# connecting networks between karmada-host, member1 and member2 clusters
echo "connecting cluster networks..."
util::add_routes "${MEMBER_CLUSTER_1_NAME}" "${MEMBER_CLUSTER_2_TMP_CONFIG}" "${MEMBER_CLUSTER_2_NAME}"
util::add_routes "${MEMBER_CLUSTER_2_NAME}" "${MEMBER_CLUSTER_1_TMP_CONFIG}" "${MEMBER_CLUSTER_1_NAME}"

util::add_routes "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_1_TMP_CONFIG}" "${MEMBER_CLUSTER_1_NAME}"
util::add_routes "${MEMBER_CLUSTER_1_NAME}" "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

util::add_routes "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_2_TMP_CONFIG}" "${MEMBER_CLUSTER_2_NAME}"
util::add_routes "${MEMBER_CLUSTER_2_NAME}" "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"
echo "cluster networks connected"

#join push mode member clusters
export KUBECONFIG="${MAIN_KUBECONFIG}"
${KARMADACTL_BIN} join --karmada-context="${KARMADA_APISERVER_CLUSTER_NAME}" ${MEMBER_CLUSTER_1_NAME} --cluster-kubeconfig="${MEMBER_CLUSTER_1_TMP_CONFIG}" --cluster-context="${MEMBER_CLUSTER_1_NAME}"
${KARMADACTL_BIN} join --karmada-context="${KARMADA_APISERVER_CLUSTER_NAME}" ${MEMBER_CLUSTER_2_NAME} --cluster-kubeconfig="${MEMBER_CLUSTER_2_TMP_CONFIG}" --cluster-context="${MEMBER_CLUSTER_2_NAME}"

# wait until the pull mode cluster ready
util::check_clusters_ready "${PULL_MODE_CLUSTER_TMP_CONFIG}" "${PULL_MODE_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-agent:${VERSION}" --name="${PULL_MODE_CLUSTER_NAME}"

#step7. deploy karmada agent in pull mode member clusters
"${REPO_ROOT}"/hack/deploy-karmada-agent.sh "${MAIN_KUBECONFIG}" "${KARMADA_APISERVER_CLUSTER_NAME}" "${PULL_MODE_CLUSTER_TMP_CONFIG}" "${PULL_MODE_CLUSTER_NAME}"

#step8. deploy metrics-server in member clusters
"${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${MEMBER_CLUSTER_1_TMP_CONFIG}" "${MEMBER_CLUSTER_1_NAME}"
"${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${MEMBER_CLUSTER_2_TMP_CONFIG}" "${MEMBER_CLUSTER_2_NAME}"
"${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${PULL_MODE_CLUSTER_TMP_CONFIG}" "${PULL_MODE_CLUSTER_NAME}"

# wait all of clusters member1, member2 and member3 status is ready
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_1_NAME}"
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_2_NAME}"
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${PULL_MODE_CLUSTER_NAME}"

#step9. merge temporary kubeconfig of member clusters by kubectl
export KUBECONFIG=$(find ${KUBECONFIG_PATH} -maxdepth 1 -type f | grep ${MEMBER_TMP_CONFIG_PREFIX} | tr '\n' ':')
kubectl config view --flatten > ${MEMBER_CLUSTER_KUBECONFIG}
rm $(find ${KUBECONFIG_PATH} -maxdepth 1 -type f | grep ${MEMBER_TMP_CONFIG_PREFIX})

function print_success() {
  echo -e "$KARMADA_GREETING"
  echo "Local Karmada is running."
  echo -e "\nTo start using your karmada, run:"
  echo -e "  export KUBECONFIG=${MAIN_KUBECONFIG}"
  echo "Please use 'kubectl config use-context ${HOST_CLUSTER_NAME}/${KARMADA_APISERVER_CLUSTER_NAME}' to switch the host and control plane cluster."
  echo -e "\nTo manage your member clusters, run:"
  echo -e "  export KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG}"
  echo "Please use 'kubectl config use-context ${MEMBER_CLUSTER_1_NAME}/${MEMBER_CLUSTER_2_NAME}/${PULL_MODE_CLUSTER_NAME}' to switch to the different member cluster."
}

print_success
