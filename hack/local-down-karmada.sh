#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

# This script depends on utils in: ${REPO_ROOT}/hack/util.sh

function usage() {
    echo "Usage:"
    echo "    hack/local-down-karmada.sh [-k] [-h]"
    echo "Args:"
    echo "    k: keep the local images"
    echo "    h: print help information"
}

keep_images="false"
while getopts 'kh' OPT; do
    case $OPT in
        k) keep_images="true";;
        h)
          usage
          exit 0
          ;;
        ?)
          usage
          exit 1
          ;;
    esac
done

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}

HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}

KIND_LOG_FILE=${KIND_LOG_FILE:-"/tmp/karmada"}

#step1 remove kind clusters and kubeconfig

util::delete_cluster_created_by_script "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_1_NAME}" "${MEMBER_CLUSTER_2_NAME}" "${PULL_MODE_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${MEMBER_CLUSTER_KUBECONFIG}" "${KIND_LOG_FILE}"

#step2. remove docker images
echo -e "\nStart removing images"
version="latest"
registry="docker.io/karmada"
images=(
"${registry}/karmada-controller-manager:${version}"
"${registry}/karmada-scheduler:${version}"
"${registry}/karmada-descheduler:${version}"
"${registry}/karmada-webhook:${version}"
"${registry}/karmada-scheduler-estimator:${version}"
"${registry}/karmada-aggregated-apiserver:${version}"
"${registry}/karmada-search:${version}"
)
if [[ "${keep_images}" == "false" ]] ; then
  for ((i=0;i<${#images[*]};i++)); do
    docker rmi ${images[i]} || true
  done
  echo "Remove images successfully."
else
  echo "Skip removing images as required."
fi

echo -e "\nLocal Karmada is removed successfully."
