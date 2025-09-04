#!/usr/bin/env bash
# Copyright 2021 The Karmada Authors.
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

# This script is used to create a blank cluster for karmadactl init e2e testing.
# It creates a host cluster and prepares necessary components for testing.
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh
# 1. Used by developers to test karmadactl init with custom control plane setup

function usage() {
    echo "Usage:"
    echo "    hack/init-e2e-environment.sh [-h]"
    echo "    h: print help information"
}

while getopts 'h' OPT; do
    case $OPT in
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

# variable define
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
KUBECONFIG="${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config"
CLUSTER_VERSION=${CLUSTER_VERSION:-"${DEFAULT_CLUSTER_VERSION}"}
BUILD_PATH=${BUILD_PATH:-"${HOME}/karmada/_output/bin/linux/amd64"}

# install kind and kubectl
echo -n "Preparing: 'kind' existence check - "
if util::cmd_exist kind; then
  echo "passed"
else
  echo "not pass"
  # Install kind using the version defined in util.sh
  util::install_tools "sigs.k8s.io/kind" "${KIND_VERSION}"
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

# prepare the newest crds for karmadactl init
echo "Prepare the newest crds for karmadactl init"
cd  charts/karmada/
cp -r _crds crds
tar -zcvf ../../crds.tar.gz crds
cd -

# Verify CRDs package is created successfully
if [ ! -f "./crds.tar.gz" ]; then
  echo "ERROR: Failed to create CRDs package at ./crds.tar.gz"
  exit 1
fi
echo "CRDs package created successfully: ./crds.tar.gz"

# make karmadactl binary
echo "Building karmadactl binary..."
make karmadactl
echo "the route of karmadactl"
realpath _output/bin/linux/amd64/karmadactl





# create host cluster
echo "Start create host cluster..."
hack/create-cluster.sh ${HOST_CLUSTER_NAME} ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config > /dev/null 2>&1 &

# wait cluster ready
echo "Wait host cluster ready..."
util::wait_file_exist ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config 300
util::wait_context_exist ${HOST_CLUSTER_NAME} ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config 300
kubectl wait --for=condition=Ready nodes --all --timeout=800s --kubeconfig=${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config
util::wait_nodes_taint_disappear 800 ${KUBECONFIG_PATH}/${HOST_CLUSTER_NAME}.config

echo "Environment is ready for running karmadactl init e2e tests."