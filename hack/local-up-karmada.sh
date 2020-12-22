#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}

# This script starts a local karmada control plane.
# Usage: hack/local-up-karmada.sh
# Example: hack/local-up-karmada.sh (start local karmada)

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# Make sure KUBECONFIG path exists.
if [ ! -d "$KUBECONFIG_PATH" ]; then
  mkdir -p "$KUBECONFIG_PATH"
fi

KARMADA_KUBECONFIG="${KUBECONFIG_PATH}/karmada.config"

# create a cluster to deploy karmada control plane components.
"${SCRIPT_ROOT}"/hack/create-cluster.sh karmada "${KARMADA_KUBECONFIG}"
export KUBECONFIG="${KARMADA_KUBECONFIG}"

# make controller-manager image
export VERSION="latest"
export REGISTRY="swr.ap-southeast-1.myhuaweicloud.com/karmada"
make images

# load controller-manager image
kind load docker-image "${REGISTRY}/karmada-controller-manager:${VERSION}" --name=karmada

# deploy karmada control plane
"${SCRIPT_ROOT}"/hack/deploy-karmada.sh

function print_success() {
  echo
  echo "Local Karmada is running."
  echo "To start using your karmada, run:"
cat <<EOF
  export KUBECONFIG=${KUBECONFIG_PATH}/karmada-apiserver.config
EOF
}

print_success
