#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function usage() {
  echo "This script will deploy karmada control plane to a given cluster."
  echo "Usage: hack/deploy-karmada.sh <KUBECONFIG> <CONTEXT_NAME>"
  echo "Example: hack/deploy-karmada.sh ~/.kube/config karmada-host"
}

if [[ $# -ne 2 ]]; then
  usage
  exit 1
fi

# check config file existence
HOST_CLUSTER_KUBECONFIG=$1
if [[ ! -f "${HOST_CLUSTER_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${HOST_CLUSTER_KUBECONFIG}', not existed.\n"
  usage
  exit 1
fi

# check context existence
export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
HOST_CLUSTER_NAME=$2
if ! kubectl config get-contexts "${HOST_CLUSTER_NAME}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${HOST_CLUSTER_NAME}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi

# deploy karmada control plane
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KARMADA_APISERVER_IP=$(kubectl cluster-info --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" | head -1 | sed -n 's/\([0-9]\+\.[0-9]\+\.[0-9]\+\.[0-9]\+\).*/\n\1/;s/^.*\n//p')
"${SCRIPT_ROOT}"/hack/deploy-karmada.sh "${HOST_CLUSTER_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${KARMADA_APISERVER_IP}"

function print_success() {
  echo
  echo "Karmada is installed."
  echo
  echo "Kubeconfig for karmada in file: ${HOST_CLUSTER_KUBECONFIG}, so you can run:"
  echo "  export KUBECONFIG=\"${HOST_CLUSTER_KUBECONFIG}\""
  echo "Or use kubectl with --kubeconfig=${HOST_CLUSTER_KUBECONFIG}"
  echo "Please use 'kubectl config use-context karmada-apiserver' to switch the cluster of karmada control plane"
  echo "And use 'kubectl config use-context ${HOST_CLUSTER_NAME}' for debugging karmada installation"
}

print_success

