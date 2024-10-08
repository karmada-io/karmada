---
title: Karmadactl Supports Configuration File Method for Production-Grade Installation and Deployment
authors:
  - "@tiansuo114"
reviewers:
  - "@liangyuanpeng"
  - "@zhzhuang-zju"
  - "@XiShanYongYe-Chang"

approvers:
  - "@liangyuanpeng"
  - "@zhzhuang-zju"
  - "@XiShanYongYe-Chang"

creation-date: 2024-07-29
---

# Karmadactl Supports Configuration File Method for Production-Grade Installation and Deployment

## Summary

The `karmadactl init` command is expected to support loading deployment parameters from a configuration file, simplifying the process of production-grade installation and reducing the complexity of managing numerous command-line flags. This enhancement allows users to streamline their multi-cluster management setups by utilizing predefined configuration files, similar to Kubernetes' `kubeadm`, making it easier to customize and maintain consistent deployments across environments.

## Motivation

As Karmada is widely used in multi-cluster management, the current `karmadactl init` command has accumulated dozens of command-line parameters. This not only increases the learning curve for users but also reduces the convenience of usage. With the ongoing development of the community, the number and complexity of these parameters are likely to continue growing, further impacting the user experience.

### Goals

- Provide the ability for `karmadactl` to construct `init` command parameters from a configuration file.

### Non-Goals
- Add additional commands for karmadactl config, such as karmadactl config --init-config to add the ability to output default initialization config to the command

## Proposal

I believe that the flags in `karmada init` can mainly be divided into the following categories:

- Certificate-related Options
- Etcd-related Options
- Karmada Control Plane-related Options
- Kubernetes-related Options
- Networking Options
- Image Options

Therefore, I think the configuration file can be designed to focus on these parts, structuring the YAML accordingly, and incorporating the existing configuration items into them. Additionally, similar to the commonly used `Extra Arg` field in Kubernetes structures, an extra field can be added to store other general fields. When designing the fields, it is advisable to refer to the field names in the kubeadm configuration file as much as possible, to facilitate user familiarity and quick adoption.

### User Stories (Optional)

#### User Story 1: Simplified Cluster Deployment

As a cloud platform operations engineer, Mr. Wang is responsible for managing multiple Karmada clusters for his company. Due to the large number of parameters that need to be set for each Karmada deployment, he finds the command line too complex and prone to errors. To improve deployment efficiency, Mr. Wang wants to use a predefined configuration file and streamline the cluster deployment process using the `karmadactl init --config` command.

Now that Karmada supports loading deployment parameters from a configuration file, Mr. Wang only needs to create a configuration file and apply the parameters, reducing the complexity of manual command-line input and ensuring consistent deployments across different environments. This way, Mr. Wang can easily reuse the same configuration file, significantly simplifying his workflow.

### Notes/Constraints/Caveats (Optional)

### Risks and Mitigations

## Design Details

<!--
This section should contain enough information that the specifics of your
change are understandable. This may include API specs (though not always
required) or even code snippets. If there's any ambiguity about HOW your
proposal will be implemented, this is the place to discuss them.
-->

### CLI flags changes

This proposal proposes new flags `config`in `karmadactl init` flag set.

| name   | shorthand | default | usage                             |
|--------|:---------:|---------|-----------------------------------|
| config |     /     | ""      | Karmada init --config=<file path> |

With these flag, we will:

- When the user has a karmadactl init file, they can use the `--config` flag to specify that `karmadactl init` should read the configuration from the specified config file. Additionally, external cmd flags can still be used. In the design, if there are overlapping parts between the two, the configuration file configuration items will take higher priority.

#### Example Performance:

1. ##### Using a Default Configuration File for Karmada Initialization

   Assume that the user has prepared a default `karmada-init.yaml` configuration file that contains basic parameters for Karmada deployment, such as control plane components, etcd configuration, and image repositories. The user can start the Karmada initialization process with the following command:

   ```
   karmadactl init --config karmada-init.yaml
   ```

   yaml example

   ```
   apiVersion: config.karmada.io/v1alpha1
   kind: InitConfiguration
   general:
     namespace: "karmada-system"
     kubeConfigPath: "/etc/karmada/kubeconfig"
     kubeImageTag: "v1.21.0"
     privateImageRegistry: "registry.k8s.io"
     port: 32443
   karmadaControlPlane:
     apiServer:
       replicas: 3
   etcd:
     local:
       replicas: 3
   ```

   If we configure without using a configuration file and rely solely on command-line flags, the command would look like this:

   ```
   karmadactl init \
     --namespace=karmada-system \
     --kubeconfig=/etc/karmada/kubeconfig \
     --kube-image-tag=v1.21.0 \
     --private-image-registry=registry.k8s.io \
     --port=32443 \
     --karmada-apiserver-replicas=3 \
     --etcd-replicas=3
   ```

