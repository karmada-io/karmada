---
title: Karmadactl Supports Configuration File Method for Production-Grade Installation and Deployment
authors:
  - "@tiansuo114"
reviewers:
  - "@liangyuanpeng"

approvers:
  - "@liangyuanpeng"

creation-date: 2024-07-29
---

# Karmadactl Supports Configuration File Method for Production-Grade Installation and Deployment

## Summary

Additionally, `karmadactl init` requires the use of multiple container images during deployment, and users currently cannot easily obtain a list of the container images used. Therefore, we plan to add an `images list` subcommand to allow users to print the list of container images used during the `karmadactl init` process to the console.

## Motivation

As Karmada is widely used in multi-cluster management, the current `karmadactl init` command has accumulated dozens of command-line parameters. This not only increases the learning curve for users but also reduces the convenience of usage. With the ongoing development of the community, the number and complexity of these parameters are likely to continue growing, further impacting the user experience.

Additionally, during the deployment process of `karmadactl init`, multiple container images are needed. Users may want to download the corresponding images in advance to save deployment time, but currently, there is no easy way for users to obtain the list of required container images.

### Goals

- Provide the ability for `karmadactl` to construct `init` command parameters from a configuration file.
- Implement the `karmadactl config images list` command to provide a list of container images.

### Non-Goals
- 增添karmadactl config的其他命令,如karmadactl config --init-config来为命令添加输出默认初始化config的能力
- 通过扩展karmadactl config image的cmd flag,增添为karmadactl addon部署中所需的镜像列表进行展示的能力

## Proposal

I believe that the flags in `karmada init` can mainly be divided into the following categories:

- Certificate-related Options
- Etcd-related Options
- Karmada Control Plane-related Options
- Kubernetes-related Options
- Networking Options
- Image Options

Therefore, I think the configuration file can be designed to focus on these parts, structuring the YAML accordingly, and incorporating the existing configuration items into them. Additionally, similar to the commonly used `Extra Arg` field in Kubernetes structures, an extra field can be added to store other general fields. When designing the fields, it is advisable to refer to the field names in the kubeadm configuration file as much as possible, to facilitate user familiarity and quick adoption.

After completing the design for reading config files in `karmada init`, we can extend the functionality of the `karmadactl config image list` command to read the required image configuration from the config file by reusing the config file reading method.

### User Stories (Optional)

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

| name   | shorthand | default | usage                  |
| ------ | :-------: | ------- | ---------------------- |
| config |     /     | ""      | Karmada init file path |

With these flag, we will:

- When the user has a karmadactl init file, they can use the `--config` flag to specify that `karmadactl init` should read the configuration from the specified config file. Additionally, external cmd flags can still be used. In the design, if there are overlapping parts between the two, the configuration file configuration items will take higher priority.


### Adding New Commands

#### karmadactl config image list

- description:Shows information about the images required for Karmada deployment.
- usage scope:Default image and configure the specified image
- image-related flag set:

| name                      | shorthand | default | usage                                                        |
| ------------------------- | --------- | ------- | ------------------------------------------------------------ |
| private-image-registry    | /         | ""      | Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios. |
| kube-image-mirror-country | /         | ""      | Country code of the kube image registry to be used. For Chinese mainland users, set it to cn |

- performance：
  1. When the `private-image-registry` and `kube-image-mirror-country` flags are empty, the `list` command prints the images required for deploying Karmada under default settings, using the default image repository `registry.k8s.io`.
  2. When the `private-image-registry` is not empty, and the `kube-image-mirror-country` flag is empty, the `list` command prints the addresses of the images required for Karmada under the user-specified image repository link.
  3. When `kube-image-mirror-country` is set to `cn`, the `list` command prints the download links for the images from the default `cn` image address set in `karmadactl`, `registry.cn-hangzhou.aliyuncs.com/google_containers`. For `global` or other addresses not listed in the system, the behavior is similar to that described in point 1, using the default image repository `registry.k8s.io`.
  4. When both `private-image-registry` and `kube-image-mirror-country` flags are not empty, it is recommended to use the `private-image-registry` specified by the user, as described in point 2. Alternatively, it should return an error message. This behavior might require further discussion



### Implementation of Reading Configuration File

Based on my classification of dozens of commands in the current `karmadactl init` command, I have designed the following data structure for reading:

