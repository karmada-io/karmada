#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# The path for KUBECONFIG.
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
# The host cluster name which used to install karmada control plane components.
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}

# This script starts a local karmada control plane.
# Usage: hack/local-up-karmada.sh
# Example: hack/local-up-karmada.sh (start local karmada)

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# The KUBECONFIG path for the 'host cluster'.
HOST_CLUSTER_KUBECONFIG="${KUBECONFIG_PATH}/karmada.config"

# Make sure go exists
source "${SCRIPT_ROOT}"/hack/util.sh
util::cmd_must_exist "go"

# Make sure KUBECONFIG path exists.
if [ ! -d "$KUBECONFIG_PATH" ]; then
  mkdir -p "$KUBECONFIG_PATH"
fi

# create a cluster to deploy karmada control plane components.
"${SCRIPT_ROOT}"/hack/create-cluster.sh "${HOST_CLUSTER_NAME}" "${HOST_CLUSTER_KUBECONFIG}"

# make controller-manager image
export VERSION="latest"
make images --directory="${SCRIPT_ROOT}"

# load controller-manager image
kind load docker-image "karmada-controller-manager:${VERSION}" --name="${HOST_CLUSTER_NAME}"

# load scheduler image
kind load docker-image "karmada-scheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"

# load webhook image
kind load docker-image "karmada-webhook:${VERSION}" --name="${HOST_CLUSTER_NAME}"

# deploy karmada control plane
KARMADA_APISERVER_IP=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${HOST_CLUSTER_NAME}-control-plane")
"${SCRIPT_ROOT}"/hack/deploy-karmada.sh "${HOST_CLUSTER_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${KARMADA_APISERVER_IP}"
kubectl config use-context karmada-apiserver --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"

function print_success() {
  echo
  echo "Local Karmada is running."
  echo
  echo "Kubeconfig for karmada in file: ${HOST_CLUSTER_KUBECONFIG}, so you can run:"
cat <<EOF
  export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
EOF
  echo "Or use kubectl with --kubeconfig=${HOST_CLUSTER_KUBECONFIG}"
  echo "Please use 'kubectl config use-context <Context_Name>' to switch cluster to operate, the following is context intro:"
cat <<EOF
  ------------------------------------------------------
  |    Context Name   |          Purpose               |
  |----------------------------------------------------|
  | karmada-host      | the cluster karmada install in |
  |----------------------------------------------------|
  | karmada-apiserver | karmada control plane          |
  ------------------------------------------------------
EOF
}

print_success
