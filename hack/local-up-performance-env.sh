#!/usr/bin/env bash
# Copyright 2026 The Karmada Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

function usage() {
    echo "Usage:"
    echo "    hack/local-up-performance-env.sh [HOST_IPADDRESS] [-h]"
    echo "Args:"
    echo "    HOST_IPADDRESS: (optional) if you want to export clusters' API server port to specific IP address"
    echo "    h: print help information"
}

while getopts 'h' OPT; do
    case $OPT in
        h)
          usage
          exit 0
          ;;
        ?)
          usage
          exit 1
          ;;
    esac
done
shift $((OPTIND-1))

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# variable define
export KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
export HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
export KARMADA_APISERVER_CLUSTER_NAME=${KARMADA_APISERVER_CLUSTER_NAME:-"karmada-apiserver"}
export MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
export HOST_IPADDRESS=${1:-}

# step1. set up a base development environment
"${REPO_ROOT}"/hack/performance-env/setup-control-plane.sh
export KUBECONFIG="${MAIN_KUBECONFIG}"

# step2. install karmada control plane components
"${REPO_ROOT}"/hack/deploy-karmada.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

# step3. set up member clusters
"${REPO_ROOT}"/hack/performance-env/setup-member-clusters.sh

function print_success() {
  echo -e "$KARMADA_GREETING"
  echo "Local Karmada is running."
  echo -e "\nTo start using your karmada, run:"
  echo -e "  export KUBECONFIG=${MAIN_KUBECONFIG}"
  echo "Please use 'kubectl config use-context ${HOST_CLUSTER_NAME}/${KARMADA_APISERVER_CLUSTER_NAME}' to switch the host and control plane cluster."
  echo -e "\nTo manage your member clusters, run:"
  echo -e "  export KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG}"
  echo "Please use 'kubectl config use-context kwok-foo-i' to switch to the different member cluster, where i is the cluster number."
}

print_success

# step4. deploy prometheus
function deploy_prometheus() {  
    kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/hack/performance-env/prometheus/rbac.yaml"
    kubectl --context="${KARMADA_APISERVER_CLUSTER_NAME}" apply -f "${REPO_ROOT}/hack/performance-env/prometheus/rbac.yaml"
    
    KARMADA_TOKEN=$(kubectl --context="${KARMADA_APISERVER_CLUSTER_NAME}" get secret prometheus -o=jsonpath={.data.token} -n monitor | base64 -d)
    
    # Create secret with token in host cluster
    kubectl --context="${HOST_CLUSTER_NAME}" create secret generic karmada-token --from-literal=token="${KARMADA_TOKEN}" -n monitor --dry-run=client -o yaml | kubectl --context="${HOST_CLUSTER_NAME}" apply -f -
    
    kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/hack/performance-env/prometheus/application.yaml"
    
    kubectl --context="${HOST_CLUSTER_NAME}" wait --for=condition=available --timeout=180s deployment/prometheus -n monitor
}

deploy_prometheus

echo "print pod status"
kubectl --context="${HOST_CLUSTER_NAME}" get pod -n monitor
kubectl --context="${HOST_CLUSTER_NAME}" get pod -n karmada-system
