### search e2e test coverage analysis

#### Test search
| Test Case                                                         | E2E Describe Text                                        | Comments |
|-------------------------------------------------------------------|----------------------------------------------------------|----------|
| Test search Service when there is no ResourceRegistry             | [Service] should be not searchable                       |          |
| Test search a searchable deployment                               | [member1 deployments] should be searchable               |          |
| Test search a not searchable deployment                           | [member2 deployments] should be not searchable           |          |
| Test search a searchable namespace                                | [member1 deployments namespace] should be searchable     |          |
| Test search a not searchable namespace                            | [member2 deployments namespace] should be not searchable |          |
| Test search node member1                                          | [member1 nodes] should be searchable                     |          |
| Test search node member2                                          | [member2 nodes] should be searchable                     |          |
| Test search member1's pods                                        | [member1 pods] should be searchable                      |          |
| Test search member2's pods                                        | [member2 pods] should be searchable                      |          |
| Test search clusterRole                                           | [clusterrole] should be searchable                       |          |
| Test search a not searchable clusterRoleBinding                   | [clusterrolebinding] should not be searchable            |          |
| Test search a clusterRoleBinding                                  | [clusterrolebinding] should be searchable                |          |
| Test search a daemonSet                                           | [daemonset] should be searchable                         |          |
| Test search a not searchable daemonSet                            | [daemonset] should not be searchable                     |          |
| Test search pods/nodes when create/update/delete resourceRegistry | create, update, delete resourceRegistry                  |          |
| Test get node                                                     | could get node                                           |          |
| Test list nodes                                                   | could list nodes                                         |          |
| Test get chunk list nodes                                         | could chunk list nodes                                   |          |
| Test list and watch nodes                                         | could list & watch nodes                                 |          |
| Test patch nodes                                                  | could path nodes                                         |          |
| Test update node                                                  | could update node                                        |          |

#### Test search after reconcile ResourceRegistry when clusters joined/updated
| Test Case                                                     | E2E Describe Text                                                         | Comments |
|---------------------------------------------------------------|---------------------------------------------------------------------------|----------|
| Test search deployment after join the cluster                 | [member clusters joined] could reconcile ResourceRegistry                 |          |
| Test search deployment after manage and unmanage the clusters | [member clusters updated, deleted label] could reconcile ResourceRegistry |          |
