#!/usr/bin/env bash

set -o errexit
set -o nounset

# This script deploy karmada control plane to any cluster you want.	REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# This script depends on utils in: ${REPO_ROOT}/hack/util.sh

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
mkdir -p "${CERT_DIR}" &>/dev/null || sudo mkdir -p "${CERT_DIR}"
rm -f "${CERT_DIR:-${HOME}/.karmada}/*"
KARMADA_APISERVER_SECURE_PORT=${KARMADA_APISERVER_SECURE_PORT:-5443}

# The host cluster name which used to install karmada control plane components.
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
ROOT_CA_FILE=${CERT_DIR}/server-ca.crt
CFSSL_VERSION="v1.5.0"
CONTROLPLANE_SUDO=$(test -w "${CERT_DIR}" || echo "sudo -E")
CLUSTER_IP_ONLY=${CLUSTER_IP_ONLY:-false} # whether create a 'ClusterIP' type service for karmada apiserver
source "${REPO_ROOT}"/hack/util.sh

function usage() {
  echo "This script will deploy karmada control plane to a given cluster."
  echo "Usage: hack/deploy-karmada.sh <KUBECONFIG> <CONTEXT_NAME> [KARMADA_API_SERVER_IP]"
  echo "Example: hack/deploy-karmada.sh ~/.kube/config karmada-host"
  unset KUBECONFIG
}

