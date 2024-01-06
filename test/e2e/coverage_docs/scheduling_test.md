### scheduling e2e test coverage analysis

#### Propagation with label and group constraints
| Test Case                                                     | E2E Describe Text                                                  | Comments                                                                                                                       |
|---------------------------------------------------------------|--------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------|
| Test schedule the Deployment with label and group constraints | Deployment propagation with label and group constraints testing    |                                                                                                                                |
| Test schedule the CRD with label and group constraints        | Crd with specified label and group constraints propagation testing |                                                                                                                                |
| Test schedule the Job with label and group constraints        | Job propagation with label and group constraints testing           |                                                                                                                                |

#### Replica Scheduling Strategy testing
| Test Case                                                                                                                                                                                    | E2E Describe Text                                       | Comments |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------|----------|
| Test ReplicaScheduling when ReplicaSchedulingType value is Duplicated                                                                                                                        | replicas duplicated testing                             |          |
| Test ReplicaScheduling when ReplicaSchedulingType value is Duplicated, trigger rescheduling when replicas have changed                                                                       | replicas duplicated testing when rescheduling           |          |
| Test ReplicaScheduling when ReplicaSchedulingType value is Divided, ReplicaDivisionPreference value is Weighted, WeightPreference is nil                                                     | replicas divided and weighted testing                   |          |
| Test ReplicaScheduling when ReplicaSchedulingType value is Divided, ReplicaDivisionPreference value is Weighted, WeightPreference is nil, trigger rescheduling when replicas have changed    | replicas divided and weighted testing when rescheduling |          |
| Test ReplicaScheduling when ReplicaSchedulingType value is Divided, ReplicaDivisionPreference value is Weighted, WeightPreference isn't nil                                                  | replicas divided and weighted testing                   |          |
| Test ReplicaScheduling when ReplicaSchedulingType value is Divided, ReplicaDivisionPreference value is Weighted, WeightPreference isn't nil, trigger rescheduling when replicas have changed | replicas divided and weighted testing when rescheduling |          |

#### Job Replica Scheduling Strategy testing
| Test Case                                                     | E2E Describe Text                         | Comments |
|---------------------------------------------------------------|-------------------------------------------|----------|
| Test Job's parallelism are divided equally on member clusters | Job replicas divided and weighted testing |          |
