---
title: Eviction Queue Rate Limiting
authors:
  - "@wanggang"
reviewers:
approvers:
creation-date: 2025-8-08
---

# Eviction Queue Rate Limiting

## Summary

This proposal introduces an eviction queue with rate limiting for the Karmada taint manager. The eviction queue enhances the fault migration mechanism by controlling the rate of resource evictions using a configurable fixed rate parameter. The implementation also provides metrics for monitoring the eviction process and improving overall system observability.

## Motivation

In multi-cluster environments, when clusters experience failures, resources need to be evicted and rescheduled to healthy clusters. If many clusters fail simultaneously or in quick succession, aggressive eviction and rescheduling can overwhelm the healthy clusters and the control plane, potentially leading to cascading failures. To mitigate this, the eviction queue supports a configurable fixed eviction rate to avoid overloading the system.

### Goals

- Implement an eviction queue with a configurable fixed rate limiting parameter
- Provide metrics for monitoring the eviction queue's performance
- Ensure the eviction queue can handle both ResourceBinding and ClusterResourceBinding resources
- Minimize changes to the existing taint manager architecture while adding these capabilities
- Support configuration of eviction rate through command-line flags

### Non-Goals

- Redesigning the entire taint manager architecture
- Implementing dynamic rate limiting based on cluster health (see Future Work)
- Providing a UI for monitoring the eviction queue
- Supporting custom rate limiting strategies beyond fixed rate parameter

## Proposal

The proposal introduces a rate-limited eviction queue that processes resource evictions at a fixed, configurable rate.

### User Stories

#### Story 1: Preventing Cascading Failures During Large-Scale Outages
As a cluster administrator, I want to ensure that when multiple clusters experience failures simultaneously, the system doesn't overwhelm the remaining healthy clusters with too many evictions at once. The rate limiting should slow down or limit evictions, protecting the system as a whole.

#### Story 2: Monitoring Eviction Performance
As a cluster administrator, I want to monitor the performance of the eviction queue, including the number of pending evictions, processing latency, and success/failure rates. This helps tune the configuration and respond to operational issues.

#### Story 3: Configuring Eviction Behavior for Different Environments
As a cluster administrator, I want to configure the eviction behavior based on my environment's characteristics. For example, in a production environment with many mission-critical workloads, a lower eviction rate might be preferable than in a development or test setup.

### Design Details

#### 1. EvictionQueueOptions
A configuration structure that controls the behavior of the eviction queue:
- `EvictionRate`: The default eviction rate (per second).
  This option can be configured through command-line flags.

#### 2. EvictionWorker
- An enhanced worker manages the eviction queue and processes resources at a limited rate based on the configuration.
- Collects metrics on queue depth, processing latency, and success/failure rates.
- Supports tracking metrics by resource kind.

#### 3. Metrics Collection
- Queue depth metrics
- Resource kind metrics
- Processing latency metrics
- Success/failure metrics

#### 4. Component Interactions
1. The controller manager initializes the TaintManager with EvictionQueueOptions
2. The TaintManager creates two EvictionWorker instances, one for ResourceBinding and one for ClusterResourceBinding
3. When resources need to be evicted, they are added to the appropriate queue
4. EvictionWorkers process items from the queue at the configured rate
5. Metrics are collected throughout the process and exposed to Prometheus

### Implementation Details
#### New Files / Code changes
- **pkg/controllers/cluster/evictionqueue_config/evictionoption.go**
    - Defines the EvictionQueueOptions structure and methods to register command-line flags
- **pkg/controllers/cluster/eviction_worker.go**
    - Implements the EvictionWorker interface and queue rate limiting
    - Integrates with metrics collection
- **cmd/controller-manager/app/options/options.go**
    - Add EvictionQueueOptions field to Options structure
    - Register command-line flag for eviction rate option
- **cmd/controller-manager/app/controllermanager.go**
    - Pass EvictionQueueOptions to TaintManager on creation
- **pkg/controllers/context/context.go**
    - Add EvictionQueueOptions field to Options structure

### Workflow
1. When a cluster becomes unhealthy, the taint manager identifies resources that need to be evicted
2. These resources are added to the appropriate eviction queue (ResourceBinding or ClusterResourceBinding)
3. The EvictionWorker processes items from the queue at a fixed rate
4. Metrics are collected and exposed to Prometheus for monitoring

## Alternatives Considered

### Alternative 1: Separate Eviction Manager Component
Implementing a separate eviction manager was considered, but would require significant changes to the existing architecture.

### Alternative 2: Single Queue for All Resource Types
Using a single queue for all resource types rather than maintaining separate queues for ResourceBinding and ClusterResourceBinding was considered but not prioritized for this iteration.

## Implementation Plan
1. Implement the EvictionQueueOptions structure and command-line flags
2. Implement the EvictionWorker with fixed rate limiting
3. Implement metrics collection
4. Modify the TaintManager to use the eviction queue
5. Add tests and documentation
6. Submit for review

## Open Issues
1. The current design has two separate eviction queues for ResourceBinding and ClusterResourceBinding. While this aligns with the existing taint manager architecture, it might be more efficient to have a single queue for all resource types in the future.

## Outlook
- **Dynamic Rate Limiting**: In a future iteration, we may introduce a dynamic rate limiter that can adjust the eviction rate based on real-time system health (e.g., number or percentage of unhealthy clusters, API server load, pending evictions, rate of cluster failures). This would allow for more adaptive and safer scaling of evictions during large-scale failures or recovery events.
- **Custom Rate Strategies**: Support for pluggable/customized rate-limiting logic or more granular controls (e.g., prioritization, selective throttling, etc.)
- **More Metrics/Observability**: Add richer, more granular metrics for queue operations and health.

Dynamic and adaptive rate limiting is left as future work to ensure the initial delivery is simple and robust, with expansion possible based on operational experience and best practices. 