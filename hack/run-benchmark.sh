#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script runs benchmark test on karmada control plane.
# Before run it, you should:
# 1. setup test environment, and set the karmada apiserver config file to environment var `KUBECONFIG`;
# 2. join some member clusters into karmada control plane.
# 3. run mass pods on member clusters
#
# Usage:
#   hack/run-benchmark.sh <OPTIONS>
# Environments:
#   KUBECONFIG: absolute path of karmada apiserver config file
# Options:
#  --context string
#        Name of the cluster context in control plane kubeconfig file.
#  --kube-api-burst int
#        Burst to use while talking with kubernetes apiserver. (default 2000)
#  --kube-api-qps float
#        QPS to use while talking with kubernetes apiserver. (default 1000)
#  --kubeconfig string
#        Paths to a kubeconfig. Only required if out-of-cluster.
#  --sampling-max-number int
#        the maximum number of samples to record. (default 0)
#  --sampling-max-time duration
#        The maximum amount of time to spend recording samples. (default 5m0s)
#  --sampling-number-parallel int
#        The number of parallel workers to spin up to record samples. (default 10)
#  --test-namespace string
#        test namespace. (default default)
#  --create-resource-registry bool
#        Create createResourceRegistry before test or not. (default true)
#
# Examples:
#   hack/run-benchmark.sh
#   hack/run-benchmark.sh --context=karmada-apiserver
#   hack/run-benchmark.sh --test-namespace=testing

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# Install ginkgo
GO111MODULE=on go install github.com/onsi/ginkgo/v2/ginkgo

ginkgo -v --trace --fail-fast "${REPO_ROOT}"/test/benchmark/ -- "$@"
