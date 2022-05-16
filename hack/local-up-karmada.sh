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

CLUSTER_VERSION=${CLUSTER_VERSION:-"kindest/node:v1.23.4"}
KIND_LOG_FILE=${KIND_LOG_FILE:-"/tmp/karmada"}

#step0: prepare
# proxy setting in China mainland

if [[ -n ${CHINA_MAINLAND:-} ]]; then
export GOPROXY=https://goproxy.cn,direct # set domestic go proxy
# set mirror registry of k8s.gcr.io
registry_files=( # Yaml files that contain image host 'k8s.gcr.io' need to be replaced
  "artifacts/deploy/karmada-etcd.yaml"
  "artifacts/deploy/karmada-apiserver.yaml"
  "artifacts/deploy/kube-controller-manager.yaml"
)
for registry_file in "${registry_files[@]}"; do
  sed -i'' -e "s#k8s.gcr.io#registry.aliyuncs.com/google_containers#g" ${REPO_ROOT}/${registry_file}
done
# set mirror registry in the dockerfile of components of karmada
dockerfile_list=( # Dockerfile files need to be replaced
  "cluster/images/Dockerfile"
  "cluster/images/buildx.Dockerfile"
)
for dockerfile in "${dockerfile_list[@]}"; do
  grep 'mirrors.ustc.edu.cn' ${REPO_ROOT}/${dockerfile} > /dev/null || sed -i'' -e "s#FROM alpine:3.15.1#FROM alpine:3.15.1\nRUN echo -e http://mirrors.ustc.edu.cn/alpine/v3.15/main/ > /etc/apk/repositories#" ${REPO_ROOT}/${dockerfile}
done
fi

# Make sure go exists and the go version is a viable version.
util::cmd_must_exist "go"
util::verify_go_version

# Make sure docker exists
util::cmd_must_exist "docker"

# install kind and kubectl
kind_version=v0.12.0
echo -n "Preparing: 'kind' existence check - "
if util::cmd_exist kind; then
  echo "passed"
else
  echo "not pass"
  util::install_tools "sigs.k8s.io/kind" $kind_version
fi
# get arch name and os name in bootstrap
BS_ARCH=$(go env GOARCH)
BS_OS=$(go env GOOS)
# check arch and os name before installing
util::install_environment_check "${BS_ARCH}" "${BS_OS}"
echo -n "Preparing: 'kubectl' existence check - "
if util::cmd_exist kubectl; then
  echo "passed"
else
  echo "not pass"
  util::install_kubectl "" "${BS_ARCH}" "${BS_OS}"
fi
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
  sed -i'' -e 's/networking:/&\'$'\n''  apiServerAddress: "'${HOST_IPADDRESS}'"/' "${TEMP_PATH}"/member1.yaml
  sed -i'' -e 's/networking:/&\'$'\n''  apiServerAddress: "'${HOST_IPADDRESS}'"/' "${TEMP_PATH}"/member2.yaml
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
kind load docker-image "${REGISTRY}/karmada-descheduler:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-webhook:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-scheduler-estimator:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-aggregated-apiserver:${VERSION}" --name="${HOST_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-search:${VERSION}" --name="${HOST_CLUSTER_NAME}"

#step5. install karmada control plane components
"${REPO_ROOT}"/hack/deploy-karmada.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

#step6. wait until the member cluster ready and join member clusters
util::check_clusters_ready "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_1_NAME}"
util::check_clusters_ready "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_2_NAME}"

# connecting networks between karmada-host, member1 and member2 clusters
echo "connecting cluster networks..."
util::add_routes "${MEMBER_CLUSTER_1_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_2_NAME}"
util::add_routes "${MEMBER_CLUSTER_2_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_1_NAME}"

util::add_routes "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_1_NAME}"
util::add_routes "${MEMBER_CLUSTER_1_NAME}" "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"

util::add_routes "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_2_NAME}"
util::add_routes "${MEMBER_CLUSTER_2_NAME}" "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}"
echo "cluster networks connected"

#join push mode member clusters
export KUBECONFIG="${MAIN_KUBECONFIG}"
kubectl config use-context "${KARMADA_APISERVER_CLUSTER_NAME}"
${KARMADACTL_BIN} join member1 --cluster-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}"
"${REPO_ROOT}"/hack/deploy-scheduler-estimator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_1_NAME}"
${KARMADACTL_BIN} join member2 --cluster-kubeconfig="${MEMBER_CLUSTER_KUBECONFIG}"
"${REPO_ROOT}"/hack/deploy-scheduler-estimator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${MEMBER_CLUSTER_2_NAME}"

# wait until the pull mode cluster ready
util::check_clusters_ready "${MEMBER_CLUSTER_KUBECONFIG}" "${PULL_MODE_CLUSTER_NAME}"
kind load docker-image "${REGISTRY}/karmada-agent:${VERSION}" --name="${PULL_MODE_CLUSTER_NAME}"

#step7. deploy karmada agent in pull mode member clusters
"${REPO_ROOT}"/hack/deploy-agent-and-estimator.sh "${MAIN_KUBECONFIG}" "${HOST_CLUSTER_NAME}" "${MAIN_KUBECONFIG}" "${KARMADA_APISERVER_CLUSTER_NAME}" "${MEMBER_CLUSTER_KUBECONFIG}" "${PULL_MODE_CLUSTER_NAME}"

function print_success() {
  echo -e "$KARMADA_GREETING"
  echo "Local Karmada is running."
  echo -e "\nTo start using your karmada, run:"
  echo -e "  export KUBECONFIG=${MAIN_KUBECONFIG}"
  echo "Please use 'kubectl config use-context karmada-host/karmada-apiserver' to switch the host and control plane cluster."
  echo -e "\nTo manage your member clusters, run:"
  echo -e "  export KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG}"
  echo "Please use 'kubectl config use-context member1/member2/member3' to switch to the different member cluster."
}

print_success
