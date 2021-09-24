#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

# This script starts a local karmada control plane based on current codebase and with a certain number of clusters joined.
# Parameters: [HOST_IPADDRESS](optional) if you want to export clusters' API server port to specific IP address
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh
# 1. used by developer to setup develop environment quickly.
# 2. used by e2e testing to setup test environment automatically.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# variable define
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
KARMADA_APISERVER_CLUSTER_NAME=${KARMADA_APISERVER_CLUSTER_NAME:-"karmada-apiserver"}
MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}
HOST_IPADDRESS=${1:-}

CLUSTER_VERSION=${CLUSTER_VERSION:-"kindest/node:v1.19.1"}
KIND_LOG_FILE=${KIND_LOG_FILE:-"/tmp/karmada"}

#step0: prepare
# proxy setting in China mainland

if [[ -n ${CHINA_MAINLAND:-} ]]; then
export GOPROXY=https://goproxy.cn # set go proxy
# set mirror registry of k8s.gcr.io
sed -i'' -e "s#k8s.gcr.io#registry.aliyuncs.com/google_containers#g" artifacts/deploy/karmada-etcd.yaml
sed -i'' -e "s#k8s.gcr.io#registry.aliyuncs.com/google_containers#g" artifacts/deploy/karmada-apiserver.yaml
sed -i'' -e "s#k8s.gcr.io#registry.aliyuncs.com/google_containers#g" artifacts/deploy/kube-controller-manager.yaml
fi

# Make sure go exists
util::cmd_must_exist "go"
# install kind and kubectl
util::install_tools sigs.k8s.io/kind v0.11.1
# get arch name and os name in bootstrap
BS_ARCH=$(go env GOARCH)
BS_OS=$(go env GOOS)
# check arch and os name before installing
util::install_environment_check "${BS_ARCH}" "${BS_OS}"
# we choose v1.18.0, because in kubectl after versions 1.18 exist a bug which will give wrong output when using jsonpath.
# bug details: https://github.com/kubernetes/kubernetes/pull/98057
util::install_kubectl "v1.18.0" "${BS_ARCH}" "${BS_OS}"

#step1. create host cluster and member clusters in parallel
# host IP address: script parameter ahead of macOS IP
if [[ -z "${HOST_IPADDRESS}" ]]; then
  util::get_macos_ipaddress # Adapt for macOS
  HOST_IPADDRESS=${MAC_NIC_IPADDRESS:-}
fi
#prepare for kindClusterConfig
TEMP_PATH=$(mktemp -d)
echo -e "Preparing kindClusterConfig in path: ${TEMP_PATH}"
cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/member1.yaml "${TEMP_PATH}"/member1.yaml
cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/member2.yaml "${TEMP_PATH}"/member2.yaml
if [[ -n "${HOST_IPADDRESS}" ]]; then # If bind the port of clusters(karmada-host, member1 and member2) to the host IP
  cp -rf "${REPO_ROOT}"/artifacts/kindClusterConfig/karmada-host.yaml "${TEMP_PATH}"/karmada-host.yaml
  sed -i'' -e "s/{{host_ipaddress}}/${HOST_IPADDRESS}/g" "${TEMP_PATH}"/karmada-host.yaml
  sed -i'' -e '/networking:/a\'$'\n''  apiServerAddress: '"${HOST_IPADDRESS}"''$'\n' "${TEMP_PATH}"/member1.yaml
  sed -i'' -e '/networking:/a\'$'\n''  apiServerAddress: '"${HOST_IPADDRESS}"''$'\n' "${TEMP_PATH}"/member2.yaml
  util::create_cluster "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}"/karmada-host.yaml
else
  util::create_cluster "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}"
fi
util::create_cluster "${MEMBER_CLUSTER_1_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}"/member1.yaml
util::create_cluster "${MEMBER_CLUSTER_2_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}" "${TEMP_PATH}"/member2.yaml
util::create_cluster "${PULL_MODE_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${CLUSTER_VERSION}" "${KIND_LOG_FILE}"

#step2. make images and get karmadactl
export VERSION="latest"
export REGISTRY="swr.ap-southeast-1.myhuaweicloud.com/karmada"
make images GOOS="linux" --directory="${REPO_ROOT}"

GO111MODULE=on go install "github.com/karmada-io/karmada/cmd/karmadactl"
GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
KARMADACTL_BIN="${GOPATH}/bin/karmadactl"

#step3. wait until the host cluster ready
echo "Waiting for the host clusters to be ready..."
util::check_clusters_ready "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

#step4. load components images to kind cluster
export VERSION="latest"
export REGISTRY="swr.ap-southeast-1.myhuaweicloud.com/karmada"
kind load docker-image "${REGISTRY}/karmada-controller-manager:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-scheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-webhook:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-scheduler-estimator:${VERSION}" --name="${HOST_CLUSTER_NAME}"

#step5. install karmada control plane components
"${REPO_ROOT}"/hack/deploy-karmada.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

#step6. wait until the member cluster ready and join member clusters
util::check_clusters_ready "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_1_NAME}"
util::check_clusters_ready "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_2_NAME}"

# connecting networks between member1 and member2 clusters
echo "connecting cluster networks..."
util::add_routes "${MEMBER_CLUSTER_1_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_2_NAME}"
util::add_routes "${MEMBER_CLUSTER_2_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_1_NAME}"
echo "cluster networks connected"

#join push mode member clusters
export KUBECONFIG="${MAIN_KUBECONFIG}"
kubectl config use-context "${KARMADA_APISERVER_CLUSTER_NAME}"
${KARMADACTL_BIN} join member1 --cluster-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}"
${KARMADACTL_BIN} join member2 --cluster-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}"

# wait until the pull mode cluster ready
util::check_clusters_ready "${MEMBER_CLUSTER_KUBECONFIG}" "${PULL_MODE_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-agent:${VERSION}" --name="${PULL_MODE_CLUSTER_NAME}"

#step7. deploy karmada agent in pull mode member clusters
"${REPO_ROOT}"/hack/deploy-karmada-agent.sh "${MAIN_KUBECONFIG}" "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${PULL_MODE_CLUSTER_NAME}"

function print_success() {
  echo -e "---\n"
  echo "Local Karmada is running."
  echo -e "\nTo start using your karmada, run:"
  echo -e "  export KUBECONFIG=${MAIN_KUBECONFIG}"
  echo "Please use 'kubectl config use-context karmada-host/karmada-apiserver' to switch the host and control plane cluster."
  echo -e "\nTo manage your member clusters, run:"
  echo -e "  export KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG}"
    echo "Please use 'kubectl config use-context member1/member2/member3' to switch to the different member cluster."
}

print_success
