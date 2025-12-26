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

# This script runs init e2e test against a blank cluster to test karmadactl init functionality.
# You should prepare your environment in advance and following environment may be you need to set or use default one.
# - HOST_KUBECONFIG: absolute path of host cluster KUBECONFIG file.
#
# Usage: hack/run-e2e-init.sh
# Example 1: hack/run-e2e-init.sh (run init e2e with default config)
# Example 2: export HOST_KUBECONFIG=<KUBECONFIG PATH> hack/run-e2e-init.sh (run init e2e with your KUBECONFIG)

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_KUBECONFIG=${HOST_KUBECONFIG:-"$KUBECONFIG_PATH/karmada.config"}
DATA_DIR=${DATA_DIR:-"${HOME}/karmada-data"}

# KARMADA_RUNNING_ON_KIND indicates if current testing against on karmada that installed on a kind cluster.
# Defaults to true.
# For kind cluster, the kind related logs will be collected after the testing.
KARMADA_RUNNING_ON_KIND=${KARMADA_RUNNING_ON_KIND:-true}

HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}

ARTIFACTS_PATH=${ARTIFACTS_PATH:-"${HOME}/karmada-e2e-init-logs"}
mkdir -p "$ARTIFACTS_PATH"

# Install ginkgo
GO111MODULE=on go install github.com/onsi/ginkgo/v2/ginkgo

# Run init e2e
export KUBECONFIG=${HOST_KUBECONFIG}
export PUSH_MODE_CLUSTER_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
export PUSH_MODE_KUBECONFIG_PATH=${PUSH_MODE_KUBECONFIG_PATH:-"$KUBECONFIG_PATH/members.config"}
export PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}
export PULL_MODE_KUBECONFIG_PATH=${PULL_MODE_KUBECONFIG_PATH:-"$KUBECONFIG_PATH/members.config"}
export CRDs_PATH=${CRDs_PATH:-"${DATA_DIR}/crds.tar.gz"}

set +e
ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/suites/init -- --host-context=${HOST_CLUSTER_NAME}
TESTING_RESULT=$?

# Collect logs
echo "Collect logs to $ARTIFACTS_PATH..."
cp "$HOST_KUBECONFIG" "$ARTIFACTS_PATH"

if [ "$KARMADA_RUNNING_ON_KIND" = true ]; then
  echo "Collecting $HOST_CLUSTER_NAME logs..."
  mkdir -p "$ARTIFACTS_PATH/$HOST_CLUSTER_NAME"
  kind export logs --name="$HOST_CLUSTER_NAME" "$ARTIFACTS_PATH/$HOST_CLUSTER_NAME"
fi

echo "Collected logs at $ARTIFACTS_PATH:"
ls -al "$ARTIFACTS_PATH"

# If E2E test failed, exit directly with the test result
echo "Init E2E test completed with exit code $TESTING_RESULT."
exit $TESTING_RESULT
