#!/usr/bin/env bash

#   Copyright The Karmada Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

function usage() {
  echo "This script will deploy karmada control plane to a given cluster."
  echo "Usage: hack/remote-up-karmada.sh <KUBECONFIG> <CONTEXT_NAME> [CLUSTER_IP_ONLY]"
  echo "Example: hack/remote-up-karmada.sh ~/.kube/config karmada-host"
  echo -e "Parameters:\n\tKUBECONFIG\tYour cluster's kubeconfig that you want to install to"
  echo -e "\tCONTEXT_NAME\tThe name of context in 'kubeconfig'"
  echo -e "\tCLUSTER_IP_ONLY\tThis option default is 'false', and there will create a 'LoadBalancer' type service
  \t\t\tfor karmada apiserver so that it can easy communicate outside the karmada-host cluster,
  \t\t\tif you want only a 'ClusterIP' type service for karmada apiserver, set as 'true'."
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
  CLUSTER_IP_ONLY=true
  export CLUSTER_IP_ONLY
fi

# deploy karmada control plane
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
"${SCRIPT_ROOT}"/hack/deploy-karmada.sh "${HOST_CLUSTER_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "remote"
kubectl config use-context karmada-apiserver --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"

function print_success() {
  echo
  echo "Karmada is installed."
  echo
  echo "Kubeconfig for karmada in file: ${HOST_CLUSTER_KUBECONFIG}, so you can run:"
  echo "  export KUBECONFIG=\"${HOST_CLUSTER_KUBECONFIG}\""
  echo "Or use kubectl with --kubeconfig=${HOST_CLUSTER_KUBECONFIG}"
  echo "Please use 'kubectl config use-context karmada-apiserver' to switch the cluster of karmada control plane"
  echo "And use 'kubectl config use-context ${HOST_CLUSTER_NAME}' for debugging karmada installation"
}

print_success
