#!/bin/bash

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

kubectl --kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" config use-context "${MEMBER_CLUSTER_NAME}"

# check whether the kubeconfig secret has been created before
if ! kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" get secrets -n karmada-system | grep "${MEMBER_CLUSTER_NAME}-kubeconfig"; then
  # create secret
  kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${HOST_CLUSTER_NAME}" create secret generic ${MEMBER_CLUSTER_NAME}-kubeconfig --from-file=${MEMBER_CLUSTER_NAME}-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}" -n "karmada-system"
fi

# deploy karmada agent
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
