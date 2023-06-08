#!/bin/bash

set -o errexit
set -o nounset

_tmp=$(mktemp -d)

function usage() {
  echo "This script will deploy metrics-server in member clusters."
  echo "Usage: hack/deploy-k8s-metrics-server.sh <MEMBER_CLUSTER_KUBECONFIG> <MEMBER_CLUSTER_NAME>"
  echo "Example: hack/deploy-k8s-metrics-server.sh ~/.kube/members.config member1"
}

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

if [[ $# -ne 2 ]]; then
  usage
  exit 1
fi

# check kube config file existence
if [[ ! -f "${1}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${1}', not existed.\n"
  usage
  exit 1
fi
MEMBER_CLUSTER_KUBECONFIG=$1

# check context existence
if ! kubectl config get-contexts "${2}" --kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${2}' not in ${MEMBER_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
MEMBER_CLUSTER_NAME=$2

# get deploy yaml
wget https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.6.3/components.yaml -O "${_tmp}/components.yaml"
sed -i'' -e 's/args:/args:\n        - --kubelet-insecure-tls=true/' "${_tmp}/components.yaml"

# deploy metrics-server in member cluster
kubectl --kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" --context="${MEMBER_CLUSTER_NAME}" apply -f "${_tmp}/components.yaml"