2. ##### Specifying a Private Image Registry for Offline Deployment

   If a user is deploying Karmada in an offline environment, they may need to pull images from an internal private image registry. In this case, the user can specify the `privateImageRegistry` parameter in the configuration file and load it using the `--config` option.

   yaml example

   ```
   apiVersion: config.karmada.io/v1alpha1
   kind: InitConfiguration
   general:
     namespace: "karmada-system"
     kubeConfigPath: "/etc/karmada/kubeconfig"
     privateImageRegistry: "registry.company.com"
   karmadaControlPlane:
     apiServer:
       replicas: 3
   etcd:
     local:
       replicas: 3
   ```

   `karmadactl init` will pull all required Karmada images from the private image registry `registry.company.com`, ensuring that the deployment can proceed smoothly even in an offline or restricted network environment.

### Implementation of Loading Configuration File

Based on my classification of dozens of flags in the current `karmadactl init` command, I have designed the following data structure:

```go
// KarmadaInitConfig defines the configuration for initializing Karmada
type KarmadaInitConfig struct {
    metav1.TypeMeta   `json:",inline" yaml:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

    // Spec defines the desired state for initializing Karmada
    // +optional
    Spec KarmadaInitSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// KarmadaInitSpec is the specification part of KarmadaInitConfig, containing all configurable options
type KarmadaInitSpec struct {
    // Certificates configures the certificate information required by Karmada
    // +optional
    Certificates Certificates `json:"certificates,omitempty" yaml:"certificates,omitempty"`

    // Etcd configures the information of the Etcd cluster
    // +optional
    Etcd Etcd `json:"etcd,omitempty" yaml:"etcd,omitempty"`

    // ExternalEtcd configures the information of an external Etcd cluster
    // +optional
    ExternalEtcd *ExternalEtcd `json:"externalEtcd,omitempty" yaml:"externalEtcd,omitempty"`

    // HostCluster configures the information of the host cluster
    // +optional
    HostCluster HostCluster `json:"hostCluster,omitempty" yaml:"hostCluster,omitempty"`

    // Images configures image-related information
    // +optional
    Images Images `json:"images,omitempty" yaml:"images,omitempty"`

    // Components configures information about Karmada components
    // +optional
    Components KarmadaComponents `json:"components,omitempty" yaml:"components,omitempty"`

    // Networking configures network-related information
    // +optional
    Networking Networking `json:"networking,omitempty" yaml:"networking,omitempty"`

    // KarmadaDataPath configures the data directory for Karmada
    // +optional
    KarmadaDataPath string `json:"karmadaDataPath,omitempty" yaml:"karmadaDataPath,omitempty"`

    // KarmadaPkiPath configures the PKI directory for Karmada
    // +optional
    KarmadaPkiPath string `json:"karmadaPkiPath,omitempty" yaml:"karmadaPkiPath,omitempty"`

    // WaitComponentReadyTimeout configures the timeout (in seconds) for waiting for components to be ready
    // +optional
    WaitComponentReadyTimeout int `json:"waitComponentReadyTimeout,omitempty" yaml:"waitComponentReadyTimeout,omitempty"`
}

// Certificates defines the configuration related to certificates
type Certificates struct {
    // CACertFile is the path to the root CA certificate file
    // +optional
    CACertFile string `json:"caCertFile,omitempty" yaml:"caCertFile,omitempty"`

    // CAKeyFile is the path to the root CA key file
    // +optional
    CAKeyFile string `json:"caKeyFile,omitempty" yaml:"caKeyFile,omitempty"`

    // ExternalDNS is the list of external DNS names for the certificate
    // +optional
    ExternalDNS []string `json:"externalDNS,omitempty" yaml:"externalDNS,omitempty"`

    // ExternalIP is the list of external IPs for the certificate
    // +optional
    ExternalIP []string `json:"externalIP,omitempty" yaml:"externalIP,omitempty"`

    // ValidityPeriod is the validity period of the certificate
    // +optional
    ValidityPeriod metav1.Duration `json:"validityPeriod,omitempty" yaml:"validityPeriod,omitempty"`
}

