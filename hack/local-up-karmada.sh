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
CLUSTER_VERSION=${CLUSTER_VERSION:-"kindest/node:v1.19.1"}
KIND_LOG_FILE=${KIND_LOG_FILE:-"/tmp/karmada"}

# Make sure go exists
source "${SCRIPT_ROOT}"/hack/util.sh
util::cmd_must_exist "go"

# Check kind exists or install
if [[ ! -x $(command -v kind) ]]; then
  util::install_kind "v0.11.1"
fi

# Make sure KUBECONFIG path exists.
if [ ! -d "$KUBECONFIG_PATH" ]; then
  mkdir -p "$KUBECONFIG_PATH"
fi

util::get_macos_ipaddress # Adapt for macOS

# create a cluster to deploy karmada control plane components.
if [[ -n "${MAC_NIC_IPADDRESS}" ]]; then # install on macOS
  TEMP_PATH=$(mktemp -d)
  cp -rf "${SCRIPT_ROOT}"/artifacts/kindClusterConfig/karmada-host.yaml "${TEMP_PATH}"/karmada-host.yaml
  sed -i'' -e "s/{{host_ipaddress}}/${MAC_NIC_IPADDRESS}/g" "${TEMP_PATH}"/karmada-host.yaml
  util::create_cluster "${HOST_CLUSTER_NAME}" "${HOST_CLUSTER_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}/karmada-host.yaml"
else
  util::create_cluster "${HOST_CLUSTER_NAME}" "${HOST_CLUSTER_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}"
fi

# make controller-manager image
export VERSION="latest"
export REGISTRY="swr.ap-southeast-1.myhuaweicloud.com/karmada"
make images GOOS="linux" --directory="${SCRIPT_ROOT}"

echo "Waiting for the host clusters to be ready... it may take a long time for pulling the kind image"
util::check_clusters_ready "${HOST_CLUSTER_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

# load controller-manager image
kind load docker-image "${REGISTRY}/karmada-controller-manager:${VERSION}" --name="${HOST_CLUSTER_NAME}"

# load scheduler image
kind load docker-image "${REGISTRY}/karmada-scheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"

# load webhook image
kind load docker-image "${REGISTRY}/karmada-webhook:${VERSION}" --name="${HOST_CLUSTER_NAME}"

# deploy karmada control plane
"${SCRIPT_ROOT}"/hack/deploy-karmada.sh "${HOST_CLUSTER_KUBECONFIG}" "${HOST_CLUSTER_NAME}"
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
