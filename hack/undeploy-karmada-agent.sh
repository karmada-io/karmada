#!/bin/bash

set -o errexit
set -o nounset

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

function usage() {
  echo "This script will undeploy karmada agent from a cluster."
  echo "Usage: hack/undeploy-karmada-agent.sh <KARMADA_APISERVER_KUBECONFIG> <KARMADA_APISERVER_CONTEXT_NAME> <MEMBER_CLUSTER_KUBECONFIG> <MEMBER_CLUSTER_CONTEXT_NAME>"
  echo "Example: hack/undeploy-karmada-agent.sh ~/.kube/karmada.config karmada-apiserver ~/.kube/members.config member1"
}

if [[ $# -ne 4 ]]; then
  usage
  exit 1
fi

# check kube config file existence
if [[ ! -f "${1}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${1}', not existed.\n"
  usage
  exit 1
fi
KARMADA_APISERVER_KUBECONFIG=$1

# check context existence
if ! kubectl config use-context "${2}" --kubeconfig="${KARMADA_APISERVER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to use context: '${2}' not in ${KARMADA_APISERVER_KUBECONFIG}. \n"
  usage
  exit 1
fi
KARMADA_APISERVER_CONTEXT_NAME=$2

# check kube config file existence
if [[ ! -f "${3}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${3}', not existed.\n"
  usage
  exit 1
fi
MEMBER_CLUSTER_KUBECONFIG=$3

# check context existence
if ! kubectl config get-contexts "${4}" --kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${4}' not in ${MEMBER_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
MEMBER_CLUSTER_NAME=$4

source "${REPO_ROOT}"/hack/util.sh

# remove the member cluster from karmada control plane
kubectl delete cluster ${MEMBER_CLUSTER_NAME}

# remove agent from the member cluster
if [ -n "${KUBECONFIG+x}" ];then
  CURR_KUBECONFIG=$KUBECONFIG # backup current kubeconfig
fi
export KUBECONFIG="${MEMBER_CLUSTER_KUBECONFIG}" # switch to member cluster
kubectl config use-context "${MEMBER_CLUSTER_NAME}"

# remove namespace of karmada agent
kubectl delete -f "${REPO_ROOT}/artifacts/agent/namespace.yaml"
kubectl delete namespace karmada-cluster

# remove clusterrole and clusterrolebinding of karmada agent
kubectl delete -f "${REPO_ROOT}/artifacts/agent/clusterrole.yaml"
kubectl delete -f "${REPO_ROOT}/artifacts/agent/clusterrolebinding.yaml"

# recover the kubeconfig after removing agent if necessary
if [ -n "${CURR_KUBECONFIG+x}" ];then
  export KUBECONFIG="${CURR_KUBECONFIG}"
else
  unset KUBECONFIG
fi