if [[ $# -lt 2 ]]; then
  usage
  exit 1
fi

# check config file existence
HOST_CLUSTER_KUBECONFIG=$1
if [[ ! -f "${HOST_CLUSTER_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get kubernetes config file: '${HOST_CLUSTER_KUBECONFIG}', not existed.\n"
  usage
  exit 1
fi

# check context existence
export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
HOST_CLUSTER_NAME=$2
if ! kubectl config get-contexts "${HOST_CLUSTER_NAME}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${HOST_CLUSTER_NAME}' not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi

KARMADA_APISERVER_IP=${3:-}

# generate a secret to store the certificates
function generate_cert_secret {
    local karmada_ca
    karmada_ca=$(base64 "${ROOT_CA_FILE}" | tr -d '\r\n')

    local TEMP_PATH
    TEMP_PATH=$(mktemp -d)

    cp -rf "${REPO_ROOT}"/artifacts/deploy/karmada-cert-secret.yaml "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    cp -rf "${REPO_ROOT}"/artifacts/deploy/secret.yaml "${TEMP_PATH}"/secret-tmp.yaml
    cp -rf "${REPO_ROOT}"/artifacts/deploy/karmada-webhook-cert-secret.yaml "${TEMP_PATH}"/karmada-webhook-cert-secret-tmp.yaml

    sed -i "s/{{ca_crt}}/${karmada_ca}/g" "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    sed -i "s/{{client_cer}}/${KARMADA_CRT}/g" "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    sed -i "s/{{client_key}}/${KARMADA_KEY}/g" "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml

    sed -i "s/{{ca_crt}}/${karmada_ca}/g" "${TEMP_PATH}"/secret-tmp.yaml
    sed -i "s/{{client_cer}}/${KARMADA_CRT}/g" "${TEMP_PATH}"/secret-tmp.yaml
    sed -i "s/{{client_key}}/${KARMADA_KEY}/g" "${TEMP_PATH}"/secret-tmp.yaml

    sed -i "s/{{server_key}}/${KARMADA_KEY}/g" "${TEMP_PATH}"/karmada-webhook-cert-secret-tmp.yaml
    sed -i "s/{{server_certificate}}/${KARMADA_CRT}/g" "${TEMP_PATH}"/karmada-webhook-cert-secret-tmp.yaml

    kubectl apply -f "${TEMP_PATH}"/karmada-cert-secret-tmp.yaml
    kubectl apply -f "${TEMP_PATH}"/secret-tmp.yaml
    kubectl apply -f "${TEMP_PATH}"/karmada-webhook-cert-secret-tmp.yaml
    rm -rf "${TEMP_PATH}"
}

function installCRDs() {
    # install APIs
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/cluster.karmada.io_clusters.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_propagationpolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_clusterpropagationpolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_overridepolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_clusteroverridepolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/policy.karmada.io_replicaschedulingpolicies.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/work.karmada.io_works.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/work.karmada.io_resourcebindings.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/work.karmada.io_clusterresourcebindings.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/multicluster.x-k8s.io_serviceexports.yaml"
    kubectl apply -f "${REPO_ROOT}/artifacts/deploy/multicluster.x-k8s.io_serviceimports.yaml"
}

#generate cert
util::cmd_must_exist "openssl"
util::cmd_must_exist_cfssl ${CFSSL_VERSION}
# create CA signers
util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" server '"client auth","server auth"'
# signs a certificate
util::create_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "server-ca" karmada system:admin kubernetes.default.svc "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1"

# create namespace for control plane components
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/namespace.yaml"

# create service account, cluster role for controller-manager
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/serviceaccount.yaml"
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/clusterrole.yaml"
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/clusterrolebinding.yaml"

KARMADA_CRT=$(sudo base64 "${CERT_DIR}/karmada.crt" | tr -d '\r\n')
KARMADA_KEY=$(sudo base64 "${CERT_DIR}/karmada.key" | tr -d '\r\n')
generate_cert_secret

# deploy karmada etcd
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-etcd.yaml"

# Wait for karmada-etcd to come up before launching the rest of the components.
util::wait_pod_ready "${ETCD_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# If it provided a karmada API Server IP we can access karmada API Server (cluster by kind), we will create a ClusterIP type Service
# Or we need to create a LoadBalancer service($CLUSTER_IP_ONLY=false) so that we can access karmada API Server outside the karmada-host cluster
KARMADA_APISERVER_SERVICE_TYPE="ClusterIP"
if [[ -z "${KARMADA_APISERVER_IP}" ]] && [ "${CLUSTER_IP_ONLY}" = false ]; then
  KARMADA_APISERVER_SERVICE_TYPE="LoadBalancer"
fi

# deploy karmada apiserver
TEMP_PATH_APISERVER=$(mktemp -d)
cp "${REPO_ROOT}"/artifacts/deploy/karmada-apiserver.yaml "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml
sed -i "s/{{service_type}}/${KARMADA_APISERVER_SERVICE_TYPE}/g" "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml
echo -e "\nApply dynamic rendered apiserver service in ${TEMP_PATH_APISERVER}/karmada-apiserver.yaml."
kubectl apply -f "${TEMP_PATH_APISERVER}"/karmada-apiserver.yaml

# Wait for karmada-apiserver to come up before launching the rest of the components.
util::wait_pod_ready "${APISERVER_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"

# get Karmada apiserver IP
if [[ -z "${KARMADA_APISERVER_IP}" ]]; then
  case $KARMADA_APISERVER_SERVICE_TYPE in
    ClusterIP)  KARMADA_APISERVER_IP=$(kubectl get service karmada-apiserver -n "${KARMADA_SYSTEM_NAMESPACE}" -o=jsonpath='{.spec.clusterIP}')
    ;;
    LoadBalancer)  if util::wait_service_external_ip "karmada-apiserver" "${KARMADA_SYSTEM_NAMESPACE}"; then
      KARMADA_APISERVER_IP=$(util::get_load_balancer_ip)
      fi
    ;;
  esac
fi

if [[ -z "${KARMADA_APISERVER_IP}" ]]; then
  echo -e "ERROR: failed to get Karmada API server IP after creating service 'karmada-apiserver', please verify.\n"
  exit 1
fi

# write karmada api server config to kubeconfig file
util::append_client_kubeconfig "${HOST_CLUSTER_KUBECONFIG}" "${CERT_DIR}/karmada.crt" "${CERT_DIR}/karmada.key" "${KARMADA_APISERVER_IP}" "${KARMADA_APISERVER_SECURE_PORT}" karmada-apiserver

# deploy kube controller manager
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/kube-controller-manager.yaml"

# install CRD APIs on karmada apiserver.
if ! kubectl config get-contexts karmada-apiserver > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: karmada-apiserver not in ${HOST_CLUSTER_KUBECONFIG}."
  exit 1
fi
kubectl config use-context karmada-apiserver
installCRDs

# deploy webhook configurations on karmada apiserver
util::deploy_webhook_configuration "${ROOT_CA_FILE}" "${REPO_ROOT}/artifacts/deploy/webhook-configuration.yaml"

kubectl config use-context "${HOST_CLUSTER_NAME}"

# deploy controller-manager on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/controller-manager.yaml"
# deploy scheduler on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-scheduler.yaml"
# deploy webhook on host cluster
kubectl apply -f "${REPO_ROOT}/artifacts/deploy/karmada-webhook.yaml"

# make sure all karmada control plane components are ready
util::wait_pod_ready "${KARMADA_CONTROLLER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${KARMADA_SCHEDULER_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${KUBE_CONTROLLER_POD_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
util::wait_pod_ready "${KARMADA_WEBHOOK_LABEL}" "${KARMADA_SYSTEM_NAMESPACE}"
