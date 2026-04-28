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

# This script works for both linux and macOS.
set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

function usage() {
  echo "This script starts a kube cluster by kind."
  echo "Usage: hack/create-cluster.sh <CLUSTER_NAME> [KUBECONFIG]"
  echo "Example: hack/create-cluster.sh host /root/.kube/karmada.config"
}

CLUSTER_VERSION=${CLUSTER_VERSION:-"${DEFAULT_CLUSTER_VERSION}"}

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

function rand() {
    min=$1
    max=$(($2-$min+1))
    num=$(date +%s)
    echo $(($num%$max+$min))
}

function generate_cidr() {
    local val=$(rand 1 254)
    echo "$1.${val}.0.0/16"
}

POD_CIDR=${POD_CIDR:-""}
if [[ -z "${POD_CIDR}" ]]; then
    POD_CIDR=$(generate_cidr 10)
fi

SERVICE_CIDR=${SERVICE_CIDR:-""}
if [[ -z "${SERVICE_CIDR}" ]]; then
    SERVICE_CIDR=$(generate_cidr 100)
fi

#generate for kindClusterConfig
TEMP_PATH=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH}; }' EXIT
cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/general-config.yaml "${TEMP_PATH}"/"${CLUSTER_NAME}"-config.yaml
sed -i'' -e "s#{{pod_cidr}}#${POD_CIDR}#g" "${TEMP_PATH}"/"${CLUSTER_NAME}"-config.yaml
sed -i'' -e "s#{{service_cidr}}#${SERVICE_CIDR}#g" "${TEMP_PATH}"/"${CLUSTER_NAME}"-config.yaml

mkdir -p /tmp/kind-log/
kind_log="/tmp/kind-log/$(date +%s)"
echo "Creating cluster \"${CLUSTER_NAME}\" ..."
kind create cluster --name "${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}" --image="${CLUSTER_VERSION}" --config="${TEMP_PATH}"/"${CLUSTER_NAME}"-config.yaml > ${kind_log} 2>&1 || (
  echo "Creating cluster ${CLUSTER_NAME} failed, see detail log in ${kind_log}."
  exit 1
)
rm -rf "${kind_log}"

# Kind cluster's context name contains a "kind-" prefix by default.
# Change context name to cluster name.
kubectl config rename-context "kind-${CLUSTER_NAME}" "${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}"

# Kind cluster uses `127.0.0.1` as kube-apiserver endpoint by default, thus kind clusters can't reach each other.
# So we need to update endpoint with container IP.
container_ip=$(docker inspect --format='{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${CLUSTER_NAME}-control-plane")
kubectl config set-cluster "kind-${CLUSTER_NAME}" --server="https://${container_ip}:6443" --kubeconfig="${KUBECONFIG}"

echo "cluster \"${CLUSTER_NAME}\" is created successfully!"
echo "You can now use your cluster with:"
echo kubectl cluster-info --context "${CLUSTER_NAME}" --kubeconfig "${KUBECONFIG}"
