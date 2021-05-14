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
HOST_CLUSTER_KUBECONFIG="${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config"

# Make sure KUBECONFIG path exists.
if [ ! -d "$KUBECONFIG_PATH" ]; then
  mkdir -p "$KUBECONFIG_PATH"
fi

# create a cluster to deploy karmada control plane components.
"${SCRIPT_ROOT}"/hack/create-cluster.sh "${HOST_CLUSTER_NAME}" "${HOST_CLUSTER_KUBECONFIG}"

# make controller-manager image
export VERSION="latest"
export REGISTRY="swr.ap-southeast-1.myhuaweicloud.com/karmada"
make images

# load controller-manager image
kind load docker-image "${REGISTRY}/karmada-controller-manager:${VERSION}" --name="${HOST_CLUSTER_NAME}"

# load scheduler image
kind load docker-image "${REGISTRY}/karmada-scheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"

# load webhook image
kind load docker-image "${REGISTRY}/karmada-webhook:${VERSION}" --name="${HOST_CLUSTER_NAME}"

# deploy karmada control plane
KARMADA_APISERVER_IP=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${HOST_CLUSTER_NAME}-control-plane")
"${SCRIPT_ROOT}"/hack/deploy-karmada.sh "${HOST_CLUSTER_KUBECONFIG}" "kind-${HOST_CLUSTER_NAME}" "${KARMADA_APISERVER_IP}"

function print_success() {
  echo
  echo "Local Karmada is running."
  echo "To start using your karmada, run:"
cat <<EOF
  export KUBECONFIG=/var/run/karmada/karmada-apiserver.config
EOF
  echo "To start checking karmada components running status on the host cluster, please run:"
cat <<EOF
  export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
EOF
}

print_success
