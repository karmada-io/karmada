# Failover Configuration Example

This example demonstrates a ProductionPropagationPolicy with comprehensive failover
configuration, including application-level failover with state preservation and
cluster-level failover.

## Scenario

A stateful application (Deployment + PVC) needs:
- Deployment to primary cluster (member1) with member2 as backup
- Application failover with 60s toleration
- State preservation during failover (preserve availableReplicas)
- Immediate purge on application failover (exactly-once consistency)
- Graceful purge on cluster failure
