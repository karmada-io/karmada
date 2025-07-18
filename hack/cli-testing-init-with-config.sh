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

# This script starts a local karmada control plane with karmadactl and with a certain number of clusters joined.
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh
# 1. used by developer to setup develop environment quickly.
# 2. used by e2e testing to setup test environment automatically.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# variable define
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"config-member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"config-member2"}
CLUSTER_VERSION=${CLUSTER_VERSION:-"${DEFAULT_CLUSTER_VERSION}"}
BUILD_PATH=${BUILD_PATH:-"_output/bin/linux/amd64"}
CONFIG_FILE_PATH=${CONFIG_FILE_PATH:-"/tmp/karmada-config.yaml"}

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

# prepare the newest crds
echo "Prepare the newest crds"
cd  charts/karmada/
cp -r _crds crds
tar -zcvf ../../crds.tar.gz crds
cd -

# make images
export VERSION="latest"
export REGISTRY="docker.io/karmada"
make images GOOS="linux" --directory="${REPO_ROOT}"

# make karmadactl binary
make karmadactl

# create host/member1/member2 cluster
echo "Start create clusters..."
hack/create-cluster.sh ${HOST_CLUSTER_NAME} ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config > /dev/null 2>&1 &
hack/create-cluster.sh ${MEMBER_CLUSTER_1_NAME} ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config > /dev/null 2>&1 &
hack/create-cluster.sh ${MEMBER_CLUSTER_2_NAME} ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config > /dev/null 2>&1 &

# wait cluster ready
echo "Wait clusters ready..."
util::wait_file_exist ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config 300
util::wait_context_exist ${HOST_CLUSTER_NAME} ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config 300
kubectl wait --for=condition=Ready nodes --all --timeout=800s --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config
util::wait_nodes_taint_disappear 800 ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config

util::wait_file_exist ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config 300
util::wait_context_exist "${MEMBER_CLUSTER_1_NAME}" ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config 300
kubectl wait --for=condition=Ready nodes --all --timeout=800s --kubeconfig=${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config
util::wait_nodes_taint_disappear 800 ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config

util::wait_file_exist ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config 300
util::wait_context_exist "${MEMBER_CLUSTER_2_NAME}" ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config 300
kubectl wait --for=condition=Ready nodes --all --timeout=800s --kubeconfig=${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config
util::wait_nodes_taint_disappear 800 ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config

# load components images to kind cluster
kind load docker-image "${REGISTRY}/karmada-controller-manager:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-scheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-webhook:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-aggregated-apiserver:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-agent:${VERSION}" --name="${MEMBER_CLUSTER_1_NAME}"

# Ensure the parent directory of CONFIG_FILE_PATH exists
CONFIG_DIR=$(dirname "${CONFIG_FILE_PATH}")
if [ ! -d "${CONFIG_DIR}" ]; then
  echo "Creating directory ${CONFIG_DIR}..."
  mkdir -p "${CONFIG_DIR}"
fi

# build Karmada init configuration file
CONFIG_TEMPLATE=$(cat <<EOF
apiVersion: config.karmada.io/v1alpha1
kind: KarmadaInitConfig
spec:
  hostCluster:
    kubeconfig: "${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config"
  components:
    karmadaControllerManager:
      repository: "${REGISTRY}/karmada-controller-manager"
      tag: "${VERSION}"
    karmadaScheduler:
      repository: "${REGISTRY}/karmada-scheduler"
      tag: "${VERSION}"
    karmadaWebhook:
      repository: "${REGISTRY}/karmada-webhook"
      tag: "${VERSION}"
    karmadaAggregatedAPIServer:
      repository: "${REGISTRY}/karmada-aggregated-apiserver"
      tag: "${VERSION}"
  karmadaDataPath: "${HOME}/karmada"
  karmadaPkiPath: "${HOME}/karmada/pki"
  karmadaCrds: "./crds.tar.gz"
EOF
)

filled_config=$(echo "$CONFIG_TEMPLATE" | sed \
  -e "s|\${KUBECONFIG_PATH}|${KUBECONFIG_PATH}|g" \
  -e "s|\${HOST_CLUSTER_NAME}|${HOST_CLUSTER_NAME}|g" \
  -e "s|\${REGISTRY}|${REGISTRY}|g" \
  -e "s|\${VERSION}|${VERSION}|g" \
  -e "s|\${HOME}|${HOME}|g")

echo "${filled_config}" > ${CONFIG_FILE_PATH}

echo "Karmada init config file generated at ${CONFIG_FILE_PATH}"

# init Karmada control plane
echo "Start init karmada control plane..."
${BUILD_PATH}/karmadactl init --config=${CONFIG_FILE_PATH}

# join cluster
echo "Join member clusters..."
TOKEN_CMD=$(${BUILD_PATH}/karmadactl --kubeconfig ${HOME}/karmada/karmada-apiserver.config token create --print-register-command)
TOKEN=$(echo "$TOKEN_CMD" | grep -o '\--token [^ ]*' | cut -d' ' -f2)
HASH=$(echo "$TOKEN_CMD" | grep -o '\--discovery-token-ca-cert-hash [^ ]*' | cut -d' ' -f2)
ENDPOINT=$(kubectl --kubeconfig ${HOME}/karmada/karmada-apiserver.config config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed 's|^https://||')

${BUILD_PATH}/karmadactl register ${ENDPOINT} \
    --token ${TOKEN} \
    --discovery-token-ca-cert-hash ${HASH} \
    --kubeconfig=${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config \
    --cluster-name=${MEMBER_CLUSTER_1_NAME} \
    --karmada-agent-image "${REGISTRY}/karmada-agent:${VERSION}" \
    --v=4

${BUILD_PATH}/karmadactl --kubeconfig ${HOME}/karmada/karmada-apiserver.config join ${MEMBER_CLUSTER_2_NAME} --cluster-kubeconfig=${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config

kubectl wait --for=condition=Ready clusters --all --timeout=800s --kubeconfig=${HOME}/karmada/karmada-apiserver.config
