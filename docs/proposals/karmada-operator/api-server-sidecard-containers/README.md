---
title: Support for API Server Sidecar Containers in Karmada Operator
authors:
- "@jabellard"
reviewers:
- "@RainbowMango"
approvers:
- "@RainbowMango"

creation-date: 2025-02-07

---

# Support for API Server Sidecar Containers in Karmada Operator

## Summary

This proposal aims to enhance the Karmada operator by introducing support for configuring additional containers  for the Karmada API server component.
By allowing users to define additional containers, this feature unlocks new use cases such as integration with [Key Management Service (KMS)](https://kubernetes.io/docs/tasks/administer-cluster/kms-provider/) plugins for configuring encryption at rest. 
This enhancement provides greater flexibility while maintaining the declarative nature of the Karmada operator’s deployment model.

## Motivation

Currently, the Karmada operator provisions the API server as a standalone container within a pod. While the API server supports configurability via extra arguments, additional volumes, and volume mounts, it lacks built-in support for defining additional containers.
By introducing explicit support for additional containers, this feature enables:

- Seamless integration of auxiliary services such as KMS-based encryption providers.
- Greater deployment flexibility by enabling users to extend the API server’s functionality without modifying the core operator logic.

### Goals
- Introduce support for configuring sidecar containers for the Karmada API server.

### Non-Goals

- Introduce changes beyond the scope of what is needed to support additional containers for the Karmada API server component.

### API Changes

```go
// KarmadaAPIServer holds settings for the Karmada API server.
type KarmadaAPIServer struct {
    // Other, existing fields omitted for brevity
    
    // SidecarContainers specifies a list of sidecar containers to be deployed
    // within the Karmada API server pod.
    // This enables users to integrate auxiliary services such as KMS plugins for configuring encryption at rest.
    // +optional
    SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`
}
```

### User Stories

#### Story 1
As an infrastructure engineer, I want to integrate with my organization's KMS to configure encryption at rest for managed Karmada control planes.
When using a KMS-based encryption provider, a KMS plugin  is required to enable the integration. Once set up, the API server communicates to the plugin over a UNIX domain socket via gRPC. 
For that to work, when the API server is deployed as a pod as is the case when provisioned by the Karmada operator, the plugin has to run as a sidecar of the API server container.

##### Sample Configuration

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: encryption-config
  namespace: tenant1
data:
  encryption-config.yaml: |
    apiVersion: apiserver.config.k8s.io/v1
    kind: EncryptionConfiguration
    resources:
      - resources:
          - secrets
        providers:
          - kms:
              apiVersion: v2
              name: custom-kms-provider
              endpoint: unix:///var/run/kmsplugin/socket.sock
              cachesize: 1000
              timeout: 3s
          - identity: {}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kube-apiserver
  namespace: tenant1
  labels:
    app: kube-apiserver
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kube-apiserver
  template:
    metadata:
      labels:
        app: kube-apiserver
    spec:
      containers:
        - name: kube-apiserver
          image: k8s.gcr.io/kube-apiserver:v1.21.0
          command:
            - kube-apiserver
            - --encryption-provider-config=/etc/kubernetes/encryption-config.yaml
          volumeMounts:
            - name: encryption-config
              mountPath: /etc/kubernetes/encryption-config.yaml
              subPath: encryption-config.yaml
            - name: kms-socket
              mountPath: /var/run/kmsplugin
        - name: kms-plugin
          image: custom-kms-plugin:latest
          volumeMounts:
            - name: kms-socket
              mountPath: /var/run/kmsplugin
      volumes:
        - name: encryption-config
          configMap:
            name: encryption-config
        - name: kms-socket
          emptyDir: {}
```


## Design Details

During the reconciliation process, the Karmada operator will:

- Check if `SidecarContainers` is specified in the KarmadaAPIServer spec.
- If specified:
  - Append the defined containers to the API server pod specification.
- If not specified:
  - Maintain the current deployment behavior with only the API server container.
