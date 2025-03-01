#!/usr/bin/env bash
# Copyright 2025 The Karmada Authors.
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

# This script is used in workflow to set up a local karmada-operator e2e testing environment.
# It deploys karmada-operator and related resources to the host cluster.
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh.

function usage() {
    echo "Usage:"
    echo "    hack/operator-e2e-environment.sh [-h]"
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
