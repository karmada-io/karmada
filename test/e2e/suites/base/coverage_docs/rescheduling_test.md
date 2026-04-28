### rescheduling e2e test coverage analysis

| Test Case                                                                                            | E2E Describe Text                                                              | Comments |
|------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------|----------|
| Test if the deployment will rescheduled to other available clusters when some clusters are unjoined  | deployment reschedule testing                                                  |          |
| Test when the ReplicaSchedulingType is Duplicate and join some clusters then recovered               | when the ReplicaSchedulingType of the policy is Duplicated, reschedule testing |          |
| Test when the ReplicaScheduling of the policy is nil and join some clusters then recovered           | when the ReplicaScheduling of the policy is nil, reschedule testing            |          |
| Test if the deployment will rescheduled when the labels match or not match(PropagationPolicy)        | change labels to testing deployment reschedule(PropagationPolicy)              |          |
| Test if the deployment will rescheduled when the labels match or not match(ClusterPropagationPolicy) | change labels to testing deployment reschedule(ClusterPropagationPolicy)       |          |
