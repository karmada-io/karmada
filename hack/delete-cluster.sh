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

function usage() {
  echo "This script delete a kube cluster by kind."
  echo "Usage: hack/delete-cluster.sh <CLUSTER_NAME> <KUBECONFIG>"
  echo "Example: hack/delete-cluster.sh host /root/.kube/karmada.config"
}

if [[ $# -ne 2 ]]; then
  usage
  exit 1
fi

CLUSTER_NAME=$1
if [[ -z "${CLUSTER_NAME}" ]]; then
  usage
  exit 1
fi
KUBECONFIG=$2
if [[ -z "${KUBECONFIG}" ]]; then
  usage
  exit 1
fi

# The context name has been changed when creating clusters by 'create-cluster.sh'.
# This will result in the context can't be removed by kind when deleting a cluster.
# So, we need to change context name back and let kind take care about it.
kubectl config rename-context "${CLUSTER_NAME}" "kind-${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}"

kind delete cluster --name "${CLUSTER_NAME}" --kubeconfig="${KUBECONFIG}"
