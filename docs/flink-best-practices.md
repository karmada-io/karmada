# Best Practice Guide: Running Flink on Multi-Cluster with Karmada

This document describes commonly observed deployment patterns and community practices. It does not imply official feature guarantees or support commitments.

## Overview

Apache Flink is a stateful stream processing framework. Running Flink in a multi-cluster environment with Karmada offers significant benefits for high availability (HA) and disaster recovery (DR), utilizing Karmada's unified management capabilities.

## Why Run Flink on Karmada?

- **Disaster Recovery**: Enabling workload rescheduling to healthy clusters during cluster-wide failures.
- **Resource Bursting**: Utilizing available capacity across multiple availability zones or regions.
- **Unified Management**: Centralized orchestration of Flink Deployments and Operators.

## Architecture Guidelines

A typical multi-cluster Flink setup involves:

1.  **Control Plane (Karmada)**: Manages the distribution of the Flink Kubernetes Operator and Flink applications.
2.  **Member Clusters**: Host the Flink workloads. Each member cluster typically runs a local instance of the **Flink Kubernetes Operator**.
3.  **Shared Storage**: External object storage (e.g., S3, GCS, OSS, MinIO) is strictly recommended for storing Checkpoints and Savepoints to ensure accessibility across cluster boundaries.
4.  **Observability Layer**: A federated metrics system (e.g., Thanos, VictoriaMetrics) to aggregate data from multiple clusters.

## Deployment Patterns

### Flink Kubernetes Operator

It is recommended to deploy the Flink Kubernetes Operator in every member cluster where Flink jobs are expected to run.

- **Propagation**: A `ClusterPropagationPolicy` can be used to distribute the Operator (via Helm or manifests) to all member clusters.
- **Consistency**: Maintaining consistent Operator versions across clusters helps avoid compatibility issues during failover.

### Flink Applications

Flink jobs are typically deployed using the `FlinkDeployment` Custom Resource (CR).

- **Application Mode**: This mode is commonly preferred over Session Mode for multi-cluster setups due to better isolation.
- **Scheduling**:
  - **Strict Placement**: Using `placement.clusterAffinity` pins a job to a specific cluster when data locality is a priority.
  * **Failover Strategy**: A `Failover` propagation strategy allows Karmada to reschedule deployments to healthy clusters upon detection of cluster faults.

## Failover & Recovery Considerations

### State Maintenance

When a Flink deployment moves between clusters, the new pod replicas essentially start fresh in the new environment.

- **External State**: Configuring Flink to write checkpoints to external object storage (e.g., `s3://...`) is critical for allowing a job to resume state in a new cluster.
- **Recovery Flow**: In a typical failover scenario where a new cluster takes over, the new `FlinkDeployment` must know the path to the latest checkpoint.
  - **Manual**: Operators can manually update the `initialSavepointPath` in the CR.
  - **Automated**: Advanced setups often use admission webhooks or controllers in the member cluster to inject the correct checkpoint path based on the Job ID.

## State Management Configuration

- **High Availability**: Enable Kubernetes HA (`high-availability: org.apache.flink.kubernetes.highavailability.KubernetesHaServicesFactory`).
- **Checkpoint Directory**: Ensure `state.checkpoints.dir` points to shared, externally accessible storage.

## Network & Data Locality

- **Ingress**: Multi-Cluster Ingress (MCI) or weighted DNS checks are common patterns for exposing the Flink Web UI.
- **Data Sources**: Streaming sources (e.g., Kafka) need to be routable from all potential member clusters.

## Observability

- **Metrics**: Flink metrics are typically scraped by a local Prometheus instance in each member cluster.
- **Aggregation**: A centralized view is achieved by federating these local instances into a global store.

## Operational Considerations & Limitations

The following aspects represent current observations and common operational constraints:

1.  **State Resume Mechanism**: Karmada propagates the resource specification. If a job moves to a new cluster, the `FlinkDeployment` spec must be updated (manually or via external tooling) with the `initialSavepointPath` for the job to resume exactly where it left off.
2.  **Storage Scope**: Standard Kubernetes PVCs are bound to specific clusters. State storage should rely on object storage rather than PVCs for multi-cluster portability.
3.  **Job Scope**: Spanning a single Flink Job (e.g., JobManager in Cluster A, TaskManager in Cluster B) is generally discouraged due to network latency sensitivity. The recommended pattern is to keep a whole Job within a single cluster boundary.
