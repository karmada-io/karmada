# karmada-controller-manager

## Description
Understand and interact with the Karmada controller-manager component — its
controllers, configuration, logging, and troubleshooting.

## When to Load
- User asks about "karmada-controller-manager", "binding controller", "execution controller".
- User asks "which controller handles X?".
- User needs to check controller-manager logs, configuration, or status.
- User asks about controller-manager flags, leader election, or tuning.
- User asks about feature gates controlled by the controller-manager.

## Workflow

1. **Understand the user's goal.**
   - Checking controller health? → Step 2 (status).
   - Debugging propagation issue? → Step 3 (relevant controller + logs).
   - Understanding a controller's responsibility? → Step 4 (controller overview).
   - Configuring controller-manager? → Step 5 (flags/options).

2. **Check controller-manager health:**
   ```bash
   kubectl get pods -n karmada-system -l app=karmada-controller-manager
   kubectl logs -n karmada-system deployment/karmada-controller-manager --tail=50
   ```
   Key health indicators:
   - Pod status: Running, CrashLoopBackOff, Pending?
   - Logs: Error messages, reconciliation failures?
   - Leader election: Multiple replicas? Only leader processes.

3. **Identify the relevant controller for debugging:**

   | Controller | Responsibility | Debug Signal |
   |-----------|---------------|-------------|
   | **Binding Controller** | Matches resources to policies, creates ResourceBindings | No ResourceBinding for matched resource |
   | **Execution Controller** | Creates Work objects from scheduled ResourceBindings | No Work for scheduled binding |
   | **Override Controller** | Applies OverridePolicy rules to workloads | Overrides not taking effect |
   | **Cluster Controller** | Manages cluster join/unjoin, health status | Cluster status issues |
   | **Namespace Controller** | Auto-propagates namespaces to member clusters | Namespace not created on member clusters |
   | **Taint Manager** | Handles cluster taints and workload eviction | Taint-related eviction issues |
   | **Graceful Eviction Controller** | Manages graceful eviction tasks | Eviction stuck or incomplete |
   | **FederatedResourceQuota Controller** | Manages federated resource quotas | Quota enforcement issues |

4. **Access controller logs:**
   ```bash
   # Last 100 lines
   kubectl logs -n karmada-system deployment/karmada-controller-manager --tail=100

   # Search for specific resource in logs
   kubectl logs -n karmada-system deployment/karmada-controller-manager | grep "<resource-name>"

   # Search for errors
   kubectl logs -n karmada-system deployment/karmada-controller-manager | grep -i error

   # Follow logs in real-time
   kubectl logs -n karmada-system deployment/karmada-controller-manager -f
   ```

5. **Controller-manager configuration flags:**

   Common flags (check actual deployment for full list):
   - `--controllers`: Comma-separated list of controllers to enable (default: all).
   - `--feature-gates`: Feature gates (e.g., `Failover=true,PropagateDeps=true`).
   - `--leader-elect`: Enable leader election (default: true).
   - `--leader-elect-lease-duration`: Lease duration in seconds.
   - `--bind-address`: Address to bind metrics and health probes.
   - `--metrics-bind-address`: Address for metrics endpoint.
   - `--health-probe-bind-address`: Address for health probe endpoint.
   - `--skipped-propagating-namespaces`: Additional namespaces to skip (beyond system namespaces).
   - `--skipped-propagating-apis`: APIs to skip propagating (e.g., to exclude specific CRDs).
   - `--no-execute-taint-eviction-purge-mode`: Default purge mode for taint-based evictions.

6. **Feature gates managed by controller-manager:**
   - `PropagateDeps`: Auto-propagate dependent resources.
   - `Failover`: Application and cluster failover handling.
   - `GracefulEviction`: Graceful workload eviction.
   - `MultiplePodTemplatesScheduling`: Multi-component workload support.
   - `StatefulFailoverInjection`: State preservation during failover.
   - `PriorityBasedScheduling`: Priority-based scheduling.
   - `Flux2GitopsAdaption`: Flux2 GitOps integration.
   - `ResourceQuotaEstimate`: Resource quota estimation.

7. **Provide guidance based on findings:**
   - Controller crash → Check resource usage, recent changes, API server connectivity.
   - Controller not processing → Check leader election, skipped namespaces/APIs.
   - Specific error in logs → Match to troubleshooting knowledge base.

## Knowledge References
- `hack/agent-skills/knowledge/components.md`
- `hack/agent-skills/knowledge/troubleshooting.md`
