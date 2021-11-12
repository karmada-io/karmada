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
  echo "This script will remove karmada control plane from a cluster."
  echo "Usage: hack/undeploy-karmada.sh [KUBECONFIG] [CONTEXT_NAME]"
  echo "Example: hack/undeploy-karmada.sh ~/.kube/karmada.config karmada-host"
}

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# delete all keys and certificates
rm -fr "${HOME}/.karmada"
# set default host cluster's kubeconfig file (created by kind)
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_CLUSTER_KUBECONFIG="${KUBECONFIG_PATH}/karmada.config"
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}

# if provider the custom kubeconfig and context name
if [[ -f "${1:-}" ]]; then
  if ! kubectl config get-contexts "${2:-}" --kubeconfig="${1}" > /dev/null 2>&1;
  then
    echo -e "ERROR: failed to get context: '${2:-}' not in ${1}. \n"
    usage
    exit 1
  else
    HOST_CLUSTER_KUBECONFIG=${1:-}
    HOST_CLUSTER_NAME=${2:-}
  fi
fi

kubectl config use-context "${HOST_CLUSTER_NAME}" --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"

# clear all in namespace karmada-system
kubectl delete ns karmada-system --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"

# clear configs about karmada-apiserver in kubeconfig
kubectl config delete-cluster karmada-apiserver --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"
kubectl config unset users.karmada-apiserver --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"
kubectl config delete-context karmada-apiserver --kubeconfig="${HOST_CLUSTER_KUBECONFIG}"
