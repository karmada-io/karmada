---
title: Runtime value retention
authors:
- "@RainbowMango"
reviewers:
- "@TBD"
approvers:
- "@TBD"

creation-date: 2021-08-12

---

# Runtime value retention

## Summary

For now, Karmada keeps watching the propagated resources in member clusters to ensure the resource is always in desire
state.

In many cases, the controllers running in member clusters will make changes to the resource, such as:
- The Kubernetes will assign a `clusterIP` for `Service` with type `ClusterIP`.
- The Kubernetes will assign a `nodeName` for `Pod` in the scheduling phase.

When Karmada users make changes to the resource template, Karmada will update the resource against the member cluster,
but before the update, Karmada should `retain` the changes made by member cluster controllers.

Karmada has implemented different `retain` methods for common resources, such as `Service`, `Pod`, `ServiceAccount`, and so on.
The `retain` feature works for most cases, but still has disadvantages:
- The `retain` methods are built-in and users can't customize them.
- No `retain` method for custom resources(declared by `CustomResourceDefinition`).

This proposal aims to provide a strategy to customize the `retain` methods for any kind of resource.

## Motivation

Nowadays, Kubernetes has provided dozens of resources, even though we don't need to implement the `retain` method for
each of them, but it is still a heavy burden to maintain, it's hard to meet different kinds of expectations.

On the other hand, we can't define the `retain` method for custom resources as you even don't know the fields in it.

### Goals

- Provide a strategy to support customize `retain` methods for any kind of resources.
 * Support customize built-in `retain` methods of Kubernetes resources.
 * Support customize `retain` method for custom resources(declared by CustomResourceDefinition).
- Provide a general mechanism to customize karmada controller behaviors.
 *  The mechanism can be reused by other customized requirements.

### Non-Goals

- Define specific `retain` methods for Kubernetes resources.
- Deprecate the built-in `retain` methods.
 * We should maintain built-in `retain` for the well-known resources to simplify user configuration.

## Proposal

### User Stories

#### As a user, I want to customize the built-in retain method because it can't fulfill my requirement.

The built-in retain methods aim to provide a default retain method for well-known resources, they are suitable for
most situations, but can't guarantee to meet all user's needs.

On the other hand, we usually maintain built-in retain methods for a preferred version of the resource, such as `Deployment`,
The `apps/v1` retain method might not suitable for `apps/v1beta1`.

#### As a user, I want to customize the retain method for my CRD resources.

Nowadays, it's getting pretty common that people extend Kubernetes by CRD, at the meanwhile people might implement
their controllers and the controllers probably make changes to these CRs when reconciling the changes.
In this case, people should define the retain method for the CR, otherwise, it will be a conflict when syncing changes
make on the Karmada control plane.(Karmada update--> controller update them back --> Karmada update ... endless update loop)

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

## Design Details

### Retention configuration
We propose a new configuration for customize the retention rules:
```golang
// RuntimeValueRetention represents a customized retention method for specific API resource.
type RuntimeValueRetention struct {
	// APIVersion represents the API version of the target resources.
	// +required
	APIVersion string `json:"apiVersion"`

	// Kind represents the Kind of the target resources.
	// +required
	Kind string `json:"kind"`

	// RetentionLua holds the Lua script that is used to retain runtime values to the desired specification.
	// The script should implement a function just like:
	//
	// function Retain(desiredObj, runtimeObj)
	//     desiredObj.spec.fieldFoo = runtimeObj.spec.fieldFoo
	//     return desiredObj
	// end
	//
	// RetentionLua only holds the function body part, take the `desiredObj` and `runtimeObj` as the function parameters or
	// global parameters, and finally retains a retained specification.
	// +optional
	RetentionLua string `json:"health.lua,omitempty"`
}
```

Each `RuntimeValueRetention` customizes one API resource referencing by `APIVersion` and `Kind`.
The `RetentionLua` holds a fragment of [Lua](https://www.lua.org/) script which is used to implement the retention logic.

**Note:** The `Lua` might not be the best choice, another considered approach is [cue](https://github.com/cue-lang/cue/tree/master/doc/tutorial/kubernetes) as mentioned by @pigletfly at [this discussion](https://github.com/karmada-io/karmada/pull/592#issuecomment-895759570),
we have to do some investigation. But we also reserve the possibility that uses more than one script, people can select by themselves.

### Using ConfigMap to deploy the configuration

We propose to use `ConfigMap` to hold the configuration, and reserve a dedicated name `karmada-config`. The slices of
`RuntimeValueRetention` will be resident in the field `retention.customizations`, such as:
```yaml
data:
  retention.customizations: |
    <one or more RuntimeValueRetention>
```

The `karmada-controller-manager` will detect the `karmada-config` at the start phase. That means people have to deploy
`karmada-config` first before starting the `karmada-controller-manager`, otherwise `karmada-controller-manager` should be
restarted to load the `karmada-config`.

**Note:** Technically speaking, we can discover the `karmada-config` in real-time.

### Bundle well-known custom resources

There are a lot of famous projects that defined CRD in the Kubernetes ecosystem, for these widely adopted
CRDs, we can bundle them into Karmada, just like the `built-in` retain methods.

This part of the design will continue later.

### Test Plan

- Propose E2E test cases according to user stories above:
 * Test we can customize the built-in method
 * Test we can customize custom resource

- Propose a tool that people can test the script.

## Alternatives

### Define customization as an API resource

Instead of using a `ConfigMap`, we could also use an API resource. In this case, we should define a resource type for the configuration. But will be a bit heavy to maintain an API.