#!/bin/bash
# Copyright 2023 The Karmada Authors.
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
