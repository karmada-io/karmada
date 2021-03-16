#!/bin/bash

set -o errexit
set -o nounset

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KARMADA_APISERVER_KUBECONFIG=${KARMADA_APISERVER_KUBECONFIG:-"/var/run/karmada/karmada-apiserver.config"}

# The host cluster name which used to install karmada control plane components.
MEMBER_CLUSTER_NAME=${MEMBER_CLUSTER_NAME:-"member3"}
MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${HOME}/.kube/member3.config"}

AGENT_POD_LABEL="karmada-agent"
source ${REPO_ROOT}/hack/util.sh
function usage() {
  echo "This script will deploy karmada agent to a cluster."
  echo "Usage: hack/deploy-karmada-agent.sh"
  echo "Example: hack/deploy-karmada.sh"
}

export REGISTRY="swr.ap-southeast-1.myhuaweicloud.com/karmada"
export VERSION="latest"
kind load docker-image "${REGISTRY}/karmada-agent:${VERSION}" --name="${MEMBER_CLUSTER_NAME}"

export KUBECONFIG="${MEMBER_CLUSTER_KUBECONFIG}"

# create namespace for karmada agent
kubectl apply -f "${REPO_ROOT}/artifacts/agent/namespace.yaml"

# create service account, cluster role for karmada agent
kubectl apply -f "${REPO_ROOT}/artifacts/agent/serviceaccount.yaml"
kubectl apply -f "${REPO_ROOT}/artifacts/agent/clusterrole.yaml"
kubectl apply -f "${REPO_ROOT}/artifacts/agent/clusterrolebinding.yaml"

# create secret
if [[ ! -e ${KARMADA_APISERVER_KUBECONFIG} ]]; then
    echo "the kubeconfig file of karmada control plane not exist"
    exit 1
fi
kubectl create secret generic karmada-kubeconfig --from-file=karmada-kubeconfig="$KARMADA_APISERVER_KUBECONFIG" -n karmada-system

# deploy karmada agent
cp "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml.tmp
sed -i "s/{{member_cluster_name}}/${MEMBER_CLUSTER_NAME}/g" "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml
kubectl apply -f "${REPO_ROOT}/artifacts/agent/karmada-agent.yaml"
mv "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml.tmp "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml

# Wait for karmada-etcd to come up before launching the rest of the components.
util::wait_pod_ready ${AGENT_POD_LABEL} "karmada-system"

