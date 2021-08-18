#!/usr/bin/env bash
# This script only fits for Linux, macOS adaptation will come soon

set -o errexit
set -o nounset
set -o pipefail

function usage() {
  echo "This script starts a kube cluster by kind."
  echo "Usage: hack/create-cluster.sh <CLUSTER_NAME> [KUBECONFIG]"
  echo "Example: hack/create-cluster.sh host /root/.kube/karmada.config"
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

CLUSTER_NAME=$1
if [[ -z "${CLUSTER_NAME}" ]]; then
  usage
  exit 1
fi
if [[ -z "${2-}" ]]; then
  KUBECONFIG=$KUBECONFIG
else
  KUBECONFIG=$2
fi

if [[ -z "${KUBECONFIG}" ]]; then
  usage
  exit 1
fi

# check kind
REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# shellcheck source=util.sh
source "${REPO_ROOT}"/hack/util.sh
util::cmd_must_exist "kind"

if [ -f "${KUBECONFIG}" ];then
  echo "kubeconfig file is existed, new config will append to it."
  if kubectl config get-contexts "${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}"> /dev/null 2>&1;
  then
    echo "ERROR: failed to create new cluster for context '${CLUSTER_NAME}' existed in ${KUBECONFIG}. please remove it (use 'kubectl config delete-context') if your want to overwrite it."
    exit 1
  fi
fi

util::get_macos_ipaddress # Adapt for macOS

# create a cluster to deploy karmada control plane components.
if [[ -n "${MAC_NIC_IPADDRESS}" ]]; then
    CONF=`mktemp`
    PUBLIC_PORT=$(((RANDOM%(8000-7000+1))/1*1+7000))
    cp artifacts/kindClusterConfig/member-example.yaml "${CONF}"
    sed -i '' "s/MAC_NIC_IPADDRESS/${MAC_NIC_IPADDRESS}/g" "${CONF}"
    sed -i '' "s/PUBLIC_PORT/${PUBLIC_PORT}/g" "${CONF}"
    kind create cluster --name "${CLUSTER_NAME}" \
        --kubeconfig="${KUBECONFIG}" --wait=120s \
        --config=${CONF}
else
    kind create cluster --name "${CLUSTER_NAME}" \
        --kubeconfig="${KUBECONFIG}" --wait=120s
fi

# Kind cluster's context name contains a "kind-" prefix by default.
# Change context name to cluster name.
kubectl config rename-context "kind-${CLUSTER_NAME}" "${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}"

# Kind cluster uses `127.0.0.1` as kube-apiserver endpoint by default, thus kind clusters can't reach each other.
# So we need to update endpoint with container IP.
if [[ -n "${MAC_NIC_IPADDRESS}" ]]; then
    SERVER_URL="https://${MAC_NIC_IPADDRESS}:${PUBLIC_PORT}"
else
    container_ip=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${CLUSTER_NAME}-control-plane")
    SERVER_URL="https://{container_ip}:6443"
fi
kubectl config set-cluster "kind-${CLUSTER_NAME}" \
    --server="${SERVER_URL}" \
    --kubeconfig="${KUBECONFIG}"