### cron federated hpa e2e test coverage analysis

| Test Case                                                                                                 | E2E Describe Text                                       | Comments                                                                       |
|-----------------------------------------------------------------------------------------------------------|---------------------------------------------------------|--------------------------------------------------------------------------------|
| Test the scaling of FederatedHPA by creating a CronFederatedHPA rule                                      | Test scale FederatedHPA                                 | [FederatedHPA](https://karmada.io/zh/docs/userguide/autoscaling/federatedhpa/) |
| Test the scaling of a Deployment by creating a CronFederatedHPA rule                                      | Test scale Deployment                                   |                                                                                |
| Test the suspend rule in CronFederatedHPA by creating a rule that is supposed to be suspended             | Test suspend rule in CronFederatedHPA                   |                                                                                |
| Test the unsuspend rule, then suspend it in CronFederatedHPA by manipulating the rule's suspension status | Test unsuspend rule then suspend it in CronFederatedHPA |                                                                                |
