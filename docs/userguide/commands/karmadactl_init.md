## karmadactl init

Install karmada in kubernetes

### Synopsis

Install karmada in kubernetes.

```
karmadactl init [flags]
```

### Examples

```
  # Install Karmada in Kubernetes cluster
  # The karmada-apiserver binds the master node's IP by default
  karmadactl init
  
  # China mainland registry mirror can be specified by using kube-image-mirror-country
  karmadactl init --kube-image-mirror-country=cn
  
  # Kube registry can be specified by using kube-image-registry
  karmadactl init --kube-image-registry=registry.cn-hangzhou.aliyuncs.com/google_containers
  
  # Specify the URL to download CRD tarball
  karmadactl init --crds https://github.com/karmada-io/karmada/releases/download/v0.0.0/crds.tar.gz
  
  # Specify the local CRD tarball
  karmadactl init --crds /root/crds.tar.gz
  
  # Use PVC to persistent storage etcd data
  karmadactl init --etcd-storage-mode PVC --storage-classes-name {StorageClassesName}
  
  # Use hostPath to persistent storage etcd data. For data security, only 1 etcd pod can run in hostPath mode
  karmadactl init --etcd-storage-mode hostPath  --etcd-replicas 1
  
  # Use hostPath to persistent storage etcd data but select nodes by labels
  karmadactl init --etcd-storage-mode hostPath --etcd-node-selector-labels karmada.io/etcd=true
  
  # Private registry can be specified for all images
  karmadactl init --etcd-image local.registry.com/library/etcd:3.5.3-0
  
  # Deploy highly available(HA) karmada
  karmadactl init --karmada-apiserver-replicas 3 --etcd-replicas 3 --etcd-storage-mode PVC --storage-classes-name {StorageClassesName}
  
  # Specify external IPs(load balancer or HA IP) which used to sign the certificate
  karmadactl init --cert-external-ip 10.235.1.2 --cert-external-dns www.karmada.io
```

### Options

```
      --cert-external-dns string                         the external DNS of Karmada certificate (e.g localhost,localhost.com)
      --cert-external-ip string                          the external IP of Karmada certificate (e.g 192.168.1.2,172.16.1.2)
      --context string                                   The name of the kubeconfig context to use
      --crds string                                      Karmada crds resource.(local file e.g. --crds /root/crds.tar.gz) (default "https://github.com/karmada-io/karmada/releases/download/v0.0.0/crds.tar.gz")
      --etcd-data string                                 etcd data path,valid in hostPath mode. (default "/var/lib/karmada-etcd")
      --etcd-image string                                etcd image
      --etcd-init-image string                           etcd init container image (default "docker.io/alpine:3.15.1")
      --etcd-node-selector-labels string                 etcd pod select the labels of the node. valid in hostPath mode ( e.g. --etcd-node-selector-labels karmada.io/etcd=true)
      --etcd-pvc-size string                             etcd data path,valid in pvc mode. (default "5Gi")
      --etcd-replicas int32                              etcd replica set, cluster 3,5...singular (default 1)
      --etcd-storage-mode string                         etcd data storage mode(emptyDir,hostPath,PVC). value is PVC, specify --storage-classes-name (default "hostPath")
  -h, --help                                             help for init
      --karmada-aggregated-apiserver-image string        karmada aggregated apiserver image (default "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-aggregated-apiserver:v0.0.0")
      --karmada-aggregated-apiserver-replicas int32      karmada aggregated apiserver replica set (default 1)
      --karmada-apiserver-image string                   Kubernetes apiserver image
      --karmada-apiserver-replicas int32                 karmada apiserver replica set (default 1)
      --karmada-controller-manager-image string          karmada controller manager  image (default "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-controller-manager:v0.0.0")
      --karmada-controller-manager-replicas int32        karmada controller manager replica set (default 1)
  -d, --karmada-data string                              karmada data path. kubeconfig cert and crds files (default "/etc/karmada")
      --karmada-kube-controller-manager-image string     Kubernetes controller manager image
      --karmada-kube-controller-manager-replicas int32   karmada kube controller manager replica set (default 1)
      --karmada-scheduler-image string                   karmada scheduler image (default "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-scheduler:v0.0.0")
      --karmada-scheduler-replicas int32                 karmada scheduler replica set (default 1)
      --karmada-webhook-image string                     karmada webhook image (default "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-webhook:v0.0.0")
      --karmada-webhook-replicas int32                   karmada webhook replica set (default 1)
      --kube-image-mirror-country string                 Country code of the kube image registry to be used. For Chinese mainland users, set it to cn
      --kube-image-registry string                       Kube image registry. For Chinese mainland users, you may use local gcr.io mirrors such as registry.cn-hangzhou.aliyuncs.com/google_containers to override default kube image registry
  -n, --namespace string                                 Kubernetes namespace (default "karmada-system")
  -p, --port int32                                       Karmada apiserver service node port (default 32443)
      --storage-classes-name string                      Kubernetes StorageClasses Name
```

### Options inherited from parent commands

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files
      --kubeconfig string                Paths to a kubeconfig. Only required if out-of-cluster.
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory
      --log-file string                  If non-empty, use this log file
      --log-file-max-size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level)
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [karmadactl](karmadactl.md)	 - karmadactl controls a Kubernetes Cluster Federation.

###### Auto generated by spf13/cobra on 26-Jul-2022
