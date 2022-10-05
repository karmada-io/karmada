#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# variable define
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
KARMADA_APISERVER=${KARMADA_APISERVER:-"karmada-apiserver"}
MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}

# delete interpreter webhook example in karmada-host
export KUBECONFIG="${MAIN_KUBECONFIG}"
kubectl --context="${HOST_CLUSTER_NAME}" delete -f "${REPO_ROOT}"/examples/customresourceinterpreter/karmada-interpreter-webhook-example.yaml

# uninstall metallb
kubectl --context="${HOST_CLUSTER_NAME}" --ignore-not-found=true delete -f https://raw.githubusercontent.com/metallb/metallb/v0.13.5/config/manifests/metallb-native.yaml

kubectl --context="${HOST_CLUSTER_NAME}" get configmap kube-proxy -n kube-system -o yaml | \
sed -e "s/strictARP: true/strictARP: false/" | \
kubectl --context="${HOST_CLUSTER_NAME}" apply -f - -n kube-system

# delete interpreter workload webhook configuration
kubectl --context="${KARMADA_APISERVER}" delete ResourceInterpreterWebhookConfiguration examples

# delete interpreter example workload CRD in karmada-apiserver and member clusters
kubectl --context="${KARMADA_APISERVER}" delete -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
export KUBECONFIG="${MEMBER_CLUSTER_KUBECONFIG}"
kubectl --context="${MEMBER_CLUSTER_1_NAME}" delete -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
kubectl --context="${MEMBER_CLUSTER_2_NAME}" delete -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
kubectl --context="${PULL_MODE_CLUSTER_NAME}" delete -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
