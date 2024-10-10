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
  echo "Usage: hack/deploy-scheduler-estimator.sh <MEMBER_CLUSTER_NAME> <HOST_CLUSTER_KUBECONFIG> <HOST_CLUSTER_CONTEXT> <MEMBER_CLUSTER_KUBECONFIG> <MEMBER_CLUSTER_CONTEXT>"
  echo "Example: hack/deploy-scheduler-estimator.sh member1 ~/.kube/karmada.config karmada-host ~/.kube/members.config member1"
  echo "Example: SCHEDULER_ESTIMATOR_NAMESPACE=test GRPC_AUTH_SECRET_NAME=karmada-demo-cert hack/deploy-scheduler-estimator.sh member1 ~/.kube/karmada.config karmada-host ~/.kube/members.config member1"
}

if [[ $# -ne 5 ]]; then
  usage
  exit 1
fi

# check kube config file existence
if [[ ! -f "${2}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${2}', not existed.\n"
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
MEMBER_CLUSTER_KUBECONFIG=$4

# check context existence
if ! kubectl config get-contexts "${5}" --kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${5}' not in ${MEMBER_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi
MEMBER_CLUSTER_CONTEXT=$5

MEMBER_CLUSTER_NAME=$1
SCHEDULER_ESTIMATOR_NAMESPACE=${SCHEDULER_ESTIMATOR_NAMESPACE:-"karmada-system"}
GRPC_AUTH_SECRET_NAME=${GRPC_AUTH_SECRET_NAME:-"karmada-cert-secret"}

TEMP_PATH="$(mktemp -d)"
MEMBER_CLUSTER_KUBECONFIG_NAME="$(basename "${MEMBER_CLUSTER_KUBECONFIG}")"
# --context & --minify will generate minified kubeconfig file with required context
# --flatten will embed certificate
kubectl config view --kubeconfig "${MEMBER_CLUSTER_KUBECONFIG}" \
	--context "${MEMBER_CLUSTER_CONTEXT}" --minify  --flatten \
	> "${TEMP_PATH}/${MEMBER_CLUSTER_KUBECONFIG_NAME}"

# check whether the kubeconfig secret has been created before
if ! kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${HOST_CLUSTER_CONTEXT}" get secrets -n ${SCHEDULER_ESTIMATOR_NAMESPACE} | grep "${MEMBER_CLUSTER_NAME}-kubeconfig"; then
  # create secret
  kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${HOST_CLUSTER_CONTEXT}" \
  	create secret generic "${MEMBER_CLUSTER_NAME}-kubeconfig" \
  	"--from-file=${MEMBER_CLUSTER_NAME}-kubeconfig=${TEMP_PATH}/${MEMBER_CLUSTER_KUBECONFIG_NAME}" \
  	-n ${SCHEDULER_ESTIMATOR_NAMESPACE}
fi
rm -rf "${TEMP_PATH}"

# deploy scheduler estimator
TEMP_PATH=$(mktemp -d)
cp "${REPO_ROOT}"/artifacts/deploy/karmada-scheduler-estimator.yaml "${TEMP_PATH}"/karmada-scheduler-estimator.yaml
sed -i'' -e "s/{{grpc_auth_secret_name}}/${GRPC_AUTH_SECRET_NAME}/g" "${TEMP_PATH}"/karmada-scheduler-estimator.yaml
sed -i'' -e "s/{{namespace}}/${SCHEDULER_ESTIMATOR_NAMESPACE}/g" "${TEMP_PATH}"/karmada-scheduler-estimator.yaml
sed -i'' -e "s/{{member_cluster_name}}/${MEMBER_CLUSTER_NAME}/g" "${TEMP_PATH}"/karmada-scheduler-estimator.yaml
echo -e "Apply dynamic rendered deployment in ${TEMP_PATH}/karmada-scheduler-estimator.yaml\n"
kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${HOST_CLUSTER_CONTEXT}" apply -f "${TEMP_PATH}"/karmada-scheduler-estimator.yaml
rm -rf "${TEMP_PATH}"

function print_success() {
  echo -e "\nKarmada scheduler estimator of cluster ${MEMBER_CLUSTER_NAME} has been deployed."
  echo "Note: To enable scheduler estimator, please deploy other scheduler estimators of all clusters."
  echo "      After that, specify the option '--enable-scheduler-estimator=true' of karmada-scheduler."
}

print_success
