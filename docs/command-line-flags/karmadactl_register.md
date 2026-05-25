---
title: karmadactl register
---

Register a cluster to Karmada control plane with Pull mode

### Synopsis

Register a cluster to Karmada control plane with Pull mode.

```
karmadactl register [karmada-apiserver-endpoint]
```

### Examples

```
  # Register cluster into karmada control plane with Pull mode.
  # If '--cluster-name' isn't specified, the cluster of current-context will be used by default.
  karmadactl register [karmada-apiserver-endpoint] --cluster-name=<CLUSTER_NAME> --token=<TOKEN>  --discovery-token-ca-cert-hash=<CA-CERT-HASH>
  
  # The 'karmada-apiserver-endpoint' argument is an address of the Karmada API server from which info will be fetched.
  # It should be provided in the format '<host>:<port>'.
  karmadactl register 172.18.0.2:5443 --cluster-name=<CLUSTER_NAME> --token=<TOKEN> --discovery-token-ca-cert-hash=<CA-CERT-HASH>
  
  # UnsafeSkipCAVerification allows token-based discovery without CA verification via CACertHashes. This can weaken
  # the security of register command since other clusters can impersonate the control-plane.
  karmadactl register [karmada-apiserver-endpoint] --token=<TOKEN> --discovery-token-unsafe-skip-ca-verification=true
```

### Options

```
      --cert-expiration-seconds int32                 The expiration time of certificate. (default 31536000)
      --cluster-name string                           The name of member cluster in the control plane, if not specified, the cluster of current-context is used by default.
      --cluster-namespace string                      Namespace in the control plane where member cluster secrets are stored. (default "karmada-cluster")
      --cluster-provider string                       Provider of the joining cluster. The Karmada scheduler can use this information to spread workloads across providers for higher availability.
      --cluster-region string                         The region of the joining cluster. The Karmada scheduler can use this information to spread workloads across regions for higher availability.
      --cluster-zones strings                         The zones of the joining cluster. The Karmada scheduler can use this information to spread workloads across zones for higher availability.
      --context string                                Name of the cluster context in kubeconfig file.
      --discovery-timeout duration                    The timeout to discovery karmada apiserver client. (default 5m0s)
      --discovery-token-ca-cert-hash strings          For token-based discovery, validate that the root CA public key matches this hash (format: "<type>:<value>").
      --discovery-token-unsafe-skip-ca-verification   For token-based discovery, allow joining without --discovery-token-ca-cert-hash pinning.
      --dry-run                                       Run the command in dry-run mode, without making any server requests.
      --enable-cert-rotation                          Enable means controller would rotate certificate for karmada-agent when the certificate is about to expire.
  -h, --help                                          help for register
      --karmada-agent-image string                    Karmada agent image. (default "docker.io/karmada/karmada-agent:v0.0.0-master")
      --karmada-agent-replicas int32                  Karmada agent replicas. (default 1)
      --kubeconfig string                             Path to the kubeconfig file of member cluster.
  -n, --namespace string                              Namespace the karmada-agent component deployed. (default "karmada-system")
      --proxy-server-address string                   Address of the proxy server that is used to proxy to the cluster.
      --token string                                  For token-based discovery, the token used to validate cluster information fetched from the API server.
```

### Options inherited from parent commands

```
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log-file string                  If non-empty, use this log file (no effect when -logtostderr=true)
      --log-file-max-size uint           Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity         logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [karmadactl](karmadactl.md)	 - karmadactl controls a Kubernetes Cluster Federation.

#### Go Back to [Karmadactl Commands](karmadactl_index.md) Homepage.


###### Auto generated by [spf13/cobra script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/genkarmadactldocs).