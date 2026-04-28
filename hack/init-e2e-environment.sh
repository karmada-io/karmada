#!/usr/bin/env bash
# Copyright 2025 The Karmada Authors.
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

# This script is used in workflow to set up a local karmadactl init e2e testing environment.
# It creates a host cluster, three member cluster and build the relevant images, CRDs, Karmadactl, etc. for the subsequent e2e testing.
# 1. Used by developers to test karmadactl init with custom control plane setup

function usage() {
    echo "This script is used in workflow to set up a local karmadactl init e2e testing environment."
    echo "It creates a host cluster, three member cluster and build the relevant images, CRDs, Karmadactl, etc. for the subsequent e2e testing."
    echo "Used by developers to test karmadactl init with custom control plane setup"
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
KARMADA_SYSTEM_NAMESPACE="karmada-system"

# variable define
export KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
export MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
export HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}

# step1. set up a base development environment
"${REPO_ROOT}"/hack/setup-dev-base.sh
export KUBECONFIG="${MAIN_KUBECONFIG}"

# step2 prepare the local crds
echo "Prepare the local crds"
cd  ${REPO_ROOT}/charts/karmada/
cp -r _crds crds
tar -zcvf ../../crds.tar.gz crds
cd -

# step2.1: Copy the local crds.tar.gz file to the specified path
DATA_DIR="${HOME}/karmada-data"
mkdir -p "${DATA_DIR}"
cp ./crds.tar.gz "${DATA_DIR}/crds.tar.gz"

echo "Environment is ready for running karmadactl init e2e tests."
