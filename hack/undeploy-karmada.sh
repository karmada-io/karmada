#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function usage() {
  echo "This script will remove karmada control plane from a cluster."
  echo "Usage: hack/undeploy-karmada.sh [KUBECONFIG] [CONTEXT_NAME]"
  echo "Example: hack/undeploy-karmada.sh ~/.kube/karmada.config karmada-host"
}

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# delete all keys and certificates
rm -fr "${HOME}/.karmada"
# set default host cluster's kubeconfig file (created by kind)
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_CLUSTER_KUBECONFIG="${KUBECONFIG_PATH}/karmada.config"
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}

# if provider the custom kubeconfig and context name
if [[ -f "${1:-}" ]]; then
  if ! kubectl config get-contexts "${2:-}" --kubeconfig="${1}" > /dev/null 2>&1;
  then
    echo -e "ERROR: failed to get context: '${2:-}' not in ${1}. \n"
    usage
    exit 1
  else
    HOST_CLUSTER_KUBECONFIG=${1:-}
    HOST_CLUSTER_NAME=${2:-}
  fi
fi

kubectl config use-context "${HOST_CLUSTER_NAME}" --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"

# clear all in namespace karmada-system
kubectl delete ns karmada-system --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"

# clear configs about karmada-apiserver in kubeconfig
kubectl config delete-cluster karmada-apiserver --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"
kubectl config delete-user karmada-apiserver --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"
kubectl config delete-context karmada-apiserver --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"
