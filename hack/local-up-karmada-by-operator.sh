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
KARMADA_SYSTEM_NAMESPACE="karmada-system"

# variable define
export KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
export HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
export KARMADA_APISERVER_CLUSTER_NAME=${KARMADA_APISERVER_CLUSTER_NAME:-"karmada-apiserver"}
export MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
export MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
export MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
export PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}

# step1. set up a base development environment
"${REPO_ROOT}"/hack/setup-dev-base.sh
export KUBECONFIG="${MAIN_KUBECONFIG}"

# step2. deploy karmada-operator
"${REPO_ROOT}"/hack/deploy-karmada-operator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

# step3. install karmada instance by karmada-operator
# step3.1 prepare the local crds
echo "Prepare the local crds"
cd  ${REPO_ROOT}/charts/karmada/
cp -r _crds crds
tar -zcvf ../../crds.tar.gz crds
cd -

# step3.2 copy the local crds.tar.gz file to the specified path of the karmada-operator, so that the karmada-operator will skip the step of downloading CRDs.
CRDTARBALL_URL="http://local"
DATA_DIR="/var/lib/karmada"
CRD_CACHE_DIR=$(getCrdsDir "${DATA_DIR}" "${CRDTARBALL_URL}")
OPERATOR_POD_NAME=$(kubectl --kubeconfig="${MAIN_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" get pods -n ${KARMADA_SYSTEM_NAMESPACE} -l app.kubernetes.io/name=karmada-operator -o custom-columns=NAME:.metadata.name --no-headers)
kubectl --kubeconfig="${MAIN_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" exec -i ${OPERATOR_POD_NAME} -n ${KARMADA_SYSTEM_NAMESPACE} -- mkdir -p ${CRD_CACHE_DIR}
kubectl --kubeconfig="${MAIN_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" cp ${REPO_ROOT}/crds.tar.gz ${KARMADA_SYSTEM_NAMESPACE}/${OPERATOR_POD_NAME}:${CRD_CACHE_DIR}

# step3.3 install karmada instance
"${REPO_ROOT}"/hack/deploy-karmada-by-operator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${KARMADA_APISERVER_CLUSTER_NAME}" "latest" true "${CRDTARBALL_URL}"

# step4. join push mode member clusters
GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
KARMADACTL_BIN="${GOPATH}/bin/karmadactl"
${KARMADACTL_BIN} join --karmada-context="${KARMADA_APISERVER_CLUSTER_NAME}" ${MEMBER_CLUSTER_1_NAME} --cluster-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" --cluster-context="${MEMBER_CLUSTER_1_NAME}"
${KARMADACTL_BIN} join --karmada-context="${KARMADA_APISERVER_CLUSTER_NAME}" ${MEMBER_CLUSTER_2_NAME} --cluster-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" --cluster-context="${MEMBER_CLUSTER_2_NAME}"

# step5. register pull mode member clusters
"${REPO_ROOT}"/hack/deploy-karmada-agent.sh "${MAIN_KUBECONFIG}" "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${PULL_MODE_CLUSTER_NAME}"

# step6. deploy metrics-server in member clusters
"${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_1_NAME}"
"${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_2_NAME}"
"${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${MEMBER_CLUSTER_KUBECONFIG}" "${PULL_MODE_CLUSTER_NAME}"

# step7. wait all of clusters member1, member2 and member3 status is ready
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_1_NAME}"
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_2_NAME}"
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${PULL_MODE_CLUSTER_NAME}"

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
