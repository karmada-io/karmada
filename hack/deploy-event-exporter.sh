#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}"/hack/util.sh

# Variable definitions
KUBECONFIG_PATH=${KUBECONFIG_PATH:-"${HOME}/.kube"}
MAIN_KUBECONFIG=${MAIN_KUBECONFIG:-"${KUBECONFIG_PATH}/karmada.config"}
HOST_CLUSTER_NAME=${HOST_CLUSTER_NAME:-"karmada-host"}
KARMADA_APISERVER_CLUSTER_NAME=${KARMADA_APISERVER_CLUSTER_NAME:-"karmada-apiserver"}
CERT_DIR=${CERT_DIR:-"${HOME}/.karmada"}
CFSSL_VERSION="v1.6.5"

# Ensure cfssl tools are available
util::cmd_must_exist_cfssl ${CFSSL_VERSION}

echo "Deploying Karmada Event Exporter..."

# Create kubeconfig secret for event-exporter to access Karmada API server
echo "[1/3] Creating kubeconfig secret for event-exporter..."

# Generate client certificate for event-exporter
util::create_certkey "" "${CERT_DIR}" "ca" event-exporter-client "system:karmada:event-exporter" "system:masters"

EVENT_EXPORTER_CLIENT_CRT=$(base64 < "${CERT_DIR}/event-exporter-client.crt" | tr -d '\r\n')
EVENT_EXPORTER_CLIENT_KEY=$(base64 < "${CERT_DIR}/event-exporter-client.key" | tr -d '\r\n')
KARMADA_CA=$(base64 < "${CERT_DIR}/ca.crt" | tr -d '\r\n')

# Create kubeconfig secret
TEMP_PATH=$(mktemp -d)
trap '{ rm -rf ${TEMP_PATH}; }' EXIT

cp "${REPO_ROOT}"/artifacts/deploy/karmada-config-secret.yaml "${TEMP_PATH}"/event-exporter-config-secret.yaml
sed -i'' -e "s/\${component}/event-exporter/g" "${TEMP_PATH}"/event-exporter-config-secret.yaml
sed -i'' -e "s/\${ca_crt}/${KARMADA_CA}/g" "${TEMP_PATH}"/event-exporter-config-secret.yaml
sed -i'' -e "s/\${client_crt}/${EVENT_EXPORTER_CLIENT_CRT}/g" "${TEMP_PATH}"/event-exporter-config-secret.yaml
sed -i'' -e "s/\${client_key}/${EVENT_EXPORTER_CLIENT_KEY}/g" "${TEMP_PATH}"/event-exporter-config-secret.yaml

kubectl --kubeconfig ~/.kube/karmada.config --context="${HOST_CLUSTER_NAME}" apply -f "${TEMP_PATH}"/event-exporter-config-secret.yaml

echo "✓ Kubeconfig secret created"

# Deploy event-exporter
echo "[2/3] Deploying event-exporter..."
kubectl --kubeconfig ~/.kube/karmada.config --context="${HOST_CLUSTER_NAME}" apply -f "${REPO_ROOT}/artifacts/deploy/karmada-event-exporter.yaml"

# Wait for event-exporter to be ready
echo "[3/3] Waiting for event-exporter to be ready..."
util::wait_pod_ready "${HOST_CLUSTER_NAME}" "app=event-exporter" "karmada-system"

echo ""
echo "✅ Event exporter deployed successfully!"
echo ""
echo "Events from the Karmada API server will now be exported as JSON logs."
echo "Query them in Loki with: {namespace=\"karmada-system\", app=\"event-exporter\"}"
echo ""
