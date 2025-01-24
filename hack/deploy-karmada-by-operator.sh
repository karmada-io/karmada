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

# This script deploy karmada instance to any cluster you want via karmada-operator.
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source ${REPO_ROOT}/hack/util.sh

KARMADA_INSTANCE_NAME=${KARMADA_INSTANCE_NAME:-"karmada-demo"}
KARMADA_INSTANCE_NAMESPACE=${KARMADA_INSTANCE_NAMESPACE:-"test"}

CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
mkdir -p "${CERT_DIR}" &>/dev/null ||  mkdir -p "${CERT_DIR}"
rm -f "${CERT_DIR}/*" &>/dev/null ||  rm -f "${CERT_DIR}/*"

function usage() {
  echo "This script deploys karmada instance to a given cluster via karmada-operator."
  echo "Note: This script is an internal script and is not intended used by end-users."
  echo "Usage: hack/deploy-karmada-by-operator.sh <KUBECONFIG> <CONTEXT_NAME> <KARMADA_CONTEXT_NAME> <KARMADA_IMAGE_TAG> <ADDON_NEEDED> <CRD_DOWNLOAD_URL>"
  echo "Example: hack/deploy-karmada-by-operator.sh ~/.kube/members.config member1 karmada-apiserver v1.11.0 true https://github.com/karmada-io/karmada/releases/download/v1.11.0/crds.tar.gz"
  echo -e "Parameters:\n\tKUBECONFIG\t\tYour cluster's kubeconfig that you want to install to"
  echo -e "\tCONTEXT_NAME\t\tThe name of context in 'kubeconfig'"
  echo -e "\tKARMADA_CONTEXT_NAME\t\tThe context name of karmada instance, and different Karmada instances must have unique contexts to avoid being overwritten.'"
  echo -e "\tKARMADA_IMAGE_TAG\t\tThe tag of image'"
  echo -e "\tADDON_NEEDED\t\tWhether you need to install addons(KarmadaSearch&karmadaDescheduler), optional, defaults to false."
  echo -e "\tCRD_DOWNLOAD_URL\t\tThe download url for CRDs, optional.'"
}

if [[ $# -le 4 ]]; then
  usage
  exit 1
fi

# check config file existence
HOST_CLUSTER_KUBECONFIG=$1
if [[ ! -f "${HOST_CLUSTER_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${HOST_CLUSTER_KUBECONFIG}', not existed.\n"
  usage
  exit 1
fi

# check context existence
CONTEXT_NAME=$2
if ! kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" config get-contexts "${CONTEXT_NAME}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${CONTEXT_NAME}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi

# check for duplicate karmada context name.
KARMADA_CONTEXT_NAME=$3
if kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" config get-contexts "${KARMADA_CONTEXT_NAME}" > /dev/null 2>&1;
then
  echo -e "ERROR: context: '${KARMADA_CONTEXT_NAME}' already exists in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi

TEMP_PATH_BOOTSTRAP=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH_BOOTSTRAP}; }' EXIT
cp -rf "${REPO_ROOT}"/operator/config/samples/karmada-sample.yaml "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml

if kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${CONTEXT_NAME}" get namespace ${KARMADA_INSTANCE_NAMESPACE} > /dev/null 2>&1; then
  echo "Namespace '${KARMADA_INSTANCE_NAMESPACE}' already exists."
else
  echo "Namespace '${KARMADA_INSTANCE_NAMESPACE}' does not exist. Creating now..."
  kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${CONTEXT_NAME}" create ns ${KARMADA_INSTANCE_NAMESPACE}
fi

# modify `karmada-sample.yaml` based on custom configuration.
ADDON_NEEDED=${5:-false}
# if choosing install addons, append karmadaSearch and karmadaDescheduler to 'karmada-sample.yaml'
if [ ${ADDON_NEEDED} ]; then
    echo -e '    karmadaDescheduler:\n      imageRepository: docker.io/karmada/karmada-descheduler\n      imageTag: {{image_tag}}\n      replicas: 1' >> "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml
    echo -e '    karmadaSearch:\n      imageRepository: docker.io/karmada/karmada-search\n      imageTag: {{image_tag}}\n      replicas: 1' >> "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml
fi

IMAGE_TAG=$4
sed -i'' -e "s/{{image_tag}}/${IMAGE_TAG}/g" "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml
sed -i'' -e "s/{{karmada_instance_name}}/${KARMADA_INSTANCE_NAME}/g" "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml
sed -i'' -e "s/{{karmada_instance_namespace}}/${KARMADA_INSTANCE_NAMESPACE}/g" "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml

CRD_DOWNLOAD_URL=${6:-""}
if [[ -z ${CRD_DOWNLOAD_URL} ]]; then
	sed -i'' -e "s/{{crd_tarball}}/""/g" "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml
else
	CRD_TAR_BALL="\n    httpSource:\n      url: ${CRD_DOWNLOAD_URL}"
  awk -v pattern="{{crd_tarball}}" -v replacement="${CRD_TAR_BALL}" '{ gsub(pattern, replacement); print }' "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml > "${TEMP_PATH_BOOTSTRAP}"/temp && mv "${TEMP_PATH_BOOTSTRAP}"/temp "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml
fi

# create and wait for karmada instance to be ready
kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${CONTEXT_NAME}" apply -f "${TEMP_PATH_BOOTSTRAP}"/karmada-sample-tmp.yaml
kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${CONTEXT_NAME}" wait --for=condition=Ready --timeout=1000s karmada ${KARMADA_INSTANCE_NAME} -n ${KARMADA_INSTANCE_NAMESPACE}

# generate kubeconfig for karmada instance
kubectl --kubeconfig="${HOST_CLUSTER_KUBECONFIG}" --context="${CONTEXT_NAME}" get secret -n ${KARMADA_INSTANCE_NAMESPACE} ${KARMADA_INSTANCE_NAME}-admin-config -o jsonpath='{.data.karmada\.config}' | base64 -d > ~/.kube/${KARMADA_INSTANCE_NAME}-${KARMADA_INSTANCE_NAMESPACE}-tmp-apiserver.config
cat ~/.kube/${KARMADA_INSTANCE_NAME}-${KARMADA_INSTANCE_NAMESPACE}-tmp-apiserver.config| grep "certificate-authority-data"| awk '{print $2}'| base64 -d  > ${CERT_DIR}/ca.crt
cat ~/.kube/${KARMADA_INSTANCE_NAME}-${KARMADA_INSTANCE_NAMESPACE}-tmp-apiserver.config| grep "client-certificate-data"| awk '{print $2}'| base64 -d  > ${CERT_DIR}/karmada.crt
cat ~/.kube/${KARMADA_INSTANCE_NAME}-${KARMADA_INSTANCE_NAMESPACE}-tmp-apiserver.config| grep "client-key-data"| awk '{print $2}'| base64 -d  > ${CERT_DIR}/karmada.key
KARMADA_APISERVER=$(cat ~/.kube/${KARMADA_INSTANCE_NAME}-${KARMADA_INSTANCE_NAMESPACE}-tmp-apiserver.config| grep "server:"| awk '{print $2}')

# write karmada api server config to kubeconfig file
util::append_client_kubeconfig "${HOST_CLUSTER_KUBECONFIG}" "${CERT_DIR}/ca.crt" "${CERT_DIR}/karmada.crt" "${CERT_DIR}/karmada.key" "${KARMADA_APISERVER}" ${KARMADA_CONTEXT_NAME}
rm ~/.kube/${KARMADA_INSTANCE_NAME}-${KARMADA_INSTANCE_NAMESPACE}-tmp-apiserver.config
