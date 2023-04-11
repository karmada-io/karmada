#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

# This script starts a local karmada control plane with karmadactl and with a certain number of clusters joined.
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh
# 1. used by developer to setup develop environment quickly.
# 2. used by e2e testing to setup test environment automatically.

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# variable define
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
CLUSTER_VERSION=${CLUSTER_VERSION:-"kindest/node:v1.26.0"}
BUILD_PATH=${BUILD_PATH:-"_output/bin/linux/amd64"}

# prepare the newest crds
echo "Prepare the newest crds"
cd  charts/karmada/
cp -r _crds crds
tar -zcvf ../../crds.tar.gz crds
cd -

# make karmadactl binary
make karmadactl

# create host/member1/member2 cluster
echo "Start create clusters..."
hack/create-cluster.sh ${HOST_CLUSTER_NAME} ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config > /dev/null 2>&1 &
hack/create-cluster.sh ${MEMBER_CLUSTER_1_NAME} ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config > /dev/null 2>&1 &
hack/create-cluster.sh ${MEMBER_CLUSTER_2_NAME} ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config > /dev/null 2>&1 &

# wait cluster ready
echo "Wait clusters ready..."
util::wait_file_exist ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config 300
util::wait_context_exist ${HOST_CLUSTER_NAME} ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config 300
kubectl wait --for=condition=Ready nodes --all --timeout=800s --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config
util::wait_nodes_taint_disappear 800 ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config

util::wait_file_exist ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config 300
util::wait_context_exist "kind-${MEMBER_CLUSTER_1_NAME}" ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config 300
kubectl wait --for=condition=Ready nodes --all --timeout=800s --kubeconfig=${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config
util::wait_nodes_taint_disappear 800 ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config

util::wait_file_exist ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config 300
util::wait_context_exist "kind-${MEMBER_CLUSTER_2_NAME}" ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config 300
kubectl wait --for=condition=Ready nodes --all --timeout=800s --kubeconfig=${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config
util::wait_nodes_taint_disappear 800 ${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config

# init Karmada control plane
echo "Start init karmada control plane..."
${BUILD_PATH}/karmadactl init --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config \
    --karmada-data=${HOME}/karmada \
    --karmada-pki=${HOME}/karmada/pki \
    --crds=./crds.tar.gz

# join cluster
echo "Join member clusters..."
${BUILD_PATH}/karmadactl --kubeconfig ${HOME}/karmada/karmada-apiserver.config  join ${MEMBER_CLUSTER_1_NAME} --cluster-kubeconfig=${KUBECONFIG_PATH}/${MEMBER_CLUSTER_1_NAME}.config
${BUILD_PATH}/karmadactl --kubeconfig ${HOME}/karmada/karmada-apiserver.config  join ${MEMBER_CLUSTER_2_NAME} --cluster-kubeconfig=${KUBECONFIG_PATH}/${MEMBER_CLUSTER_2_NAME}.config
kubectl wait --for=condition=Ready clusters --all --timeout=800s  --kubeconfig=${HOME}/karmada/karmada-apiserver.config
