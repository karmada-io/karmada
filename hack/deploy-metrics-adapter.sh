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
  echo "This script will deploy karmada-metrics-adapter on host cluster"
  echo "Usage: hack/deploy-metrics-adapter.sh <HOST_CLUSTER_KUBECONFIG> <HOST_CONTEXT_NAME> <KARMADA_APISERVER_KUBECONFIG> <KARMADA_APISERVER_CONTEXT_NAME>"
  echo "Example: hack/deploy-metrics-adapter.sh ~/.kube/karmada.config karmada-host ~/.kube/karmada.config karmada-apiserver"
}

if [[ $# -ne 4 ]]; then
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
HOST_CONTEXT_NAME=$2

# check kube config file existence
if [[ ! -f "${3}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${3}', not existed.\n"
  usage
  exit 1
fi
KARMADA_APISERVER_KUBECONFIG=$3

# check context existence
if ! kubectl config get-contexts "${4}" --kubeconfig="${KARMADA_APISERVER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${4}' not in ${KARMADA_APISERVER_KUBECONFIG}. \n"
  usage
  exit 1
fi
KARMADA_APISERVER_CONTEXT_NAME=$4

# install metrics adapter to host cluster
if [ -n "${KUBECONFIG+x}" ];then
  CURR_KUBECONFIG=$KUBECONFIG # backup current kubeconfig
fi

export KUBECONFIG=$HOST_CLUSTER_KUBECONFIG
echo "using kubeconfig: "$KUBECONFIG

# deploy karmada-metrics-adapter
kubectl --context="${HOST_CONTEXT_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-metrics-adapter.yaml"

# make sure that karmada-metrics-adapter is ready
util::wait_pod_ready "${HOST_CONTEXT_NAME}" "${KARMADA_METRICS_ADAPTER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

export KUBECONFIG=$KARMADA_APISERVER_KUBECONFIG

# get karmada CA from configmap cluster-info, which generated in karmada-apiserver context when installing karmada.
karmada_ca=$(kubectl --context="${KARMADA_APISERVER_CONTEXT_NAME}" get cm cluster-info -n kube-public -o jsonpath='{.data.kubeconfig}' | grep 'certificate-authority-data' | awk -F ': ' '{print $2}')

# render the caBundle in apiservice with root ca, then karmada-apiserver can use caBundle to verify karmada-metrics-adapter's server-cert
TEMP_PATH_APISERVICE=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH_APISERVICE}; }' EXIT
cp -rf "${REPO_ROOT}"/artifacts/deploy/karmada-metrics-adapter-apiservice.yaml "${TEMP_PATH_APISERVICE}"/karmada-metrics-adapter-apiservice.yaml
sed -i'' -e "s/{{caBundle}}/${karmada_ca}/g" "${TEMP_PATH_APISERVICE}"/karmada-metrics-adapter-apiservice.yaml

# deploy karmada-metrics-adapter-apiservice
kubectl --context="${KARMADA_APISERVER_CONTEXT_NAME}" apply -f "${TEMP_PATH_APISERVICE}"/karmada-metrics-adapter-apiservice.yaml

# make sure that karmada-metrics-adapter-apiservice is ready
util::wait_apiservice_ready "${KARMADA_APISERVER_CONTEXT_NAME}" "${KARMADA_METRICS_ADAPTER_LABEL}"

# recover the kubeconfig before installing metrics adapter if necessary
if [ -n "${CURR_KUBECONFIG+x}" ];then
  export KUBECONFIG="${CURR_KUBECONFIG}"
else
  unset KUBECONFIG
fi

function print_success() {
  echo "Karmada metrics adapter is deployed successfully."
}

print_success
