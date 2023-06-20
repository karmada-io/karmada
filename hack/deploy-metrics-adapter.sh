#!/bin/bash

set -o errexit
set -o nounset

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh
function usage() {
  echo "This script will deploy karmada-metrics-adapter on host cluster"
  echo "Usage: hack/deploy-metrics-adapter.sh  <HOST_CLUSTER_KUBECONFIG> <HOST_CLUSTER_NAME>>"
  echo "Example: hack/deploy-metrics-adapter.sh ~/.kube/config karmada-host"
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

# install metrics adapter to host cluster
if [ -n "${KUBECONFIG+x}" ];then
  CURR_KUBECONFIG=$KUBECONFIG # backup current kubeconfig
fi

export KUBECONFIG=$HOST_CLUSTER_KUBECONFIG
echo "using kubeconfig: "$KUBECONFIG

# deploy karmada-metrics-adapter
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-metrics-adapter.yaml"

# make sure that karmada-metrics-adapter is ready
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${KARMADA_METRICS_ADAPTER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# deploy karmada-metrics-adapter-apiservice
kubectl --context="karmada-apiserver" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-metrics-adapter-apiservice.yaml"

# make sure that karmada-metrics-adapter-apiservice is ready
util::wait_apiservice_ready "karmada-apiserver" "${KARMADA_METRICS_ADAPTER_LABEL}"

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
