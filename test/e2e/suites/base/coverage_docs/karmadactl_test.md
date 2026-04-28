### karmadactl e2e test coverage analysis

| Test Case                                                                            | E2E Describe Text                                         | Comments                                                                                                         |
|--------------------------------------------------------------------------------------|-----------------------------------------------------------|------------------------------------------------------------------------------------------------------------------|
| Test promoting namespace scope resource(deployment) by karmadactl                    | Test promoting a deployment from cluster member           | [Karmadactl promote](https://karmada.io/docs/next/reference/karmadactl/karmadactl-commands/karmadactl_promote)   |
| Test promoting cluster scope resource(clusterrole, clusterrolebinding) by karmadactl | Test promoting clusterrole and clusterrolebindings        |                                                                                                                  |
| Test promoting namespace scope resource(service) by karmadactl                       | Test promoting a service from cluster member              |                                                                                                                  |
| Test join cluster and unjoin not ready cluster by karmadactl                         | Test unjoining not ready cluster                          | [Karmadactl unjoin](https://karmada.io/docs/next/reference/karmadactl/karmadactl-commands/karmadactl_unjoin)     |
| Test cordon cluster by karmadactl                                                    | cluster %s should have unschedulable:NoSchedule taint     | [Karmadactl cordon](https://karmada.io/docs/next/reference/karmadactl/karmadactl-commands/karmadactl_cordon)     |
| Test uncordon cluster by karmadactl                                                  | cluster %s should not have unschedulable:NoSchedule taint | [Karmadactl uncordon](https://karmada.io/docs/next/reference/karmadactl/karmadactl-commands/karmadactl_uncordon) |
| Test exec command by karmadactl                                                      | Test exec command                                         | [Karmadactl exec](https://karmada.io/docs/next/reference/karmadactl/karmadactl-commands/karmadactl_exec)         |

#### TODO:
1. Add the testcase for the rest commands https://karmada.io/docs/next/reference/karmadactl/karmadactl-commands/karmadactl#see-also.
