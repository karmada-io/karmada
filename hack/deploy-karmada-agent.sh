#!/bin/bash

set -o errexit
set -o nounset

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
source "${REPO_ROOT}"/hack/util.sh

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
if ! kubectl config use-context "${2}" --kubeconfig="${KARMADA_APISERVER_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to use context: '${2}' not in ${KARMADA_APISERVER_KUBECONFIG}. \n"
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

# install agent to member cluster
if [ -n "${KUBECONFIG+x}" ];then
  CURR_KUBECONFIG=$KUBECONFIG # backup current kubeconfig
fi
export KUBECONFIG="${MEMBER_CLUSTER_KUBECONFIG}" # switch to member cluster
kubectl config use-context "${MEMBER_CLUSTER_NAME}"

AGENT_IMAGE_PULL_POLICY=${IMAGE_PULL_POLICY:-IfNotPresent}

INSECURE_SKIP_TLS_VERIFY="true"
KARMADA_CRT=$(cat "${CERT_DIR}/karmada.crt")
KARMADA_KEY=$(cat "${CERT_DIR}/karmada.key")
KARMADA_APISERVER_ADDRESS=$(cat "${KARMADA_APISERVER_KUBECONFIG}" | grep server | awk '{print $2}' | head -n 1)

# extract api endpoint of member cluster
MEMBER_CLUSTER=$(kubectl config view -o jsonpath='{.contexts[?(@.name == "'${MEMBER_CLUSTER_NAME}'")].context.cluster}')
MEMBER_CLUSTER_API_ENDPOINT=$(kubectl config view -o jsonpath='{.clusters[?(@.name == "'${MEMBER_CLUSTER}'")].cluster.server}')

helm install karmada-agent -n "${KARMADA_SYSTEM_NAMESPACE}" --create-namespace --dependency-update "${REPO_ROOT}"/charts/karmada/ --set installMode=agent,agent.clusterName=${MEMBER_CLUSTER_NAME},agent.karmadaContext=${KARMADA_APISERVER_CONTEXT_NAME},agent.image.pullPolicy=${AGENT_IMAGE_PULL_POLICY},agent.clusterAPIEndpoint=${MEMBER_CLUSTER_API_ENDPOINT},agent.kubeconfig.insecureSkipTlsVerify=${INSECURE_SKIP_TLS_VERIFY},agent.kubeconfig.server=${KARMADA_APISERVER_ADDRESS},agent.kubeconfig.crt="${KARMADA_CRT}",agent.kubeconfig.key="${KARMADA_KEY}"

# Wait for karmada-etcd to come up before launching the rest of the components.
util::wait_pod_ready "${AGENT_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# recover the kubeconfig before installing agent if necessary
if [ -n "${CURR_KUBECONFIG+x}" ];then
  export KUBECONFIG="${CURR_KUBECONFIG}"
else
  unset KUBECONFIG
fi
