# Manifest Directory

This directory contains CRD files used for Karmada E2E testing.

Downloading CRDs from external URLs makes tests dependent on network availability and external sources, which can lead to test flakiness.

By vendoring the CRD files locally, we ensure that test execution is not impacted by external network issues or unexpected changes to the remote repository. This approach allows for deliberate version control of CRDs used in tests, provides a clear audit trail, and prevents silent failures caused by external factors.

## Files

### flinkdeployment-cr.yaml

**Description:**

This is a FlinkDeployment custom resource (CR) template used for testing Karmada's multi-template scheduling capability.

**Purpose:**

This CR serves as the test workload for validating that Karmada can correctly schedule and manage resources with multiple pod templates (jobManager and taskManager), ensuring proper propagation across member clusters.

### flinkdeployments.flink.apache.org-v1.yaml

**Download Source:**
```
https://raw.githubusercontent.com/apache/flink-kubernetes-operator/release-1.13/helm/flink-kubernetes-operator/crds/flinkdeployments.flink.apache.org-v1.yml
```

**Why We Need It:**

Used to test Karmada's multi-template scheduling capability. The FlinkDeployment resource contains both jobManager and taskManager templates, which is ideal for testing Karmada's support for resources with multiple templates.

