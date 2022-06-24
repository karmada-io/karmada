# Overview
This document explains how lifted code is managed.
A common user case for this task is developer lifting code from other repositories to `pkg/util/lifted` directory.

- [Steps of lifting code](#steps-of-lifting-code)
- [How to write lifted comments](#how-to-write-lifted-comments)
- [Examples](#examples)

## Steps of lifting code
- Copy code from another repository and save it to a go file under `pkg/util/lifted`.
- Optionally change the lifted code.
- Add lifted comments for the code [as guided](#how-to-write-lifted-comments).
- Run `hack/update-lifted.sh` to update the lifted doc `pkg/util/lifted/doc.go`.

## How to write lifted comments
Lifted comments shall be placed just before the lifted code (could be a func, type, var or const). Only empty lines and comments are allowed between lifted comments and lifted code.

Lifted comments are composed by one or multi comment lines, each in the format of `+lifted:KEY[=VALUE]`. Value is optional for some keys. 

Valid keys are as follow：

- source: 

  Key `source` is required. Its value indicates where the code is lifted from.

- changed: 

  Key `changed` is optional. It indicates whether the code is changed. Value is optional (`true` or `false`, defaults to `true`). Not adding this key or setting it to `false` means no code change.

## Examples
### Lifting function

Lift function `IsQuotaHugePageResourceName` to `corehelpers.go`:

```go
// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go#L57-L61

// IsQuotaHugePageResourceName returns true if the resource name has the quota
// related huge page resource prefix.
func IsQuotaHugePageResourceName(name corev1.ResourceName) bool {
	return strings.HasPrefix(string(name), corev1.ResourceHugePagesPrefix) || strings.HasPrefix(string(name), corev1.ResourceRequestsHugePagesPrefix)
}
```

Added in `doc.go`:

```markdown
| lifted file              | source file                                                                                                                   | const/var/type/func                     | changed |
|--------------------------|-------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------|---------|
| corehelpers.go           | https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/apis/core/helper/helpers.go#L57-L61                            | func IsQuotaHugePageResourceName        | N       |
```

### Changed lifting function

Lift and change function `GetNewReplicaSet` to `deployment.go`

```go
// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/controller/deployment/util/deployment_util.go#L536-L544
// +lifted:changed

// GetNewReplicaSet returns a replica set that matches the intent of the given deployment; get ReplicaSetList from client interface.
// Returns nil if the new replica set doesn't exist yet.
func GetNewReplicaSet(deployment *appsv1.Deployment, f ReplicaSetListFunc) (*appsv1.ReplicaSet, error) {
	rsList, err := ListReplicaSetsByDeployment(deployment, f)
	if err != nil {
		return nil, err
	}
	return FindNewReplicaSet(deployment, rsList), nil
}
```

Added in `doc.go`:

```markdown
| lifted file              | source file                                                                                                                   | const/var/type/func                     | changed |
|--------------------------|-------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------|---------|
| deployment.go            | https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/controller/deployment/util/deployment_util.go#L536-L544        | func GetNewReplicaSet                   | Y       |
```

### Lifting const

Lift const `isNegativeErrorMsg` to `corevalidation.go  `:

```go
// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/apis/core/validation/validation.go#L59
const isNegativeErrorMsg string = apimachineryvalidation.IsNegativeErrorMsg
```

Added in `doc.go`:

```markdown
| lifted file              | source file                                                                                                                   | const/var/type/func                     | changed |
|--------------------------|-------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------|---------|
| corevalidation.go        | https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/apis/core/validation/validation.go#L59                         | const isNegativeErrorMsg                | N       |
```

### Lifting type

Lift type `Visitor` to `visitpod.go`:

```go
// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/api/v1/pod/util.go#L82-L83

// Visitor is called with each object name, and returns true if visiting should continue
type Visitor func(name string) (shouldContinue bool)
```

Added in `doc.go`:

```markdown
| lifted file              | source file                                                                                                                   | const/var/type/func                     | changed |
|--------------------------|-------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------|---------|
| visitpod.go              | https://github.com/kubernetes/kubernetes/blob/release-1.23/pkg/api/v1/pod/util.go#L82-L83                                     | type Visitor                            | N       |
```
