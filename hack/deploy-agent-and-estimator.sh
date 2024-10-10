#!/bin/bash
# Copyright 2021 The Karmada Authors.
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
function usage() {
  echo "This script will deploy karmada-agent in member cluster and karmada-scheduler-estimator of the cluster in karmada-host."
  echo "Usage: hack/deploy-agent-and-estimator.sh <MEMBER_CLUSTER_NAME> <HOST_CLUSTER_KUBECONFIG> <HOST_CLUSTER_CONTEXT> <KARMADA_APISERVER_KUBECONFIG> <KARMADA_APISERVER_CONTEXT> <MEMBER_CLUSTER_KUBECONFIG> <MEMBER_CLUSTER_CONTEXT>"
  echo "Example: hack/deploy-agent-and-estimator.sh member1 ~/.kube/karmada.config karmada-host ~/.kube/karmada.config karmada-apiserver ~/.kube/members.config member1"
}

if [[ $# -ne 7 ]]; then
  usage
  exit 1
fi

# check kube config file existence
if [[ ! -f "${2}" ]]; then
  echo -e "ERROR: failed to get host kubernetes config file: '${2}', not existed.\n"
  usage
  exit 1
fi
HOST_CLUSTER_KUBECONFIG=$2

# check context existence
if ! kubectl config get-contexts "${3}" --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${3}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
HOST_CLUSTER_CONTEXT=$3


# check kube config file existence
if [[ ! -f "${4}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${4}', not existed.\n"
  usage
  exit 1
fi
KARMADA_APISERVER_KUBECONFIG=$4

# check context existence
if ! kubectl config get-contexts "${5}" --kubeconfig="${KARMADA_APISERVER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${5}' not in ${KARMADA_APISERVER_KUBECONFIG}. \n"
  usage
  exit 1
fi
KARMADA_APISERVER_CONTEXT=$5

# check kube config file existence
if [[ ! -f "${6}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${6}', not existed.\n"
  usage
  exit 1
fi
MEMBER_CLUSTER_KUBECONFIG=$6

# check context existence
if ! kubectl config get-contexts "${7}" --kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${7}' not in ${MEMBER_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
MEMBER_CLUSTER_CONTEXT=$7
MEMBER_CLUSTER_NAME=$1

"${REPO_ROOT}"/hack/deploy-scheduler-estimator.sh ${MEMBER_CLUSTER_NAME} ${HOST_CLUSTER_KUBECONFIG} ${HOST_CLUSTER_CONTEXT} ${MEMBER_CLUSTER_KUBECONFIG} ${MEMBER_CLUSTER_CONTEXT}
"${REPO_ROOT}"/hack/deploy-karmada-agent.sh ${KARMADA_APISERVER_KUBECONFIG} ${KARMADA_APISERVER_CONTEXT} ${MEMBER_CLUSTER_KUBECONFIG} ${MEMBER_CLUSTER_CONTEXT}
