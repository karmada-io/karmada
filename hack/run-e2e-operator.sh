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

# This script runs e2e test against on a kubernetes cluster which deployed karmada operator.
# You should prepare your environment in advance, for example, use hack/operator-e2e-environment.sh to set up the environment.
#
# Usage: hack/run-e2e-operator.sh
# Example 1: hack/run-e2e-operator.sh (run operator e2e with default config)

KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
HOST_KUBECONFIG=${HOST_KUBECONFIG:-"$KUBECONFIG_PATH/karmada.config"}

# RUNNING_ON_KIND indicates if current testing against on a kind cluster.
# Defaults to true.
# For kind cluster, the kind related logs will be collected after the testing.
RUNNING_ON_KIND=${RUNNING_ON_KIND:-true}

HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}

ARTIFACTS_PATH=${ARTIFACTS_PATH:-"${HOME}/karmada-operator-e2e-logs"}
mkdir -p "$ARTIFACTS_PATH"

# Install ginkgo
GO111MODULE=on go install github.com/onsi/ginkgo/v2/ginkgo

# Run e2e
export KUBECONFIG=${HOST_KUBECONFIG}

set +e
ginkgo -v --race --trace --fail-fast -p --randomize-all ./test/e2e/suites/operator
TESTING_RESULT=$?

# Collect logs
echo "Collect logs to $ARTIFACTS_PATH..."
cp "$HOST_KUBECONFIG" "$ARTIFACTS_PATH"

if [ "$RUNNING_ON_KIND" = true ]; then
  echo "Collecting $HOST_CLUSTER_NAME logs..."
  mkdir -p "$ARTIFACTS_PATH/$HOST_CLUSTER_NAME"
  kind export logs --name="$HOST_CLUSTER_NAME" "$ARTIFACTS_PATH/$HOST_CLUSTER_NAME"
fi

echo "Collected logs at $ARTIFACTS_PATH:"
ls -al "$ARTIFACTS_PATH"

exit $TESTING_RESULT
