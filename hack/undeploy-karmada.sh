#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function usage() {
  echo "This script will remove karmada control plane from a cluster."
  echo "Usage: hack/undeploy-karmada.sh"
  echo "Example: hack/undeploy-karmada.sh"
}

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_CLUSTER_KUBECONFIG="${KUBECONFIG_PATH}/karmada.config"

export KUBECONFIG=${HOST_CLUSTER_KUBECONFIG}
kubectl delete ns karmada-system
