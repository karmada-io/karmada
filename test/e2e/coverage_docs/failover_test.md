### failover e2e test coverage analysis

| Test Case                                                                          | E2E Describe Text                              | Comments                                                                                     |
|------------------------------------------------------------------------------------|------------------------------------------------|----------------------------------------------------------------------------------------------|
| Test if the deployment of failed cluster is rescheduled to other available cluster | deployment failover testing                    | [Failover](https://karmada.io/docs/next/userguide/failover/failover-overview)                |
| Test if the deployment of taint cluster is rescheduled to other available cluster  | taint cluster                                  |                                                                                              |
| Test if the deployment will rescheduled to other available cluster                 | application failover with purgeMode graciously | [Application failover](https://karmada.io/docs/next/userguide/failover/application-failover) |
| Test if the deployment will never rescheduled to other available cluster           | application failover with purgeMode never      |                                                                                              |

#### TODO
1. There are 2 way to evict the deployment of the Graciously PurgeMode, the third case cover the first way only, we may add a test case [when the GracePeriodSeconds is reach out](https://karmada.io/docs/next/userguide/failover/application-failover/#:~:text=after%20a%20timeout%20is%20reached%20before%20evicting%20the%20application).
