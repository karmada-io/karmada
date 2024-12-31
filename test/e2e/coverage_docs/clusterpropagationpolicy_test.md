### cluster propagation policy e2e test coverage analysis

#### Basic propagation testing
| Test Case                                          | E2E Describe Text                      | Comments                                                                                       |
|----------------------------------------------------|----------------------------------------|------------------------------------------------------------------------------------------------|
| Test the propagation policy for CRD                | crd propagation testing                | [Resource propagating](https://karmada.io/docs/next/userguide/scheduling/resource-propagating) |
| Test the propagation policy for clusterRole        | clusterRole propagation testing        |                                                                                                |
| Test the propagation policy for clusterRoleBinding | clusterRoleBinding propagation testing |                                                                                                |
| Test the propagation policy for deployment         | deployment propagation testing         |                                                                                                |

#### Advanced propagation testing
| Test Case                                                                  | E2E Describe Text                              | Comments                                                                                                                    |
|----------------------------------------------------------------------------|------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------|
| Test add resourceSelector of the propagation policy for the deployment     | add resourceSelectors item(namespace scope)    | [Update propagationPolicy](https://karmada.io/docs/next/userguide/scheduling/resource-propagating#update-propagationpolicy) |
| Test update resourceSelector of the propagation policy for the deployment  | update resourceSelectors item(namespace scope) |                                                                                                                             |
| Test add resourceSelector of the propagation policy for the clusterRole    | add resourceSelectors item(cluster scope)      |                                                                                                                             |
| Test update resourceSelector of the propagation policy for the clusterRole | update resourceSelectors item(cluster scope)   |                                                                                                                             |
| Test update propagateDeps of the propagation policy for deployment         | update policy propagateDeps(namespace scope)   |                                                                                                                             |
| Test update placement of the propagation policy for deployment             | update policy placement(namespace scope)       |                                                                                                                             |
| Test update placement of the propagation policy for clusterRole            | update policy placement(cluster scope)         |                                                                                                                             |

#### ImplicitPriority propagation testing
| Test Case                                                                                                   | E2E Describe Text                                             | Comments |
|-------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------|----------|
| Test the priorityMatchName/priorityMatchLabel/priorityMatchAll implicit priority propagation for deployment | priorityMatchName/priorityMatchLabel/priorityMatchAll testing |          |

#### ExplicitPriority propagation testing
| Test Case                                                                        | E2E Describe Text                                                                         | Comments                                                                                                                                                                                             |
|----------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Test the high explicit/low priority/implicit priority propagation for deployment | high explicit/low priority/implicit priority ClusterPropagationPolicy propagation testing | [Configure PropagationPolicy/ClusterPropagationPolicy priority](https://karmada.io/docs/next/userguide/scheduling/resource-propagating#configure-propagationpolicyclusterpropagationpolicy-priority) |
| Test the same explicit priority propagation for deployment                       | same explicit priority ClusterPropagationPolicy propagation testing                       | [Choose from same-priority PropagationPolicies](https://karmada.io/docs/next/userguide/scheduling/resource-propagating#choose-from-same-priority-propagationpolicies)                                |

#### Delete clusterPropagation testing
| Test Case                                                                | E2E Describe Text                                                                                               | Comments |
|--------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------|----------|
| Test delete clusterpropagationpolicy for deployment                      | delete ClusterPropagationPolicy and check whether labels and annotations are deleted correctly(namespace scope) |          |
| Test delete clusterpropagationpolicy for deployment and create a new one | delete the old ClusterPropagationPolicy to unbind and create a new one(namespace scope)                         |          |
| Test delete clusterpropagationpolicy for CRD                             | delete ClusterPropagationPolicy and check whether labels and annotations are deleted correctly(cluster scope)   |          |
| Test delete clusterpropagationpolicy for CRD and create a new one        | delete the old ClusterPropagationPolicy to unbind and create a new one(cluster scope)                           |          |
