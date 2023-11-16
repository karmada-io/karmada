#!/bin/bash
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

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh
function usage() {
  echo "This script will deploy karmada-opensearch on host cluster"
  echo "Usage: hack/deploy-karmada-opensearch.sh  <HOST_CLUSTER_KUBECONFIG> <HOST_CLUSTER_NAME>>"
  echo "Example: hack/deploy-karmada-opensearch.sh ~/.kube/config karmada-host"
}

if [[ $# -ne 2 ]]; then
  usage
  exit 1
fi

# check kube config file existence
if [[ ! -f "${1}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${1}', not existed.\n"
  usage
  exit 1
fi
HOST_CLUSTER_KUBECONFIG=$1

# check context existence
if ! kubectl config get-contexts "${2}" --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${2}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
HOST_CLUSTER_NAME=$2

# install opensearch to host cluster
if [ -n "${KUBECONFIG+x}" ];then
  CURR_KUBECONFIG=$KUBECONFIG # backup current kubeconfig
fi

export KUBECONFIG=$HOST_CLUSTER_KUBECONFIG
echo "using kubeconfig: "$KUBECONFIG

# deploy karmada opensearch
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/opensearch/karmada-opensearch.yaml"
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/opensearch/karmada-opensearch-dashboards.yaml"

# make sure all karmada-opensearch components are ready
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KARMADA_OPENSEARCH_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KARMADA_OPENSEARCH_DASHBOARDS_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# recover the kubeconfig before installing opensearch if necessary
if [ -n "${CURR_KUBECONFIG+x}" ];then
  export KUBECONFIG="${CURR_KUBECONFIG}"
else
  unset KUBECONFIG
fi

function print_success() {
  echo "Opensearch is deployed successfully."
  echo "You can access the opensearch at http://karmada-opensearch.karmada-system.svc:9200"
}

print_success