```go
// InitConfiguration holds the configuration for initializing a Kubernetes cluster.
type InitConfiguration struct {
	metav1.TypeMeta json:",inline"

	// GeneralConfig contains the common configuration at initialization.
	GeneralConfig GeneralConfig yaml:"general"

	// CertificateConfig contains configurations related to certificates.
	CertificateConfig CertificateConfig yaml:"certificate"

	// EtcdConfig contains configuration for etcd.
	EtcdConfig EtcdConfig yaml:"etcd"

	// KarmadaControlPlaneConfig contains control plane configurations.
	KarmadaControlPlaneConfig KarmadaControlPlaneConfig yaml:"karmadaControlPlane"

	// ImageConfig contains image-related configurations.
	ImageConfig ImageConfig yaml:"image"
}

// GeneralConfig contains general configuration parameters.
type GeneralConfig struct {
	Namespace                 string yaml:"namespace"
	KubeConfigPath            string yaml:"kubeConfigPath"
	KubeImageTag              string yaml:"kubeImageTag"
	KubeContextName           string yaml:"context"
	KubeStorageClassesName    string yaml:"storageClassesName"
	PrivateImageRegistry      string yaml:"privateImageRegistry"
	WaitComponentReadyTimeout int    yaml:"waitComponentReadyTimeout"
	Port                      int    yaml:"port"
	StorageClassesName        string yaml:"storageClassesName"
}

// CertificateConfig contains certificate-related configuration.
type CertificateConfig struct {
	ExternalDNS    []string yaml:"externalDNS"
	ExternalIP     []string yaml:"externalIP"
	ValidityPeriod string   yaml:"validityPeriod"
	CaCertFile     string   yaml:"caCertFile"
	CaCertKeyFile  string   yaml:"caCertKeyFile"
	ExtraArgs      []Arg    yaml:"extraArgs"
}

// EtcdConfig contains etcd configuration parameters.
type EtcdConfig struct {
	Local    *LocalEtcd    json:"local,omitempty"
	External *ExternalEtcd json:"external,omitempty"
}

// LocalEtcd contains configuration for a local etcd instance.
type LocalEtcd struct {
	Image              string yaml:"image"
	InitImage          string yaml:"initImage"
	DataDir            string yaml:"dataDir"
	PVCSize            string yaml:"pvcSize"
	NodeSelectorLabels map[string]string yaml:"nodeSelectorLabels"
	StorageMode        string yaml:"storageMode"
	Replicas           int32  yaml:"replicas"
	ExtraArgs          []Arg  yaml:"extraArgs"
}

// ExternalEtcd contains configuration for connecting to an external etcd cluster.
type ExternalEtcd struct {
	CAPath   string yaml:"caPath"
	CertPath string yaml:"certPath"
	KeyPath  string yaml:"keyPath"
	Servers  string yaml:"servers"
	Prefix   string yaml:"prefix"
	ExtraArgs        []Arg  yaml:"extraArgs"
}

// KarmadaControlPlaneConfig contains configuration for the control plane components.
type KarmadaControlPlaneConfig struct {
	APIServer                   APIServerConfig             yaml:"apiServer"
	ControllerManager           ControllerManagerConfig     yaml:"controllerManager"
	Scheduler                   SchedulerConfig             yaml:"scheduler"
	Webhook                     WebhookConfig               yaml:"webhook"
	AggregatedAPIServerConfig   AggregatedAPIServerConfig   yaml:"aggregatedAPIServerConfig"
	KubeControllerManagerConfig KubeControllerManagerConfig yaml:"kubeControllerManagerConfig"
	DataPath                    string                      yaml:"dataPath"
	PkiPath                     string                      yaml:"pkiPath"
	CRDs                        string                      yaml:"crds"
	HostClusterDomain           string                      yaml:"hostClusterDomain"
}

// APIServerConfig contains configuration for the API server.
type APIServerConfig struct {
	Image            string yaml:"image"
	AdvertiseAddress string yaml:"advertiseAddress"
	Replicas         int32  yaml:"replicas"
	NodePort         int32  yaml:"nodePort"
	ExtraArgs        []Arg  yaml:"extraArgs"
}

// ControllerManagerConfig contains configuration for the controller manager.
type ControllerManagerConfig struct {
	Image     string yaml:"image"
	Replicas  int32  yaml:"replicas"
	ExtraArgs []Arg  yaml:"extraArgs"
}

// SchedulerConfig contains configuration for the scheduler.
type SchedulerConfig struct {
	Image     string yaml:"image"
	Replicas  int32  yaml:"replicas"
	ExtraArgs []Arg  yaml:"extraArgs"
}

// WebhookConfig contains configuration for the webhook.
type WebhookConfig struct {
	Image     string yaml:"image"
	Replicas  int32  yaml:"replicas"
	ExtraArgs []Arg  yaml:"extraArgs"
}

type AggregatedAPIServerConfig struct {
	Image     string yaml:"image"
	Replicas  int32  yaml:"replicas"
	ExtraArgs []Arg  yaml:"extraArgs"
}

type KubeControllerManagerConfig struct {
	Image     string yaml:"image"
	Replicas  int32  yaml:"replicas"
	ExtraArgs []Arg  yaml:"extraArgs"
}

// ImageConfig contains configuration for images used in the cluster.
type ImageConfig struct {
	KubeImageTag           string   yaml:"kubeImageTag"
	KubeImageRegistry      string   yaml:"kubeImageRegistry"
	KubeImageMirrorCountry string   yaml:"kubeImageMirrorCountry"
	ImagePullPolicy        string   yaml:"imagePullPolicy"
	ImagePullSecrets       []string yaml:"imagePullSecrets"
}

// Arg represents a name-value pair argument.
type Arg struct {
	Name  string yaml:"name"
	Value string yaml:"value"
}
```

