---
title: Enhance `EcternalEtcd` Configuration to Support Retrieving etcd Client Credentials from a Kubernetes Secret
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
- "@zhzhuang-zju"
approvers:
- "@RainbowMango"
- "@zhzhuang-zju"

creation-date: 2024-07-26

---

# Enhance `EcternalEtcd` Configuration to Support Retrieving etcd Client Credentials from a Kubernetes Secret

## Summary

This proposal aims to enhance the `EternalEtcd` configuration in the Karmada API by introducing support for loading external etcd client credentials from a Kubernetes secret. 
This change will improve the security and manageability of sensitive data, such as TLS certificates and private keys by leveraging Kubernetes' native secret management capabilities.

## Motivation

The current implementation of the `ExternalEtcd` configuration requires the etcd client credentials, including the certificate private key, to be provided inline within the configuration. 
This approach poses security risks and can be cumbersome to manage, especially in CI/CD scenarios driven by GitOps. By enabling the ability of loading those credentials from a secret, we can ensure that sensitive data is stored securely and managed more efficiently. 
Additionally, this enhancement is very well-aligned with Kubernetes best practices for managing sensitive information and provides a more flexible and secure way to configure external etcd connections.

### Goals

- Add support for connecting to an external etcd cluster.
- Enable the use of Kubernetes secrets for storing and retrieving external etcd connection credentials.
- Improve the security of sensitive data by leveraging Kubernetes' native secret management capabilities.
- Simplify the management of external etcd configurations.

### Non-Goals

- Modifying the internal etcd configuration.
- Changing the existing inline configuration approach.

## Proposal

Introduce a new field, `SecretRef`, within the `ExternalEtcd` configuration to reference a Kubernetes secret that contains the necessary etcd connection credentials. 
The existing inline configuration fields will be preserved to maintain backward compatibility. However, given that they are currently unused, it would make sense to mark them as deprecated.

## Design Details

Introduce the new field `SecretRef` within the `ExternalEtcd` configuration:
```go
// ExternalEtcd describes an external etcd cluster.
type ExternalEtcd struct {
  // Other, existing fields
	
  
  // SecretRef references a Kubernetes secret containing the etcd connection credentials.
  // The secret must contain the following data keys:
  // ca.crt: The Certificate Authority (CA) certificate data.
  // tls.crt: The TLS certificate data.
  // tls.key: The TLS private key data.
  // +optional
  SecretRef *LocalSecretReference `json:"secretRef,omitempty"`
}
```

Type `LocalSecretReference` already exists in the API.

### Secret Structure

The Kubernetes secret referenced by `SecretRef` must contain the following data keys:

- `ca.crt`: The Certificate Authority (CA) certificate data.
- `tls.crt`: The TLS certificate data.
- `tls.key`: The TLS private key data.

Example:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tenant1-etcd-credentials
  namespace: tenant1
type: Opaque
data:
  ca.crt: <base64-encoded-data>
  tls.crt: <base64-encoded-data>
  tls.key: <base64-encoded-data>
```

### Control Plane Component Configurations

Today, the Karmada operator stores the in-cluster etcd client credentials in a secret. That secret is then used as the source of a volume that is mounted into control plane component containers 
that need to connect to etcd such as `karmada-apiserver`, `karmada-aggregated-apiserver` and `karmada-search`. The configuration for these components will be very similar when connecting to an external etcd cluster.