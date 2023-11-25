### cluster override policy e2e test coverage analysis

#### The basic ClusterOverridePolicy testing
| Test Case                                                                | E2E Describe Text               | Comments                                                                                |
|--------------------------------------------------------------------------|---------------------------------|-----------------------------------------------------------------------------------------|
| Check if the ClusterOverridePolicy will update the namespace label value | Namespace labelOverride testing | [Override Policy](https://karmada.io/zh/docs/next/userguide/scheduling/override-policy) |

#### The ClusterOverridePolicy with nil resourceSelectors testing
| Test Case                                                                                           | E2E Describe Text                | Comments                                                                                                                            |
|-----------------------------------------------------------------------------------------------------|----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------|
| Check if the ClusterOverridePolicy with nil resourceSelector will update the deployment image value | deployment imageOverride testing | [Override Policy with nil resourceSelector](https://karmada.io/zh/docs/next/userguide/scheduling/override-policy#resource-selector) |
