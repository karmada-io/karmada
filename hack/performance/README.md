# Karmada Local Performance Testing Guide

This guide explains how to run performance tests locally and generate a comparison report
between two versions of Karmada (e.g. a feature branch vs. the `master` branch).

The steps below mirror what the
[CI Performance Compare workflow](../../.github/workflows/ci-performance-compare.yaml)
does automatically on GitHub Actions, but executed entirely on your own machine.

---

## Overview

The full workflow consists of three stages:

```
1. Set up environment  →  hack/local-up-performance.sh
2. Run performance test  →  hack/run-performance-test.sh
                              └─ calls hack/performance/collect-metrics.sh
3. Compare results  →  hack/performance/visualize-metrics.py
```

Stages 1–2 must be repeated **once per version** you want to compare (baseline and target).
Stage 3 is run once at the end to produce the comparison charts.

---

## Prerequisites

| Tool | Purpose |
|------|---------|
| Go (version in `go.mod`) | Building Karmada from source |
| Docker | Required by Kind |
| `kind` | Creates local Kubernetes clusters (auto-installed if missing) |
| `kubectl` | Interacts with clusters (auto-installed if missing) |
| `kwokctl` | Creates lightweight simulated member clusters (auto-installed if missing) |
| `jq` | JSON processing used by `collect-metrics.sh` (auto-installed if missing) |
| `curl` | Querying Prometheus from `collect-metrics.sh` |
| Python 3 + `pip` | Running `visualize-metrics.py` |
| `matplotlib` | Python charting library used by `visualize-metrics.py` |

Before running the scripts, make sure Python 3 and `pip` are available, then install
`matplotlib`:

```bash
# Install Python 3 and pip (if not already present)
# On Debian/Ubuntu:
sudo apt-get install -y python3 python3-pip
# On macOS:
brew install python3

# Install the required Python package
pip install matplotlib
```

---

## Directory Structure

```
hack/performance/
├── setup-control-plane.sh       # Creates the Karmada host Kind cluster
├── setup-member-clusters.sh     # Creates simulated member clusters via kwokctl
├── collect-metrics.sh           # Queries Prometheus and writes a metrics JSON file
├── visualize-metrics.py         # Generates comparison charts from two JSON files
├── clusterloader2-config.yaml   # ClusterLoader2 load-test scenario
├── clusterloader2-configmap.yaml
├── clusterloader2-deployment.yaml
├── clusterloader2-policy.yaml   # PropagationPolicy used during the load test
├── kwok-node.yaml               # Node template for simulated nodes
├── prometheus-application.yaml  # Prometheus deployment
└── prometheus-rbac.yaml         # RBAC resources for Prometheus
```

The two top-level entry points that tie everything together:

| Script | What it does |
|--------|-------------|
| `hack/local-up-performance.sh` | Deploys the full test environment (calls `setup-control-plane.sh` then `setup-member-clusters.sh`) |
| `hack/run-performance-test.sh` | Runs the load test and collects metrics (calls `collect-metrics.sh` internally) |

---

## Step 1 – Set up the test environment

Run this from the repository root:

```bash
hack/local-up-performance.sh
```

Internally this script:

1. Builds all Karmada component images from source (unless `BUILD_FROM_SOURCE=false`).
2. Creates a Kind cluster named `karmada-host` and installs Karmada on it.
3. Loads the built images into the Kind cluster.
4. Creates `CLUSTER_COUNT` simulated member clusters via `kwokctl`, each pre-populated
   with `NODES_PER_CLUSTER` virtual nodes.
5. Joins all simulated clusters to the Karmada control plane.
6. Deploys Prometheus for metrics collection (accessible on NodePort `31801`).

**Environment variables** (all optional, shown with their defaults):

| Variable | Default | Description |
|----------|---------|-------------|
| `BUILD_FROM_SOURCE` | `true` | Build Karmada images locally before loading into Kind. Set to `false` to use published images. |
| `HOST_CLUSTER_NAME` | `karmada-host` | Name of the Kind cluster that hosts Karmada |
| `CLUSTER_COUNT` | `10` | Number of simulated member clusters to create |
| `NODES_PER_CLUSTER` | `10` | Number of virtual nodes per simulated cluster |
| `CLUSTER_VERSION` | *(defined in `hack/util.sh`)* | Kubernetes version used for Kind/kwok clusters |
| `CHINA_MAINLAND` | *(unset)* | Set to any non-empty value to pull images from Chinese mirror registries |

---

## Step 2 – Run the performance test and collect metrics

```bash
export OUTPUT_FILE=./baseline-metrics/karmada-metrics.json
hack/run-performance-test.sh
```

This runs the ClusterLoader2 load test against the Karmada environment and then invokes
`hack/performance/collect-metrics.sh` to query Prometheus and write all metrics to
`OUTPUT_FILE` as a JSON file.

**Environment variables for `collect-metrics.sh`** (all optional):

