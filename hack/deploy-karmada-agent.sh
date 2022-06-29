#!/bin/bash

set -o errexit
set -o nounset

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

function usage() {
  echo "This script will deploy karmada agent to a cluster."
  echo "Usage: hack/deploy-karmada-agent.sh <KARMADA_APISERVER_KUBECONFIG> <KARMADA_APISERVER_CONTEXT_NAME> <MEMBER_CLUSTER_KUBECONFIG> <MEMBER_CLUSTER_CONTEXT_NAME>"
  echo "Example: hack/deploy-karmada-agent.sh ~/.kube/karmada.config karmada-apiserver ~/.kube/members.config member1"
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
KARMADA_APISERVER_KUBECONFIG=$1

# check context existence
if ! kubectl config get-contexts "${2}" --kubeconfig="${KARMADA_APISERVER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${2}' not in ${KARMADA_APISERVER_KUBECONFIG}. \n"
  usage
  exit 1
fi
KARMADA_APISERVER_CONTEXT_NAME=$2

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

source "${REPO_ROOT}"/hack/util.sh

# install agent to member cluster
if [ -n "${KUBECONFIG+x}" ];then
  CURR_KUBECONFIG=$KUBECONFIG # backup current kubeconfig
fi
export KUBECONFIG="${MEMBER_CLUSTER_KUBECONFIG}" # switch to member cluster

AGENT_IMAGE_PULL_POLICY=${IMAGE_PULL_POLICY:-IfNotPresent}

# create namespace for karmada agent
kubectl --context="${MEMBER_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/agent/namespace.yaml"

# create service account, cluster role for karmada agent
kubectl --context="${MEMBER_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/agent/serviceaccount.yaml"
kubectl --context="${MEMBER_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/agent/clusterrole.yaml"
kubectl --context="${MEMBER_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/agent/clusterrolebinding.yaml"

# create secret
kubectl --context="${MEMBER_CLUSTER_NAME}" create secret generic karmada-kubeconfig --from-file=karmada-kubeconfig="${KARMADA_APISERVER_KUBECONFIG}" -n "${KARMADA_SYSTEM_NAMESPACE}"

# extract api endpoint of member cluster
MEMBER_CLUSTER=$(kubectl config view -o jsonpath='{.contexts[?(@.name == "'${MEMBER_CLUSTER_NAME}'")].context.cluster}')
MEMBER_CLUSTER_API_ENDPOINT=$(kubectl config view -o jsonpath='{.clusters[?(@.name == "'${MEMBER_CLUSTER}'")].cluster.server}')

# deploy karmada agent
TEMP_PATH=$(mktemp -d)
cp "${REPO_ROOT}"/artifacts/agent/karmada-agent.yaml "${TEMP_PATH}"/karmada-agent.yaml
sed -i'' -e "s/{{karmada_context}}/${KARMADA_APISERVER_CONTEXT_NAME}/g" "${TEMP_PATH}"/karmada-agent.yaml
sed -i'' -e "s/{{member_cluster_name}}/${MEMBER_CLUSTER_NAME}/g" "${TEMP_PATH}"/karmada-agent.yaml
sed -i'' -e "s/{{image_pull_policy}}/${AGENT_IMAGE_PULL_POLICY}/g" "${TEMP_PATH}"/karmada-agent.yaml
sed -i'' -e "s|{{member_cluster_api_endpoint}}|${MEMBER_CLUSTER_API_ENDPOINT}|g" "${TEMP_PATH}"/karmada-agent.yaml
echo -e "Apply dynamic rendered deployment in ${TEMP_PATH}/karmada-agent.yaml.\n"
kubectl --context="${MEMBER_CLUSTER_NAME}" apply -f "${TEMP_PATH}"/karmada-agent.yaml

# Wait for karmada-etcd to come up before launching the rest of the components.
util::wait_pod_ready "${MEMBER_CLUSTER_NAME}" "${AGENT_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# recover the kubeconfig before installing agent if necessary
if [ -n "${CURR_KUBECONFIG+x}" ];then
  export KUBECONFIG="${CURR_KUBECONFIG}"
else
  unset KUBECONFIG
fi
