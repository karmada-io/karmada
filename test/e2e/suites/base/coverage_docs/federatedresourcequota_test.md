### federated resource quota e2e test coverage analysis

| Test Case                                                                  | E2E Describe Text                                                  | Comments                                                                                                 |
|----------------------------------------------------------------------------|--------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------|
| Test if the ResourceQuota propagated to member clusters                    | federatedResourceQuota should be propagated to member clusters     | [FederatedResourceQuota](https://karmada.io/docs/next/userguide/bestpractices/federated-resource-quota/) |
| Test if the ResourceQuota removed from member clusters                     | federatedResourceQuota should be removed from member clusters      |                                                                                                          |
| Test if the ResourceQuota propagated to new joined clusters                | federatedResourceQuota should be propagated to new joined clusters |                                                                                                          |
| Test if the federatedResourceQuota collect CPu and Memory status correctly | federatedResourceQuota status should be collect correctly          |                                                                                                          |