| Variable | Default | Description |
|----------|---------|-------------|
| `OUTPUT_FILE` | `./karmada-metrics.json` | Destination path for the collected metrics JSON |
| `PROMETHEUS_ENDPOINT` | `http://localhost:31801` | Prometheus service URL |
| `END_TIME` | current Unix timestamp | End of the collection window |
| `START_TIME` | `END_TIME − 300` (5 min before) | Start of the collection window |
| `STEP` | `15s` | Resolution step for Prometheus range queries |
| `RATE_INTERVAL` | `2m` | PromQL rate/range window (used in `rate()` expressions) |

---

## Step 3 – Repeat for the target version

Check out the version you want to compare and repeat Steps 1–2 with a different output
path. `hack/local-up-performance.sh` automatically tears down any existing clusters and
recreates the environment from scratch, so no manual cleanup is needed. Running both
tests on the same machine also ensures the environment is consistent between the two
measurements.

```bash
# Check out the target branch / PR
git checkout <your-feature-branch>          # or: git fetch origin pull/<PR>/head && git checkout FETCH_HEAD

# Re-deploy the environment (existing clusters are cleaned up automatically)
hack/local-up-performance.sh

# Run the test, saving results to a different path
export OUTPUT_FILE=./target-metrics/karmada-metrics.json
hack/run-performance-test.sh
```

---

## Step 4 – Generate the comparison report

Run the visualization script:

```bash
python3 hack/performance/visualize-metrics.py \
  --baseline ./baseline-metrics/karmada-metrics.json \
  --target   ./target-metrics/karmada-metrics.json \
  --output-dir ./performance-comparison/
```

The script produces one or more PNG images (e.g. `metrics-report-01.png`,
`metrics-report-02.png`, …) inside `--output-dir`, each containing up to 8 sub-plots.

### `visualize-metrics.py` arguments

| Argument | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `--baseline` | ✅ | — | Path to the baseline metrics JSON |
| `--target` | ✅ | — | Path to the target metrics JSON |
| `--output-dir` | | `performance-comparison` | Output directory (created if absent) |
| `--output` | | `metrics-report` | Filename prefix for the generated images |

### Understanding the charts

Each chart overlays **baseline** (dashed lines) and **target** (solid lines):

| Plot type | What it shows |
|-----------|--------------|
| **Latency** | P50 / P90 / P99 percentiles in seconds |
| **Rate** | Success and error throughput (requests per second) |
| **Count** | Cumulative success / error totals over time |

---

## Complete example

```bash
# ── Baseline (master) ──────────────────────────────────────────────────────
git checkout master
hack/local-up-performance.sh
export OUTPUT_FILE=./baseline-metrics/karmada-metrics.json
hack/run-performance-test.sh

# ── Target (feature branch) ────────────────────────────────────────────────
# hack/local-up-performance.sh automatically tears down the existing clusters
# and recreates the environment, so no manual cleanup is needed.
git checkout my-feature-branch
hack/local-up-performance.sh
export OUTPUT_FILE=./target-metrics/karmada-metrics.json
hack/run-performance-test.sh

# ── Generate comparison charts ─────────────────────────────────────────────
python3 hack/performance/visualize-metrics.py \
  --baseline ./baseline-metrics/karmada-metrics.json \
  --target   ./target-metrics/karmada-metrics.json \
  --output-dir ./performance-comparison/

# Open the generated images
ls ./performance-comparison/
```

---

## CI workflow reference

The [ci-performance-compare.yaml](../../.github/workflows/ci-performance-compare.yaml)
workflow automates all of the above on GitHub Actions. To trigger it:

1. Go to **Actions → Performance Compare Workflow → Run workflow** in the GitHub UI.
2. Fill in the two inputs:
   - **`base_branch`** – the baseline branch (default: `master`).
   - **`target_pr_number`** – the PR number whose `HEAD` is the target.
3. The workflow runs `base-performance-test` and `target-performance-test` in parallel,
   uploads each `karmada-metrics.json` as an artifact, and finally runs
   `compare-performance` to produce and upload the comparison charts.

---

## Cleanup

After all testing is done, remove the remaining clusters with the following commands
(adjust `HOST_CLUSTER_NAME` and `CLUSTER_COUNT` if you overrode those variables during
setup):

```bash
# Delete the Karmada host Kind cluster
kind delete cluster --name "${HOST_CLUSTER_NAME:-karmada-host}"

# Delete all simulated member clusters
for i in $(seq 1 "${CLUSTER_COUNT:-10}"); do
  kwokctl delete cluster --name="foo-${i}"
done
```

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| `matplotlib` not found | Missing Python dependency | `pip install matplotlib` |
| Empty or all-zero metrics JSON | Prometheus unreachable | Check `PROMETHEUS_ENDPOINT`; run `kubectl get svc -n monitor` to verify the NodePort |
| Kind cluster creation fails | Insufficient disk space | Free up disk or increase Docker's storage limit |
| `kwokctl` command not found | Tool not installed | Install `kwokctl` and ensure it is on `$PATH` |
| Slow image loading into Kind | Large Docker layers | Pre-pull base images, or set `BUILD_FROM_SOURCE=false` to use published images |
