#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function usage() {
  echo "This script delete a kube cluster by kind."
  echo "Usage: hack/delete-cluster.sh <CLUSTER_NAME> <KUBECONFIG>"
  echo "Example: hack/delete-cluster.sh host /root/.kube/karmada.config"
}

if [[ $# -ne 2 ]]; then
  usage
  exit 1
fi

CLUSTER_NAME=$1
if [[ -z "${CLUSTER_NAME}" ]]; then
  usage
  exit 1
fi
KUBECONFIG=$2
if [[ -z "${KUBECONFIG}" ]]; then
  usage
  exit 1
fi

# The context name has been changed when creating clusters by 'create-cluster.sh'.
# This will result in the context can't be removed by kind when deleting a cluster.
# So, we need to change context name back and let kind take care about it.
kubectl config rename-context "${CLUSTER_NAME}" "kind-${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}"

kind delete cluster --name "${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}"
