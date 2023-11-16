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
  echo "This script will deploy karmada-scheduler-estimator of a cluster."
  echo "Usage: hack/deploy-scheduler-estimator.sh <HOST_CLUSTER_KUBECONFIG> <HOST_CLUSTER_NAME> <MEMBER_CLUSTER_KUBECONFIG> <MEMBER_CLUSTER_NAME>"
  echo "Example: hack/deploy-scheduler-estimator.sh ~/.kube/karmada.config karmada-host ~/.kube/members.config member1"
}

if [[ $# -ne 4 ]]; then
  usage
  exit 1
fi

# check kube config file existence
if [[ ! -f "${1}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${1}', not existed.\n"
  usage
  exit 1
fi
HOST_CLUSTER_KUBECONFIG=$1

# check context existence
if ! kubectl config get-contexts "${2}" --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${2}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
HOST_CLUSTER_NAME=$2

# check kube config file existence
if [[ ! -f "${3}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${3}', not existed.\n"
  usage
  exit 1
fi
MEMBER_CLUSTER_KUBECONFIG=$3

# check context existence
if ! kubectl config get-contexts "${4}" --kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${4}' not in ${MEMBER_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
MEMBER_CLUSTER_NAME=$4

TEMP_PATH="$(mktemp -d)"
MEMBER_CLUSTER_KUBECONFIG_NAME="$(basename "${MEMBER_CLUSTER_KUBECONFIG}")"
# --context & --minify will generate minified kubeconfig file with required context
# --flatten will embed certificate
kubectl config view --kubeconfig "${MEMBER_CLUSTER_KUBECONFIG}" \
	--context "${MEMBER_CLUSTER_NAME}" --minify  --flatten \
	> "${TEMP_PATH}/${MEMBER_CLUSTER_KUBECONFIG_NAME}"

# check whether the kubeconfig secret has been created before
if ! kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" get secrets -n karmada-system | grep "${MEMBER_CLUSTER_NAME}-kubeconfig"; then
  # create secret
  kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" \
  	create secret generic "${MEMBER_CLUSTER_NAME}-kubeconfig" \
  	"--from-file=${MEMBER_CLUSTER_NAME}-kubeconfig=${TEMP_PATH}/${MEMBER_CLUSTER_KUBECONFIG_NAME}" \
  	-n "karmada-system"
fi
rm -rf "${TEMP_PATH}"

# deploy scheduler estimator
TEMP_PATH=$(mktemp -d)
cp "${REPO_ROOT}"/artifacts/deploy/karmada-scheduler-estimator.yaml "${TEMP_PATH}"/karmada-scheduler-estimator.yaml
sed -i'' -e "s/{{member_cluster_name}}/${MEMBER_CLUSTER_NAME}/g" "${TEMP_PATH}"/karmada-scheduler-estimator.yaml
echo -e "Apply dynamic rendered deployment in ${TEMP_PATH}/karmada-scheduler-estimator.yaml\n"
kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" apply -f "${TEMP_PATH}"/karmada-scheduler-estimator.yaml
rm -rf "${TEMP_PATH}"

function print_success() {
  echo -e "\nKarmada scheduler estimator of cluster ${MEMBER_CLUSTER_NAME} has been deployed."
  echo "Note: To enable scheduler estimator, please deploy other scheduler estimators of all clusters."
  echo "      After that, specify the option '--enable-scheduler-estimator=true' of karmada-scheduler."
}

print_success
