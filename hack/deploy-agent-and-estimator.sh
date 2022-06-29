#!/bin/bash

set -o errexit
set -o nounset

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
function usage() {
  echo "This script will deploy karmada-agent in member cluster and karmada-scheduler-estimator of the cluster in karmada-host."
  echo "Usage: hack/deploy-agent-and-estimator.sh <HOST_CLUSTER_KUBECONFIG> <HOST_CLUSTER_NAME> <KARMADA_APISERVER_KUBECONFIG> <KARMADA_APISERVER_CONTEXT_NAME> <MEMBER_CLUSTER_KUBECONFIG> <MEMBER_CLUSTER_NAME>"
  echo "Example: hack/deploy-agent-and-estimator.sh ~/.kube/karmada.config karmada-host ~/.kube/karmada.config karmada-apiserver ~/.kube/members.config member1"
}

if [[ $# -ne 6 ]]; then
  usage
  exit 1
fi

# check kube config file existence
if [[ ! -f "${1}" ]]; then
  echo -e "ERROR: failed to get host kubernetes config file: '${1}', not existed.\n"
  usage
  exit 1
fi
HOST_CLUSTER_KUBECONFIG=$1

# check context existence
if ! kubectl config get-contexts "${2}" --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${2}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
HOST_CLUSTER_NAME=$2


# check kube config file existence
if [[ ! -f "${3}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${3}', not existed.\n"
  usage
  exit 1
fi
KARMADA_APISERVER_KUBECONFIG=$3

# check context existence
if ! kubectl config get-contexts "${4}" --kubeconfig="${KARMADA_APISERVER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${4}' not in ${KARMADA_APISERVER_KUBECONFIG}. \n"
  usage
  exit 1
fi
KARMADA_APISERVER_CONTEXT_NAME=$4

# check kube config file existence
if [[ ! -f "${5}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${5}', not existed.\n"
  usage
  exit 1
fi
MEMBER_CLUSTER_KUBECONFIG=$5

# check context existence
if ! kubectl config get-contexts "${6}" --kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${6}' not in ${MEMBER_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
MEMBER_CLUSTER_NAME=$6


"${REPO_ROOT}"/hack/deploy-scheduler-estimator.sh ${HOST_CLUSTER_KUBECONFIG} ${HOST_CLUSTER_NAME} ${MEMBER_CLUSTER_KUBECONFIG} ${MEMBER_CLUSTER_NAME}
"${REPO_ROOT}"/hack/deploy-karmada-agent.sh ${KARMADA_APISERVER_KUBECONFIG} ${KARMADA_APISERVER_CONTEXT_NAME} ${MEMBER_CLUSTER_KUBECONFIG} ${MEMBER_CLUSTER_NAME}
