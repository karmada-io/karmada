#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function usage() {
  echo "This script starts a kube cluster by kind."
  echo "Usage: hack/create-cluster.sh <CLUSTER_NAME> <KUBECONFIG>"
  echo "Example: hack/create-cluster.sh host /root/.kube/host.config"
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

kind create cluster --name "${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}" --wait=120s

# Kind cluster's context name contains a "kind-" prefix by default.
# Change context name to cluster name.
kubectl config rename-context "kind-${CLUSTER_NAME}" "${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}"

# Kind cluster uses `127.0.0.1` as kube-apiserver endpoint by default, thus kind clusters can't reach each other.
# So we need to update endpoint with container IP.
container_ip=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${CLUSTER_NAME}-control-plane")
kubectl config set-cluster "kind-${CLUSTER_NAME}" --server="https://${container_ip}:6443" --kubeconfig="${KUBECONFIG}"
