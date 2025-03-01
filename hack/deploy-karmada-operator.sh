#!/usr/bin/env bash
# Copyright 2024 The Karmada Authors.
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

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source ${REPO_ROOT}/hack/util.sh
KARMADA_SYSTEM_NAMESPACE="karmada-system"

function usage() {
  echo "This script will deploy karmada-operator on the specified cluster"
  echo "Usage: hack/deploy-karmada-operator.sh <KUBECONFIG> <CONTEXT_NAME>"
  echo "Example: hack/deploy-karmada-operator.sh ~/.kube/config karmada-host"
}

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
KUBECONFIG=$1

# check context existence
if ! kubectl config get-contexts "${2}" --kubeconfig="${KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${2}' not in ${KUBECONFIG}. \n"
  usage
  exit 1
fi
CONTEXT_NAME=$2

# make images
export VERSION="latest"
export REGISTRY="docker.io/karmada"
make image-karmada-operator GOOS="linux" --directory=.

# load the karmada-operator images
kind load docker-image "${REGISTRY}/karmada-operator:${VERSION}" --name="${CONTEXT_NAME}"

# create namespace `karmada-system`
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"

# install Karmada operator crds
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT_NAME}" apply -f operator/config/crds/

# deploy karmada-operator
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT_NAME}" apply -f "${REPO_ROOT}/operator/config/deploy/karmada-operator-clusterrole.yaml"
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT_NAME}" apply -f "${REPO_ROOT}/operator/config/deploy/karmada-operator-clusterrolebinding.yaml"
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT_NAME}" apply -f "${REPO_ROOT}/operator/config/deploy/karmada-operator-serviceaccount.yaml"
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT_NAME}" apply -f "${REPO_ROOT}/operator/config/deploy/karmada-operator-deployment.yaml"

# wait karmada-operator ready
kubectl --kubeconfig="${KUBECONFIG}" --context="${CONTEXT_NAME}" wait --for=condition=Ready --timeout=30s pods -l app.kubernetes.io/name=karmada-operator -n ${KARMADA_SYSTEM_NAMESPACE}
