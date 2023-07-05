### E2E coverage of features

Features: https://karmada.io/docs/key-features/features/

## Cross-cloud multi-cluster multi-mode management
| Feature               | E2E test coverage | Coverage details                                                             |
|-----------------------|-------------------|------------------------------------------------------------------------------|
| Safe isolation        | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/namespace_test.go |
| Multi-mode connection | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/namespace_test.go |
| Multi-cloud support   | No                | Hard to test                                                                 |


## Multi-policy multi-cluster scheduling
| Feature                                                               | E2E test coverage | Coverage details                                                                                                                                                         |
|-----------------------------------------------------------------------|-------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Cluster distribution capability under different scheduling strategies | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/clusteraffinities_test.go <br/> https://github.com/karmada-io/karmada/blob/master/test/e2e/scheduling_test.go |
| Differential configuration(OverridePolicy)                            | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/clusteroverridepolicy_test.go                                                                                 |
| Reschedule(Descheduler, Scheduler-estimator)                          | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/scheduling_test.go                                                                                            |


## Cross-cluster failover of applications
| Feature                | E2E test coverage | Coverage details                                                                   |
|------------------------|-------------------|------------------------------------------------------------------------------------|
| Cluster failover       | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/failover_test.go        |
| Cluster taint settings | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/tainttoleration_test.go |
| Uninterrupted service  | No                | No specified test for the feature                                                  |


## Global Uniform Resource View
| Feature                                    | E2E test coverage | Coverage details                                                            |
|--------------------------------------------|-------------------|-----------------------------------------------------------------------------|
| Resource status collection and aggregation | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/resource_test.go |
| Unified resource management                | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/resource_test.go |
| Unified operations:                        | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/search_test.go   |
| Global search for resources and events     | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/search_test.go   |


## Best Production Practices
| Feature                                        | E2E test coverage | Coverage details                                                                          |
|------------------------------------------------|-------------------|-------------------------------------------------------------------------------------------|
| Unified authentication                         | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/search_test.go                 |
| Unified resource quota(FederatedResourceQuota) | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/federatedresourcequota_test.go |
| Reusable scheduling strategy                   | No                | No need for e2e test                                                                      |


## Cross-cluster service governance
| Feature                                        | E2E test coverage | Coverage details                                                       |
|------------------------------------------------|-------------------|------------------------------------------------------------------------|
| Multi-cluster service discovery                | Yes               | https://github.com/karmada-io/karmada/blob/master/test/e2e/mcs_test.go |
| Multi-cluster network support                  | No                | No need for e2e test                                                   |
| Cross-Cluster service governance via ErieCanal | No                | No need for e2e test, ErieCanal is an independent component            |
