#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function usage() {
  echo "This script will deploy karmada control plane to a given cluster."
  echo "Usage: hack/remote-up-karmada.sh <KUBECONFIG> <CONTEXT_NAME> [LOAD_BALANCER]"
  echo "Example: hack/remote-up-karmada.sh ~/.kube/config karmada-host"
  echo -e "Parameters:\n\tKUBECONFIG\tYour cluster's kubeconfig that you want to install to"
  echo -e "\tCONTEXT_NAME\tThe name of context in 'kubeconfig'"
  echo -e "\tLOAD_BALANCER\tThis option default is 'false', and there will directly use 'hostNetwork' to communicate
  \t\t\toutside the karmada-host cluster
  \t\t\tif you want to create a 'LoadBalancer' type service for karmada apiserver, set as 'true'."
}

if [[ $# -lt 2 ]]; then
  usage
  exit 1
fi

# check config file existence
HOST_CLUSTER_KUBECONFIG=$1
if [[ ! -f "${HOST_CLUSTER_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${HOST_CLUSTER_KUBECONFIG}', not existed.\n"
  usage
  exit 1
fi

# check context existence
export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
HOST_CLUSTER_NAME=$2
if ! kubectl config get-contexts "${HOST_CLUSTER_NAME}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${HOST_CLUSTER_NAME}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi

if [ "${3:-false}" = true ]; then
  LOAD_BALANCER=true
  export LOAD_BALANCER
fi

# deploy karmada control plane
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${SCRIPT_ROOT}"/hack/util.sh

# proxy setting in China mainland
if [[ -n ${CHINA_MAINLAND:-} ]]; then
  util::set_mirror_registry_for_china_mainland ${SCRIPT_ROOT}
fi

"${SCRIPT_ROOT}"/hack/deploy-karmada.sh "${HOST_CLUSTER_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "remote"
kubectl config use-context karmada-apiserver --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"

function print_success() {
  echo -e "$KARMADA_GREETING"
  echo "Karmada is installed successfully."
  echo
  echo "Kubeconfig for karmada in file: ${HOST_CLUSTER_KUBECONFIG}, so you can run:"
  echo "  export KUBECONFIG=\"${HOST_CLUSTER_KUBECONFIG}\""
  echo "Or use kubectl with --kubeconfig=${HOST_CLUSTER_KUBECONFIG}"
  echo "Please use 'kubectl config use-context karmada-apiserver' to switch the cluster of karmada control plane"
  echo "And use 'kubectl config use-context ${HOST_CLUSTER_NAME}' for debugging karmada installation"
}

print_success
