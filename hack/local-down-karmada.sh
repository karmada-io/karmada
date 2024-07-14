#!/usr/bin/env bash
# Copyright 2022 The Karmada Authors.
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

#step1 remove kind clusters
echo -e "\nStart removing kind clusters"
kind delete cluster --name "${HOST_CLUSTER_NAME:-"karmada-host"}"
kind delete cluster --name "${MEMBER_CLUSTER_1_NAME:-"member1"}"
kind delete cluster --name "${MEMBER_CLUSTER_2_NAME:-"member2"}"
kind delete cluster --name "${PULL_MODE_CLUSTER_NAME:-"member3"}"
echo "Remove kind clusters successfully."

#step2. remove kubeconfig
echo -e "\nStart removing kubeconfig"
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
if [ -f "${MAIN_KUBECONFIG}" ] ; then
    rm ${MAIN_KUBECONFIG}
    echo "Remove kubeconfig ${MAIN_KUBECONFIG} successfully."
fi
if [ -f "${MEMBER_CLUSTER_KUBECONFIG}" ] ; then
    rm ${MEMBER_CLUSTER_KUBECONFIG}
    echo "Remove kubeconfig ${MEMBER_CLUSTER_KUBECONFIG} successfully."
fi
echo "Remove kubeconfig successfully."

#step3. remove docker images
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
