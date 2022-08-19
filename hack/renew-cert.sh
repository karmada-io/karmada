#!/usr/bin/env bash

set -o errexit
set -o nounset

# This script helps update certificate expiration date

CFSSL_VERSION="v1.5.0"

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
mkdir -p "${CERT_DIR}" &>/dev/null ||  mkdir -p "${CERT_DIR}"
rm -f "${CERT_DIR}/*" &>/dev/null ||  rm -f "${CERT_DIR}/*"
ROOT_CA_FILE=${CERT_DIR}/ca.crt

source "${REPO_ROOT}"/hack/util.sh

function usage() {
  echo "This script will fix certificate expiration issue."
  echo "!!!Waring: this operation will update your kubeconfig file of Karmada Control Plane."
  echo "Usage: hack/renew-cert.sh <HOST_KUBECONFIG> <HOST_CONTEXT_NAME> <KARMADA_KUBECONFIG> <KARMADA_CONTEXT_NAME>"
  echo -e "Parameters:\n\tHOST_KUBECONFIG\t\tThe kubeconfig of the Host Cluster that Karmada installed to"
  echo -e "\tHOST_CONTEXT_NAME\tThe name of context for 'HOST_KUBECONFIG'"
  echo -e "\tKARMADA_KUBECONFIG\tThe kubeconfig of the Karmada Control Plane Kubernetes that you can access Karmada's resources by"
  echo -e "\tKARMADA_CONTEXT_NAME\tThe name of context for 'KARMADA_KUBECONFIG'"
  echo "Example: hack/renwe-cert.sh ~/.kube/config karmada-host ~/.kube/config karmada-apiserver"
}

# check config file existence
HOST_CLUSTER_KUBECONFIG=${1:-}
if [[ ! -f "${HOST_CLUSTER_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get Host Cluster Kubernetes config file: '${HOST_CLUSTER_KUBECONFIG}', not existed.\n"
  usage
  exit 1
fi
export KUBECONFIG="${HOST_CLUSTER_KUBECONFIG}"
# check context existence and switch
HOST_CONTEXT_NAME=${2:-}
if ! kubectl config use-context "${HOST_CONTEXT_NAME}"> /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${HOST_CONTEXT_NAME}' is not in ${HOST_CLUSTER_KUBECONFIG}. \n"
  usage
  exit 1
fi

KARMADA_KUBECONFIG=${3:-}
if [[ ! -f "${KARMADA_KUBECONFIG}" ]]; then
  echo -e "ERROR: failed to get Karmada Control Plane Kubernetes config file: '${KARMADA_KUBECONFIG}', not existed.\n"
  usage
  exit 1
fi

KARMADA_CONTEXT_NAME=${4:-}
if ! kubectl config get-contexts "${KARMADA_CONTEXT_NAME}" --kubeconfig="${KARMADA_KUBECONFIG}" > /dev/null 2>&1;
then
  echo -e "ERROR: failed to get context: '${KARMADA_CONTEXT_NAME}' is not in ${KARMADA_KUBECONFIG}. \n"
  usage
  exit 1
fi

AUTHINFO_NAME=$(kubectl config get-contexts "${KARMADA_CONTEXT_NAME}" --no-headers=true --kubeconfig="${KARMADA_KUBECONFIG}" | awk '{print $NF}')

# Use x.x.x.6 IP address, which is the same CIDR with the node address of the Kind cluster,
# as the loadBalancer service address of component karmada-interpreter-webhook-example.
interpreter_webhook_example_service_external_ip_prefix=$(echo $(util::get_apiserver_ip_from_kubeconfig "${HOST_CONTEXT_NAME}") | awk -F. '{printf "%s.%s.%s",$1,$2,$3}')
interpreter_webhook_example_service_external_ip_address=${interpreter_webhook_example_service_external_ip_prefix}.6

# generate cert
util::cmd_must_exist "openssl"
util::cmd_must_exist_cfssl ${CFSSL_VERSION}
# create CA signers
util::create_signing_certkey "" "${CERT_DIR}" ca karmada '"client auth","server auth"'
util::create_signing_certkey "" "${CERT_DIR}" front-proxy-ca front-proxy-ca '"client auth","server auth"'
util::create_signing_certkey "" "${CERT_DIR}" etcd-ca etcd-ca '"client auth","server auth"'
# signs a certificate
util::create_certkey "" "${CERT_DIR}" "ca" karmada system:admin "system:masters" kubernetes.default.svc "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1" "${interpreter_webhook_example_service_external_ip_address}"
util::create_certkey "" "${CERT_DIR}" "ca" apiserver karmada-apiserver "" "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1"
util::create_certkey "" "${CERT_DIR}" "front-proxy-ca" front-proxy-client front-proxy-client "" kubernetes.default.svc "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1"
util::create_certkey "" "${CERT_DIR}" "etcd-ca" etcd-server etcd-server "" kubernetes.default.svc "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1"
util::create_certkey "" "${CERT_DIR}" "etcd-ca" etcd-client etcd-client "" "*.etcd.karmada-system.svc.cluster.local" "*.karmada-system.svc.cluster.local" "*.karmada-system.svc" "localhost" "127.0.0.1"

# store certs and keys to the secret
util::generate_cert_secret "${REPO_ROOT}" "${CERT_DIR}" "${ROOT_CA_FILE}"
# update kebuconfig (karmada-apiserver) file
kubectl config set-credentials "${AUTHINFO_NAME}" --client-certificate="${CERT_DIR}/karmada.crt" --client-key="${CERT_DIR}/karmada.key" --embed-certs=true --kubeconfig="${KARMADA_KUBECONFIG}"

echo "Renew Done!"
echo "Please repalce all kubeconfigs copied from ${HOST_CLUSTER_KUBECONFIG}"
