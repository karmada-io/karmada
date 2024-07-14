### propagation policy e2e test coverage analysis

#### Basic propagation testing

| Test Case                                                      | E2E Describe Text                      | Comments                                                                                       |
|----------------------------------------------------------------|----------------------------------------|------------------------------------------------------------------------------------------------|
| Test the propagationPolicy for deployment                      | deployment propagation testing         | [Resource propagating](https://karmada.io/docs/next/userguide/scheduling/resource-propagating) |
| Test the propagationPolicy for service                         | service propagation testing            |                                                                                                |
| Test the propagationPolicy for pod                             | pod propagation testing                |                                                                                                |
| Test the propagationPolicy for namespace scope custom resource | namespaceScoped cr propagation testing |                                                                                                |
| Test the propagationPolicy for job                             | job propagation testing                |                                                                                                |
| Test the propagationPolicy for role                            | role propagation testing               |                                                                                                |
| Test the propagationPolicy for roleBinding                     | roleBinding propagation testing        |                                                                                                |

#### ImplicitPriority propagation testing

| Test Case                                                     | E2E Describe Text                                                                                                                | Comments                                                                                                                           |
|---------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------|
| priorityMatchName/priorityMatchLabel/priorityMatchAll testing | check whether the deployment uses the highest priority propagationPolicy (priorityMatchName/priorityMatchLabel/priorityMatchAll) | [Configure implicit priority](https://karmada.io/docs/next/userguide/scheduling/resource-propagating/#configure-implicit-priority) |

#### ExplicitPriority propagation testing

| Test Case                                                                        | E2E Describe Text                                                                  | Comments                                                                                                                                                              |
|----------------------------------------------------------------------------------|------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Test the high explicit/low priority/implicit priority propagation for deployment | high explicit/low priority/implicit priority PropagationPolicy propagation testing | [Configure explicit priority](https://karmada.io/docs/next/userguide/scheduling/resource-propagating#configure-explicit-priority)                                     |
| Test the same explicit priority propagation for deployment                       | same explicit priority PropagationPolicy propagation testing                       | [Choose from same-priority PropagationPolicies](https://karmada.io/docs/next/userguide/scheduling/resource-propagating#choose-from-same-priority-propagationpolicies) |

#### Advanced propagation testing

| Test Case                                                                | E2E Describe Text                                                                           | Comments                                                                                                                    |
|--------------------------------------------------------------------------|---------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------|
| Test add resourceSelector of the propagationPolicy for the deployment    | add resourceSelectors item                                                                  | [Update propagationPolicy](https://karmada.io/docs/next/userguide/scheduling/resource-propagating#update-propagationpolicy) |
| Test update resourceSelector of the propagationPolicy for the deployment | update resourceSelectors item                                                               |                                                                                                                             |
| Test update propagateDeps of the propagationPolicy for the deployment    | update policy propagateDeps                                                                 |                                                                                                                             |
| Test update placement of the propagationPolicy for the deployment        | update policy placement                                                                     |                                                                                                                             |
| Test delete propagationpolicy for deployment                             | delete the propagationPolicy and check whether labels and annotations are deleted correctly |                                                                                                                             |
| Test modify the old propagationPolicy to unbind and create a new one     | modify the old propagationPolicy to unbind and create a new one                             |                                                                                                                             |
| Test delete the old propagationPolicy to unbind and create a new one     | delete the old propagationPolicy to unbind and create a new one                             |                                                                                                                             |