After reading the logic, the information in the configuration file will be placed into the `InitConfiguration` structure, and then the fields will be filled into the `CommandInitOption` structure used to control the `karmadactl init` process. This way, the deployment of the Karmada control plane can proceed according to the original logic.

For the design of the configuration file, I think it can be designed similarly to the config structure in `kubeadm init`. I will provide a YAML file example below. To provide users with a configuration file, I think we can prepare a default configuration file example and make it a sub-command in `karmadactl config`. When the feature design is complete and the official documentation is updated, an example can also be demonstrated in the official documentation.

Example configuration file:

```yaml
apiVersion: config.karmada.io/v1beta1
kind: InitConfiguration
general:
  namespace: "karmada-system"
  kubeConfigPath: "/etc/karmada/kubeconfig"
  kubeImageTag: "v1.21.0"
  context: "karmada-context"
  storageClassesName: "fast"
  privateImageRegistry: "local.registry.com"
  waitComponentReadyTimeout: 120
  port: 32443

certificate:
  externalDNS:
    - "www.karmada.io"
  externalIP:
    - "10.235.1.2"
  validityPeriod: "8760h"
  caCertFile: "/etc/karmada/pki/ca.crt"
  caCertKeyFile: "/etc/karmada/pki/ca.key"
  extraArgs:
    - name: "arg1"
      value: "value1"

etcd:
  local:
    image: "local.registry.com/library/etcd:3.5.13-0"
    initImage: "docker.io/alpine:3.19.1"
    dataDir: "/var/lib/karmada-etcd"
    pvcSize: "5Gi"
    nodeSelectorLabels:
      kubernetes.io/os: linux
      hello: world
    storageMode: "PVC"
    replicas: 3
    extraArgs:
      - name: "etcd-arg1"
        value: "etcd-value1"
  external:
    caPath: "/etc/ssl/certs/ca-certificates.crt"
    certPath: "/path/to/your/certificate.pem"
    keyPath: "/path/to/your/privatekey.pem"
    servers: "https://example.com:8443"
    prefix: "ext-"
    extraArgs:
      - name: "ext-etcd-arg1"
        value: "ext-etcd-value1"

karmadaControlPlane:
  apiServer:
    image: "karmada-apiserver:latest"
    advertiseAddress: "192.168.1.2"
    replicas: 3
    nodePort: 32443
    extraArgs:
      - name: "api-arg1"
        value: "api-value1"
  controllerManager:
    image: "karmada-controller-manager:latest"
    replicas: 3
    extraArgs:
      - name: "cm-arg1"
        value: "cm-value1"
  scheduler:
    image: "karmada-scheduler:latest"
    replicas: 3
    extraArgs:
      - name: "sched-arg1"
        value: "sched-value1"
  webhook:
    image: "karmada-webhook:latest"
    replicas: 3
    extraArgs:
      - name: "webhook-arg1"
        value: "webhook-value1"
  aggregatedAPIServer:
    image: "karmada-aggregated-apiserver:latest"
    replicas: 3
    extraArgs:
      - name: "aggregated-arg1"
        value: "aggregated-value1"
  kubeControllerManager:
    image: "kube-controller-manager:latest"
    replicas: 3
    extraArgs:
      - name: "kcm-arg1"
        value: "kcm-value1"
  dataPath: "/etc/karmada"
  pkiPath: "/etc/karmada/pki"
  crds: "/etc/karmada/crds"
  hostClusterDomain: "cluster.local"

image:
  kubeImageTag: "v1.21.0"
  kubeImageRegistry: "registry.cn-hangzhou.aliyuncs.com/google_containers"
  kubeImageMirrorCountry: "cn"
  imagePullPolicy: "IfNotPresent"
  imagePullSecrets:
    - "PullSecret1"
    - "PullSecret2"
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