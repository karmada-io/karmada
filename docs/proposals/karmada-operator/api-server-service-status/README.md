---
title: Add API Server Service Information to `KarmadaStatus`
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"

creation-date: 2024-09-24

---

# Add API Server Service Information to `KarmadaStatus`

## Summary

This proposal aims to enhance `KarmadaStatus` by adding a new field that contains the name and port of the API server service for a Karmada control plane. This change will simplify the process of referencing an API server service,
eliminating the need to infer the service name and exposed client port. This is useful for higher-level operators that may need to perform additional tasks like creating an ingress resource to configure external traffic to the service once a Karmada instance has been provisioned.

## Motivation

When managing a Karmada instance, referencing its API server service (e.g., for creating an ingress resource) currently requires inferring the service name and exposed port from conventions. Relying on this method is brittle since it depends on internal implementation details that may change.

By including the API server service information directly in the `KarmadaStatus` field, operators can directly reference the service name and exposed port, improving reliability and simplifying cluster management.

### Goals

- Add a new field to `KarmadaStatus` to store the API server service information.
- Ensure the API server service information is accessible to operators or higher-level systems.

### Non-Goals

- Modify the process of how the Karmada API server service is created.
- Affect backward compatibility of the existing `KarmadaStatus`.

## Proposal

The proposal is to introduce a new field, `APIServerService`, in `KarmadaStatus` to capture the name and port of the Karmada API server service. This would provide a more reliable method for referencing the service when creating additional resources, like `Ingress` objects.


### User Stories

#### Story 1
As an operator, I want to provision an Ingress for the Karmada API server without needing to guess or infer the service name and port. As an example, when using the Nginx
ingress controller, to configure ingress traffic to the API server running in the host cluster, I need to create an ingress resource like the following:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: HTTPS
    nginx.ingress.kubernetes.io/ssl-passthrough: "true"
  name: demo-karmada-apiserver
  namespace: demo
spec:
  ingressClassName: nginx
  rules:
  - host: demo-demo.karmada.example.com
    http:
      paths:
      - backend:
          service:
            name: demo-karmada-apiserver
            port:
              number: 5443
        path: /
        pathType: Prefix
```


#### Story 2
As an administrator, I want to ensure the control plane's API server service is reliably discoverable by systems provisioning resources on top of Karmada.

### Risks and Mitigations

This change introduces minimal risk as it is backward-compatible. The new field will be optional, and systems that do not need it can safely ignore it. 
Testing should focus on ensuring that the Karmada operator correctly populates the new field in `KarmadaStatus`.

## Design Details
With the proposed changes, `KarmadaStatus` will have the following structure:

```go
// KarmadaStatus define the most recently observed status of the Karmada.
type KarmadaStatus struct {
	// APIServerService reports the location of the Karmada API server service which
	// can be used by third-party applications to discover the Karmada Service, e.g.
	// expose the service outside the cluster by Ingress.
	// +optional
	APIServerService *APIServerService `json:"apiServerService,omitempty"`
}

// APIServerService tells the location of Karmada API server service.
// Currently, it only includes the name of the service. The namespace
// of the service is the same as the namespace of the current Karmada object.
type APIServerService struct {
	// Name represents the name of the Karmada API Server service.
	// +required
	Name string `json:"name"`
}
```
The Karmada operator will need to be updated to populate the `APIServerService` field during its reconciliation process.