// Etcd defines the configuration of the Etcd cluster
type Etcd struct {
    // Local indicates using a local Etcd cluster
    // +optional
    Local *LocalEtcd `json:"local,omitempty" yaml:"local,omitempty"`

    // External indicates using an external Etcd cluster
    // +optional
    External *ExternalEtcd `json:"external,omitempty" yaml:"external,omitempty"`
}

// LocalEtcd defines the configuration of a local Etcd cluster
type LocalEtcd struct {
    // CommonSettings contains common settings like image and resources
    CommonSettings `json:",inline" yaml:",inline"`

    // DataPath is the data storage path for Etcd
    // +optional
    DataPath string `json:"dataPath,omitempty" yaml:"dataPath,omitempty"`

    // InitImage is the image for the Etcd init container
    // +optional
    InitImage Image `json:"initImage,omitempty" yaml:"initImage,omitempty"`

    // NodeSelectorLabels are the node selector labels for the Etcd pods
    // +optional
    NodeSelectorLabels map[string]string `json:"nodeSelectorLabels,omitempty" yaml:"nodeSelectorLabels,omitempty"`

    // PVCSize is the size of the PersistentVolumeClaim for Etcd
    // +optional
    PVCSize string `json:"pvcSize,omitempty" yaml:"pvcSize,omitempty"`

    // Replicas is the number of replicas in the Etcd cluster
    // +optional
    Replicas int32 `json:"replicas,omitempty" yaml:"replicas,omitempty"`

    // StorageMode is the storage mode for Etcd (e.g., emptyDir, hostPath, PVC)
    // +optional
    StorageMode string `json:"storageMode,omitempty" yaml:"storageMode,omitempty"`
}

// ExternalEtcd defines the configuration of an external Etcd cluster
type ExternalEtcd struct {
    // Endpoints are the server addresses of the external Etcd cluster
    // +required
    Endpoints []string `json:"endpoints" yaml:"endpoints"`

    // CAFile is the path to the CA certificate for the external Etcd cluster
    // +optional
    CAFile string `json:"caFile,omitempty" yaml:"caFile,omitempty"`

    // CertFile is the path to the client certificate for the external Etcd cluster
    // +optional
    CertFile string `json:"certFile,omitempty" yaml:"certFile,omitempty"`

    // KeyFile is the path to the client key for the external Etcd cluster
    // +optional
    KeyFile string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`

    // KeyPrefix is the key prefix used in the external Etcd cluster
    // +optional
    KeyPrefix string `json:"keyPrefix,omitempty" yaml:"keyPrefix,omitempty"`
}

// HostCluster defines the configuration of the host cluster
type HostCluster struct {
    // APIEndpoint is the API server address of the host cluster
    // +optional
    APIEndpoint string `json:"apiEndpoint,omitempty" yaml:"apiEndpoint,omitempty"`

    // Kubeconfig is the path to the kubeconfig file for the host cluster
    // +optional
    Kubeconfig string `json:"kubeconfig,omitempty" yaml:"kubeconfig,omitempty"`

    // Context is the context name in the kubeconfig for the host cluster
    // +optional
    Context string `json:"context,omitempty" yaml:"context,omitempty"`

    // Domain is the domain name of the host cluster
    // +optional
    Domain string `json:"domain,omitempty" yaml:"domain,omitempty"`

    // SecretRef refers to the credentials needed to access the host cluster
    // +optional
    SecretRef *LocalSecretReference `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
}

