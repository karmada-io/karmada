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
export REGISTRY="swr.ap-southeast-1.myhuaweicloud.com/karmada"

CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
ROOT_CA_FILE=${CERT_DIR}/server-ca.crt

# load interpreter webhook example image
kind load docker-image "${REGISTRY}/karmada-interpreter-webhook-example:${VERSION}" --name="${HOST_CLUSTER_NAME}"

export KUBECONFIG="${MAIN_KUBECONFIG}"
kubectl config use-context "${HOST_CLUSTER_NAME}"

# Due to we are using kube-proxy in IPVS mode, we have to enable strict ARP mode.
# refer to https://metallb.universe.tf/installation/#preparation
kubectl get configmap kube-proxy -n kube-system -o yaml | \
sed -e "s/strictARP: false/strictARP: true/" | \
kubectl apply -f - -n kube-system

# install metallb by manifest, refer to https://metallb.universe.tf/installation/#installation-by-manifest
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.12.1/manifests/namespace.yaml
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.12.1/manifests/metallb.yaml
util::wait_pod_ready metallb metallb-system

# Use x.x.x.6 IP address, which is the same CIDR with the node address of the Kind cluster,
# as the loadBalancer service address of component karmada-interpreter-webhook-example.
interpreter_webhook_example_service_external_ip_prefix=$(echo $(util::get_apiserver_ip_from_kubeconfig "${HOST_CLUSTER_NAME}") | awk -F. '{printf "%s.%s.%s",$1,$2,$3}')
interpreter_webhook_example_service_external_ip_address=${interpreter_webhook_example_service_external_ip_prefix}.6

# config with layer 2 configuration. refer to https://metallb.universe.tf/configuration/#layer-2-configuration
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - ${interpreter_webhook_example_service_external_ip_address}-${interpreter_webhook_example_service_external_ip_address}
EOF

# deploy interpreter webhook example in karmada-host
kubectl apply -f "${REPO_ROOT}"/examples/customresourceinterpreter/karmada-interpreter-webhook-example.yaml
util::wait_pod_ready "${INTERPRETER_WEBHOOK_EXAMPLE_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# deploy interpreter workload webhook-configuration.yaml
kubectl config use-context "${KARMADA_APISERVER}"
cp -rf "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration.yaml" "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration-temp.yaml"
sed -i'' -e "s/{{karmada-interpreter-webhook-example-svc-address}}/${interpreter_webhook_example_service_external_ip_address}/g" "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration-temp.yaml"
util::deploy_webhook_configuration "${ROOT_CA_FILE}" "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration-temp.yaml"
rm -rf "${REPO_ROOT}/examples/customresourceinterpreter/webhook-configuration-temp.yaml"

# install interpreter example workload CRD in karamada-apiserver and member clusters
kubectl apply -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
export KUBECONFIG="${MEMBER_CLUSTER_KUBECONFIG}"
kubectl config use-context "${MEMBER_CLUSTER_1_NAME}"
kubectl apply -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
kubectl config use-context "${MEMBER_CLUSTER_2_NAME}"
kubectl apply -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
kubectl config use-context "${PULL_MODE_CLUSTER_NAME}"
kubectl apply -f "${REPO_ROOT}/examples/customresourceinterpreter/apis/workload.example.io_workloads.yaml"
