#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# variable define
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
KARMADA_APISERVER=${KARMADA_APISERVER:-"karmada-apiserver"}
MEMBER_CLUSTER_KUBECONFIG=${MEMBER_CLUSTER_KUBECONFIG:-"${KUBECONFIG_PATH}/members.config"}
MEMBER_CLUSTER_1_NAME=${MEMBER_CLUSTER_1_NAME:-"member1"}
MEMBER_CLUSTER_2_NAME=${MEMBER_CLUSTER_2_NAME:-"member2"}
PULL_MODE_CLUSTER_NAME=${PULL_MODE_CLUSTER_NAME:-"member3"}

export VERSION="latest"
export REGISTRY="docker.io/karmada"

CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
ROOT_CA_FILE=${CERT_DIR}/ca.crt

# load interpreter webhook example image
kind load docker-image "${REGISTRY}/karmada-interpreter-webhook-example:${VERSION}" --name="${HOST_CLUSTER_NAME}"

export KUBECONFIG="${MAIN_KUBECONFIG}"

# Due to we are using kube-proxy in IPVS mode, we have to enable strict ARP mode.
# refer to https://metallb.universe.tf/installation/#preparation
kubectl --context="${HOST_CLUSTER_NAME}" get configmap kube-proxy -n kube-system -o yaml | \
sed -e "s/strictARP: false/strictARP: true/" | \
kubectl --context="${HOST_CLUSTER_NAME}" apply -f - -n kube-system

# install metallb by manifest, refer to https://metallb.universe.tf/installation/#installation-by-manifest
# disable the webhook-mode because there is a bug:https://github.com/metallb/metallb/issues/1597
# this webhook only checks crd, it has not effects to out e2e tests
curl https://raw.githubusercontent.com/metallb/metallb/v0.13.5/config/manifests/metallb-native.yaml -k | \
  sed '0,/args:/s//args:\n        - --webhook-mode=disabled/' | \
  sed '/apiVersion: admissionregistration/,$d' | \
  kubectl --context="${HOST_CLUSTER_NAME}" apply -f -
util::wait_pod_ready "${HOST_CLUSTER_NAME}" metallb metallb-system

# Use x.x.x.8 IP address, which is the same CIDR with the node address of the Kind cluster,
# as the loadBalancer service address of component karmada-interpreter-webhook-example.
interpreter_webhook_example_service_external_ip_prefix=$(echo $(util::get_apiserver_ip_from_kubeconfig "${HOST_CLUSTER_NAME}") | awk -F. '{printf "%s.%s.%s",$1,$2,$3}')
interpreter_webhook_example_service_external_ip_address=${interpreter_webhook_example_service_external_ip_prefix}.8

# config with layer 2 configuration. refer to https://metallb.universe.tf/configuration/#layer-2-configuration
cat <<EOF | kubectl --context="${HOST_CLUSTER_NAME}" apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: metallb-config
  namespace: metallb-system
spec:
  addresses:
  - ${interpreter_webhook_example_service_external_ip_address}-${interpreter_webhook_example_service_external_ip_address}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: metallb-advertisement
  namespace: metallb-system
EOF

# deploy interpreter webhook example in karmada-host
kubectl --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}"/examples/customresourceinterpreter/karmada-interpreter-webhook-example.yaml
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "${INTERPRETER_WEBHOOK_EXAMPLE_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# deploy interpreter workload webhook-configuration.yaml
cp -rf "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration.yaml" "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration-temp.yaml"
sed -i'' -e "s/{{karmada-interpreter-webhook-example-svc-address}}/${interpreter_webhook_example_service_external_ip_address}/g" "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration-temp.yaml"
util::deploy_webhook_configuration "${KARMADA_APISERVER}" "${ROOT_CA_FILE}" "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration-temp.yaml"
rm -rf "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration-temp.yaml"

# install interpreter example workload CRD in karmada-apiserver and member clusters
kubectl --context="${KARMADA_APISERVER}" apply -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
export KUBECONFIG="${MEMBER_CLUSTER_KUBECONFIG}"
kubectl --context="${MEMBER_CLUSTER_1_NAME}" apply -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
kubectl --context="${MEMBER_CLUSTER_2_NAME}" apply -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
kubectl --context="${PULL_MODE_CLUSTER_NAME}" apply -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
