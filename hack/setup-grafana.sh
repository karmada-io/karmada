#!/usr/bin/env bash

set -e

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
KUBECONFIG="${KUBECONFIG:-$HOME/.kube/karmada.config}"
KUBE_CONTEXT="${KUBE_CONTEXT:-karmada-host}"
NAMESPACE="monitoring"
GRAFANA_SERVICE="kps-grafana"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3010}"

echo "======================================================================"
echo "Grafana Setup Script"
echo "======================================================================"
echo ""

# Get Grafana admin password
echo "[1/4] Retrieving Grafana admin credentials..."
GRAFANA_PASSWORD=$(kubectl --kubeconfig="${KUBECONFIG}" \
  --context="${KUBE_CONTEXT}" \
  --namespace="${NAMESPACE}" \
  get secret "${GRAFANA_SERVICE}" \
  -o jsonpath="{.data.admin-password}" | base64 --decode)

if [ -z "$GRAFANA_PASSWORD" ]; then
  echo "Error: Failed to retrieve Grafana password"
  exit 1
fi

GRAFANA_USER="admin"
echo "✓ Credentials retrieved"
echo ""

# Wait for Grafana to be ready
echo "[2/3] Checking Grafana availability at ${GRAFANA_URL}..."
max_retries=30
retry_count=0
until curl -s -o /dev/null -w "%{http_code}" "${GRAFANA_URL}/api/health" | grep -q "200"; do
  if [ $retry_count -eq $max_retries ]; then
    echo "Error: Grafana is not available at ${GRAFANA_URL}"
    echo "Make sure port-forward is running: hc -n monitoring port-forward svc/kps-grafana 3010:80"
    exit 1
  fi
  echo "Waiting for Grafana to be ready... (attempt $((retry_count + 1))/$max_retries)"
  sleep 2
  retry_count=$((retry_count + 1))
done
echo "✓ Grafana is ready"
echo ""

# Create Karmada folder and import dashboards
echo "[3/3] Setting up Karmada dashboards..."

# Create Karmada folder
FOLDER_PAYLOAD='{"title": "Karmada"}'
FOLDER_RESPONSE=$(curl -s -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
  -H "Content-Type: application/json" \
  -X POST "${GRAFANA_URL}/api/folders" \
  -d "${FOLDER_PAYLOAD}" 2>/dev/null || echo "")

FOLDER_UID=$(echo "$FOLDER_RESPONSE" | grep -o '"uid":"[^"]*"' | cut -d'"' -f4)

if [ -z "$FOLDER_UID" ]; then
  # Folder might already exist, try to get it
  EXISTING_FOLDER=$(curl -s -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
    "${GRAFANA_URL}/api/folders" | grep -A 10 '"title":"Karmada"' || echo "")
  FOLDER_UID=$(echo "$EXISTING_FOLDER" | grep -o '"uid":"[^"]*"' | head -1 | cut -d'"' -f4)

  if [ -z "$FOLDER_UID" ]; then
    echo "Error: Failed to create or find Karmada folder"
    exit 1
  fi
  echo "✓ Karmada folder already exists (UID: $FOLDER_UID)"
else
  echo "✓ Karmada folder created (UID: $FOLDER_UID)"
fi

# Import each dashboard
DASHBOARD_DIR="${SCRIPT_ROOT}/grafana"
IMPORTED_COUNT=0
SKIPPED_COUNT=0

for dashboard_file in "${DASHBOARD_DIR}"/*.json; do
  if [ ! -f "$dashboard_file" ]; then
    continue
  fi

  dashboard_name=$(basename "$dashboard_file")
  echo "  Importing ${dashboard_name}..."

  # Read dashboard JSON and wrap it for import
  DASHBOARD_JSON=$(cat "$dashboard_file")
  IMPORT_PAYLOAD=$(jq -n \
    --arg folderUid "$FOLDER_UID" \
    --argjson dashboard "$DASHBOARD_JSON" \
    '{
      dashboard: $dashboard,
      folderUid: $folderUid,
      overwrite: true
    }')

  IMPORT_RESPONSE=$(curl -s -w "\n%{http_code}" -u "${GRAFANA_USER}:${GRAFANA_PASSWORD}" \
    -H "Content-Type: application/json" \
    -X POST "${GRAFANA_URL}/api/dashboards/db" \
    -d "${IMPORT_PAYLOAD}")

  HTTP_CODE=$(echo "$IMPORT_RESPONSE" | tail -n1)

  if [ "$HTTP_CODE" -eq 200 ]; then
    IMPORTED_COUNT=$((IMPORTED_COUNT + 1))
    echo "  ✓ ${dashboard_name} imported"
  else
    SKIPPED_COUNT=$((SKIPPED_COUNT + 1))
    echo "  ✗ ${dashboard_name} failed (HTTP $HTTP_CODE)"
  fi
done

echo ""
echo "✓ Dashboard import complete: $IMPORTED_COUNT imported, $SKIPPED_COUNT failed"
echo ""

echo "======================================================================"
echo "✅ Grafana setup complete!"
echo "======================================================================"
echo ""
echo "You can now access Grafana at: ${GRAFANA_URL}"
echo "  - Prometheus data source: Default (configured via Helm)"
echo "  - Loki data source: Available for log queries (configured via Helm)"
echo "  - Karmada dashboards: Available in the 'Karmada' folder"
echo ""
echo "Note: Datasources are provisioned via monitoring-values.yaml"
echo ""
