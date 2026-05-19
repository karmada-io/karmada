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
CLUSTER_VERSION=${CLUSTER_VERSION:-"kindest/node:v1.27.3"}
BUILD_PATH=${BUILD_PATH:-"_output/bin/linux/amd64"}

# prepare the newest crds
# echo "Prepare the newest crds"
# cd  charts/karmada/
# cp -r _crds crds
# tar -zcvf ../../crds.tar.gz crds
# cd -

# build karmada operator
make image-karmada-operator

# create host/member1/member2 cluster
echo "Start create clusters..."
hack/create-cluster.sh ${HOST_CLUSTER_NAME} ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config

# wait cluster ready
echo "Wait clusters ready..."
util::wait_file_exist ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config 300
util::wait_context_exist ${HOST_CLUSTER_NAME} ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config 300
kubectl wait --for=condition=Ready nodes --all --timeout=800s --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config
util::wait_nodes_taint_disappear 800 ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config

kind get clusters
IMGTAG=`git describe --tags --dirty`
kubectl apply -f operator/config/crds --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config
docker tag docker.io/karmada/karmada-operator:$IMGTAG docker.io/karmada/karmada-operator:latest
kind load docker-image docker.io/karmada/karmada-operator:latest --name $HOST_CLUSTER_NAME

kubectl create namespace karmada-system --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config
kubectl create namespace test --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config
kubectl apply -f operator/config/deploy  --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config
# test for samples yamls
kubectl apply -f operator/config/samples --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config