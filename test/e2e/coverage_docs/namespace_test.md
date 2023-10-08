### namespace e2e test coverage analysis

| Test Case                                                                                                                                     | E2E Describe Text                         | Comments                                                                                                                         |
|-----------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------|
| Create a namespace that needs to be automatically synchronized across all member clusters                                                     | create a namespace in karmada-apiserver   ||
| Delete a namespace, and all member clusters need to automatically synchronize the deletion                                                    | delete a namespace from karmada-apiserver ||
| When a new cluster joins, the namespaces on the Karmada control plane (excluding reserved namespaces) need to be synchronized to that cluster | joining new cluster                       | [Namespace Management](https://karmada.io/docs/next/userguide/bestpractices/namespace-management/#default-namespace-propagation) |