// Images defines the configuration related to images
type Images struct {
    // ImagePullPolicy is the pull policy for images
    // +optional
    ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`

    // ImagePullSecrets are the secrets used for pulling images
    // +optional
    ImagePullSecrets []string `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`

    // KubeImageMirrorCountry is the country code for the Kubernetes image mirror
    // +optional
    KubeImageMirrorCountry string `json:"kubeImageMirrorCountry,omitempty" yaml:"kubeImageMirrorCountry,omitempty"`

    // KubeImageRegistry is the registry for Kubernetes images
    // +optional
    KubeImageRegistry string `json:"kubeImageRegistry,omitempty" yaml:"kubeImageRegistry,omitempty"`

    // KubeImageTag is the tag for Kubernetes images
    // +optional
    KubeImageTag string `json:"kubeImageTag,omitempty" yaml:"kubeImageTag,omitempty"`

    // PrivateRegistry is the private image registry
    // +optional
    PrivateRegistry *ImageRegistry `json:"privateRegistry,omitempty" yaml:"privateRegistry,omitempty"`
}

// KarmadaComponents defines the configuration for all Karmada components
type KarmadaComponents struct {
    // Etcd is the configuration for the Etcd component
    // +optional
    Etcd *Etcd `json:"etcd,omitempty" yaml:"etcd,omitempty"`

    // KarmadaAPIServer is the configuration for the Karmada API Server
    // +optional
    KarmadaAPIServer *KarmadaAPIServer `json:"karmadaAPIServer,omitempty" yaml:"karmadaAPIServer,omitempty"`

    // KarmadaAggregatedAPIServer is the configuration for the Karmada Aggregated API Server
    // +optional
    KarmadaAggregatedAPIServer *KarmadaAggregatedAPIServer `json:"karmadaAggregatedAPIServer,omitempty" yaml:"karmadaAggregatedAPIServer,omitempty"`

    // KubeControllerManager is the configuration for the Kube Controller Manager
    // +optional
    KubeControllerManager *KubeControllerManager `json:"kubeControllerManager,omitempty" yaml:"kubeControllerManager,omitempty"`

    // KarmadaControllerManager is the configuration for the Karmada Controller Manager
    // +optional
    KarmadaControllerManager *KarmadaControllerManager `json:"karmadaControllerManager,omitempty" yaml:"karmadaControllerManager,omitempty"`

    // KarmadaScheduler is the configuration for the Karmada Scheduler
    // +optional
    KarmadaScheduler *KarmadaScheduler `json:"karmadaScheduler,omitempty" yaml:"karmadaScheduler,omitempty"`

    // KarmadaWebhook is the configuration for the Karmada Webhook
    // +optional
    KarmadaWebhook *KarmadaWebhook `json:"karmadaWebhook,omitempty" yaml:"karmadaWebhook,omitempty"`
}

// Networking defines network-related configuration
type Networking struct {
    // Namespace is the Kubernetes namespace where Karmada is deployed
    // +optional
    Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

    // Port is the port number for the Karmada API Server
    // +optional
    Port int32 `json:"port,omitempty" yaml:"port,omitempty"`
}

// CommonSettings defines common settings for components
type CommonSettings struct {
    // Image specifies the image to use for the component
    Image `json:",inline" yaml:",inline"`

    // Replicas is the number of replicas for the component
    // +optional
    Replicas *int32 `json:"replicas,omitempty" yaml:"replicas,omitempty"`

    // Resources defines resource requests and limits for the component
    // +optional
    Resources corev1.ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`

    // NodeSelector defines node selection constraints
    // +optional
    NodeSelector map[string]string `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`

    // Tolerations define pod tolerations
    // +optional
    Tolerations []corev1.Toleration `json:"tolerations,omitempty" yaml:"tolerations,omitempty"`

    // Affinity defines pod affinity rules
    // +optional
    Affinity *corev1.Affinity `json:"affinity,omitempty" yaml:"affinity,omitempty"`
}

// Image defines image information
type Image struct {
    // ImageRepository is the repository for the image
    // +optional
    ImageRepository string `json:"imageRepository,omitempty" yaml:"imageRepository,omitempty"`

    // ImageTag is the tag for the image
    // +optional
    ImageTag string `json:"imageTag,omitempty" yaml:"imageTag,omitempty"`
}

// KarmadaAPIServer defines the configuration for the Karmada API Server
type KarmadaAPIServer struct {
    CommonSettings `json:",inline" yaml:",inline"`

    // AdvertiseAddress is the address advertised by the API server
    // +optional
    AdvertiseAddress string `json:"advertiseAddress,omitempty" yaml:"advertiseAddress,omitempty"`

    // ServiceType is the type of service for the API server (e.g., ClusterIP, NodePort)
    // +optional
    ServiceType corev1.ServiceType `json:"serviceType,omitempty" yaml:"serviceType,omitempty"`

    // ServiceAnnotations are annotations added to the API server service
    // +optional
    ServiceAnnotations map[string]string `json:"serviceAnnotations,omitempty" yaml:"serviceAnnotations,omitempty"`
}

// KarmadaAggregatedAPIServer defines the configuration for the Karmada Aggregated API Server
type KarmadaAggregatedAPIServer struct {
    CommonSettings `json:",inline" yaml:",inline"`
}

// KubeControllerManager defines the configuration for the Kube Controller Manager
type KubeControllerManager struct {
    CommonSettings `json:",inline" yaml:",inline"`
}

// KarmadaControllerManager defines the configuration for the Karmada Controller Manager
type KarmadaControllerManager struct {
    CommonSettings `json:",inline" yaml:",inline"`
}

