# Karmada Setup and Monitoring Playbook

This playbook guides you through setting up a **Karmada** instance and configuring **Prometheus** and **Grafana** monitoring on the Karmada host cluster.

---

## 1. Set Up a Local Karmada Instance

Run the following script to deploy a local Karmada environment:

```bash
hack/local-up-karmada.sh
```

This will create a Karmada instance that is joined to a set of Kind member clusters.

---

## 2. Load Shell Convenience Aliases

Source convenience aliases:

```bash
source .karmadarc
```

This step enables short commands like `hc` for interacting with the Karmada host cluster.

---

## 3. Install Kube-Prometheus-Stack in the Karmada Host Cluster

Add the Prometheus Helm repository and install the monitoring stack into the `monitoring` namespace:

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm upgrade --install kps prometheus-community/kube-prometheus-stack --namespace monitoring --create-namespace --kubeconfig ~/.kube/karmada.config --kube-context karmada-host
```

This installs Prometheus, Grafana, and related components for cluster observability.

---

## 4. Create a Certificate for Prometheus to Scrape the Karmada API Server

Generate the required TLS certificate for secure scraping:

```bash
hack/create-scrape-user.sh
```

This ensures Prometheus can securely collect metrics from the Karmada API server, whose metrics endpoint is protected.

---

## 5. Create PodMonitors

Apply the `PodMonitor` CRs to enable Prometheus to scrape metrics from the Karmada components:

```bash
hc apply -f prom
```

---

## 6. Retrieve Grafana Admin Credentials

Obtain the Grafana administrator password from the Kubernetes secret:

```bash
hc --namespace monitoring get secrets kps-grafana -o jsonpath="{.data.admin-password}" | base64 --decode
```

**Default credentials:**
- **Username:** `admin`
- **Password:** `prom-operator` (unless overridden)

---

## 7. Expose Grafana and Prometheus on the Host Network via Port Forwarding

Expose Grafana and Prometheus on the host network via port forwarding:

```bash
hc -n monitoring port-forward svc/kps-grafana 3010:80
```

```bash
hc -n monitoring port-forward svc/kps-kube-prometheus-stack-prometheus 9090
```

Grafana will be available at [http://localhost:3010](http://localhost:3010), and Prometheus at [http://localhost:9090](http://localhost:9090).

---

## 8. Import Grafana Dashboards

Log in to Grafana and import the predefined dashboards located in the `grafana` directory to visualize Karmada metrics.

---

### ✅ Summary

You now have:
- A local Karmada instance running.
- Monitoring enabled via Prometheus and Grafana.
- Secure metric collection and custom dashboards for observability.
