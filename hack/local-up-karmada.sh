#!/usr/bin/env bash
# Copyright 2020 The Karmada Authors.
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

# This script starts a local karmada control plane based on current codebase and with a certain number of clusters joined.
# Parameters: [HOST_IPADDRESS](optional) if you want to export clusters' API server port to specific IP address
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh
# 1. used by developer to setup develop environment quickly.
# 2. used by e2e testing to setup test environment automatically.

function usage() {
    echo "Usage:"
    echo "    hack/local-up-karmada.sh [HOST_IPADDRESS] [-h]"
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

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# variable define
export KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
export HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
export KARMADA_APISERVER_CLUSTER_NAME=${KARMADA_APISERVER_CLUSTER_NAME:-"karmada-apiserver"}
export MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
export MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
export MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
export PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}
export HOST_IPADDRESS=${1:-}

# step1. set up a base development environment
"${REPO_ROOT}"/hack/setup-dev-base.sh
export KUBECONFIG="${MAIN_KUBECONFIG}"

# step2. install karmada control plane components
"${REPO_ROOT}"/hack/deploy-karmada.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

# step3. join push mode member clusters and install scheduler-estimator
GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
KARMADACTL_BIN="${GOPATH}/bin/karmadactl"
${KARMADACTL_BIN} join --karmada-context="${KARMADA_APISERVER_CLUSTER_NAME}" ${MEMBER_CLUSTER_1_NAME} --cluster-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" --cluster-context="${MEMBER_CLUSTER_1_NAME}"
"${REPO_ROOT}"/hack/deploy-scheduler-estimator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_1_NAME}"
${KARMADACTL_BIN} join --karmada-context="${KARMADA_APISERVER_CLUSTER_NAME}" ${MEMBER_CLUSTER_2_NAME} --cluster-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" --cluster-context="${MEMBER_CLUSTER_2_NAME}"
"${REPO_ROOT}"/hack/deploy-scheduler-estimator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_2_NAME}"

# step4. register pull mode member clusters and install scheduler-estimator
"${REPO_ROOT}"/hack/deploy-agent-and-estimator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${PULL_MODE_CLUSTER_NAME}"

# step5. deploy metrics-server in member clusters
"${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_1_NAME}"
"${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_2_NAME}"
"${REPO_ROOT}"/hack/deploy-k8s-metrics-server.sh "${MEMBER_CLUSTER_KUBECONFIG}" "${PULL_MODE_CLUSTER_NAME}"

# step6. wait all of clusters member1, member2 and member3 status is ready
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_1_NAME}"
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_2_NAME}"
util:wait_cluster_ready "${KARMADA_APISERVER_CLUSTER_NAME}" "${PULL_MODE_CLUSTER_NAME}"

function print_success() {
  echo -e "$KARMADA_GREETING"
  echo "Local Karmada is running."
  echo -e "\nTo start using your karmada, run:"
  echo -e "  export KUBECONFIG=${MAIN_KUBECONFIG}"
  echo "Please use 'kubectl config use-context ${HOST_CLUSTER_NAME}/${KARMADA_APISERVER_CLUSTER_NAME}' to switch the host and control plane cluster."
  echo -e "\nTo manage your member clusters, run:"
  echo -e "  export KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG}"
  echo "Please use 'kubectl config use-context ${MEMBER_CLUSTER_1_NAME}/${MEMBER_CLUSTER_2_NAME}/${PULL_MODE_CLUSTER_NAME}' to switch to the different member cluster."
}

print_success