// KarmadaScheduler defines the configuration for the Karmada Scheduler
type KarmadaScheduler struct {
    CommonSettings `json:",inline" yaml:",inline"`
}

// KarmadaWebhook defines the configuration for the Karmada Webhook
type KarmadaWebhook struct {
    CommonSettings `json:",inline" yaml:",inline"`
}

// LocalSecretReference is a reference to a secret within the same namespace
type LocalSecretReference struct {
    // Name is the name of the referenced secret
    Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

// ImageRegistry represents an image registry
type ImageRegistry struct {
    // Registry is the hostname of the image registry
    // +required
    Registry string `json:"registry" yaml:"registry"`
}
```

After reading the logic, the information in the configuration file will be placed into the `InitConfiguration` structure, and then the fields will be filled into the `CommandInitOption` structure used to control the `karmadactl init` process. This way, the deployment of the Karmada control plane can proceed according to the original logic.

For the design of the configuration file, I think it can be designed similarly to the config structure in `kubeadm init`. I will provide a YAML file example below. To provide users with a configuration file, I think we can prepare a default configuration file example and make it a sub-command in `karmadactl config`. When the feature design is complete and the official documentation is updated, an example can also be demonstrated in the official documentation.

Example configuration file:

```yaml
apiVersion: v1alpha1
kind: KarmadaInitConfig
metadata:
  name: karmada-init
spec:
  certificates:
    caCertFile: "/etc/karmada/pki/ca.crt"
    caKeyFile: "/etc/karmada/pki/ca.key"
    externalDNS:
      - "localhost"
      - "example.com"
    externalIP:
      - "192.168.1.2"
      - "172.16.1.2"
    validityPeriod: "8760h0m0s"
  etcd:
    local:
      image:
        imageRepository: "karmada/etcd"
        imageTag: "latest"
      dataPath: "/var/lib/karmada-etcd"
      initImage:
        imageRepository: "alpine"
        imageTag: "3.19.1"
      nodeSelectorLabels:
        karmada.io/etcd: "true"
      pvcSize: "5Gi"
      replicas: 3
      storageMode: "PVC"
  hostCluster:
    apiEndpoint: "https://kubernetes.example.com"
    kubeconfig: "/root/.kube/config"
    context: "karmada-host"
    domain: "cluster.local"
  images:
    imagePullPolicy: "IfNotPresent"
    imagePullSecrets:
      - "PullSecret1"
      - "PullSecret2"
    kubeImageMirrorCountry: "cn"
    kubeImageRegistry: "registry.cn-hangzhou.aliyuncs.com/google_containers"
    kubeImageTag: "v1.29.6"
    privateRegistry:
      registry: "my.private.registry"
  components:
    karmadaAPIServer:
      image:
        imageRepository: "karmada/kube-apiserver"
        imageTag: "v1.29.6"
      replicas: 1
      advertiseAddress: "192.168.1.100"
      serviceType: "NodePort"
    karmadaAggregatedAPIServer:
      image:
        imageRepository: "karmada/karmada-aggregated-apiserver"
        imageTag: "v0.0.0-master"
      replicas: 1
    kubeControllerManager:
      image:
        imageRepository: "karmada/kube-controller-manager"
        imageTag: "v1.29.6"
      replicas: 1
    karmadaControllerManager:
      image:
        imageRepository: "karmada/karmada-controller-manager"
        imageTag: "v0.0.0-master"
      replicas: 1
    karmadaScheduler:
      image:
        imageRepository: "karmada/karmada-scheduler"
        imageTag: "v0.0.0-master"
      replicas: 1
    karmadaWebhook:
      image:
        imageRepository: "karmada/karmada-webhook"
        imageTag: "v0.0.0-master"
      replicas: 1
  networking:
    namespace: "karmada-system"
    port: 32443
  storage:
    storageClassName: "fast-storage"
  karmadaDataPath: "/etc/karmada"
  karmadaPkiPath: "/etc/karmada/pki"
  waitComponentReadyTimeout: 120
```

## Alternatives

<!--
What other approaches did you consider, and why did you rule them out? These do
not need to be as detailed as the proposal, but should include enough
information to express the idea and why it was not acceptable.
-->

<!--
Note: This is a simplified version of kubernetes enhancement proposal template.
https://github.com/kubernetes/enhancements/tree/3317d4cb548c396a430d1c1ac6625226018adf6a/keps/NNNN-kep-template
-->