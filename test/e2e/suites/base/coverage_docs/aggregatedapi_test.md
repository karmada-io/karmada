### aggregatedapi e2e test coverage analysis

#### Serviceaccount Access to (member1/member2/specified) cluster api with or without Permissions
| Test Case                                                                                     | E2E Describe Text                                         | Comments                                                                                                       |
|-----------------------------------------------------------------------------------------------|-----------------------------------------------------------|----------------------------------------------------------------------------------------------------------------|
| Serviceaccount Access to the member1 cluster's `/api` and `api/v1/nodes` with/without right   | tom access the member1 cluster api with and without right | [Aggregated Kubernetes API Endpoint](https://karmada.io/zh/docs/userguide/globalview/aggregated-api-endpoint/) |
| Serviceaccount Access to the member2 cluster's `/api` without right                           | tom access the member2 cluster without right              |                                                                                                                |
| Serviceaccount Access to the specified cluster's `/api` and `api/v1/nodes` with/without right | tom access the specified cluster with/without right       |                                                                                                                |
