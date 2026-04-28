---
title: karmada-aggregated-apiserver
---



### Synopsis

The karmada-aggregated-apiserver starts an aggregated server. 
It is responsible for registering the Cluster API and provides the ability to aggregate APIs, 
allowing users to access member clusters from the control plane directly.

```
karmada-aggregated-apiserver [flags]
```

### Options

```
Available Commands:
  karmada-aggregated-apiserver completion                      Generate the autocompletion script for the specified shell
  karmada-aggregated-apiserver help                            Help about any command
  karmada-aggregated-apiserver version                         Print the version information

Logs flags:

      --add-dir-header                       If true, adds the file directory to the header of the log messages
      --alsologtostderr                      log to standard error as well as files (no effect when -logtostderr=true)
      --log-backtrace-at traceLocation       when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                       If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log-file string                      If non-empty, use this log file (no effect when -logtostderr=true)
      --log-file-max-size uint               Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --log-flush-frequency duration         Maximum number of seconds between log flushes (default 5s)
      --log-text-info-buffer-size quantity   [Alpha] In text format with split output streams, the info messages can be buffered for a while to increase performance. The default value of zero bytes disables buffering. The size can be specified as number of bytes (512), multiples of 1000 (1K), multiples of 1024 (2Ki), or powers of those (3M, 4G, 5Mi, 6Gi). Enable the LoggingAlphaOptions feature gate to use this.
      --log-text-split-stream                [Alpha] In text format, write error messages to stderr and info messages to stdout. The default is to write a single stream to stdout. Enable the LoggingAlphaOptions feature gate to use this.
      --logging-format string                Sets the log format. Permitted formats: "text". (default "text")
      --logtostderr                          log to standard error instead of files (default true)
      --one-output                           If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip-headers                         If true, avoid header prefixes in the log messages
      --skip-log-headers                     If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity             logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -v, --v Level                              number for the log level verbosity
      --vmodule pattern=N,...                comma-separated list of pattern=N settings for file-filtered logging (only works for text log format)

Generic flags:

      --audit-log-batch-buffer-size int                         The size of the buffer to store events before batching and writing. Only used in batch mode. (default 10000)
      --audit-log-batch-max-size int                            The maximum size of a batch. Only used in batch mode. (default 1)
      --audit-log-batch-max-wait duration                       The amount of time to wait before force writing the batch that hadn't reached the max size. Only used in batch mode.
      --audit-log-batch-throttle-burst int                      Maximum number of requests sent at the same moment if ThrottleQPS was not utilized before. Only used in batch mode.
      --audit-log-batch-throttle-enable                         Whether batching throttling is enabled. Only used in batch mode.
      --audit-log-batch-throttle-qps float32                    Maximum average number of batches per second. Only used in batch mode.
      --audit-log-compress                                      If set, the rotated log files will be compressed using gzip.
      --audit-log-format string                                 Format of saved audits. "legacy" indicates 1-line text format for each event. "json" indicates structured json format. Known formats are legacy,json. (default "json")
      --audit-log-maxage int                                    The maximum number of days to retain old audit log files based on the timestamp encoded in their filename.
      --audit-log-maxbackup int                                 The maximum number of old audit log files to retain. Setting a value of 0 will mean there's no restriction on the number of files.
      --audit-log-maxsize int                                   The maximum size in megabytes of the audit log file before it gets rotated.
      --audit-log-mode string                                   Strategy for sending audit events. Blocking indicates sending events should block server responses. Batch causes the backend to buffer and write events asynchronously. Known modes are batch,blocking,blocking-strict. (default "blocking")
      --audit-log-path string                                   If set, all requests coming to the apiserver will be logged to this file.  '-' means standard out.
      --audit-log-truncate-enabled                              Whether event and batch truncating is enabled.
      --audit-log-truncate-max-batch-size int                   Maximum size of the batch sent to the underlying backend. Actual serialized size can be several hundreds of bytes greater. If a batch exceeds this limit, it is split into several batches of smaller size. (default 10485760)
      --audit-log-truncate-max-event-size int                   Maximum size of the audit event sent to the underlying backend. If the size of an event is greater than this number, first request and response are removed, and if this doesn't reduce the size enough, event is discarded. (default 102400)
      --audit-log-version string                                API group and version used for serializing audit events written to log. (default "audit.k8s.io/v1")
      --audit-policy-file string                                Path to the file that defines the audit policy configuration.
      --audit-webhook-batch-buffer-size int                     The size of the buffer to store events before batching and writing. Only used in batch mode. (default 10000)
      --audit-webhook-batch-max-size int                        The maximum size of a batch. Only used in batch mode. (default 400)
      --audit-webhook-batch-max-wait duration                   The amount of time to wait before force writing the batch that hadn't reached the max size. Only used in batch mode. (default 30s)
      --audit-webhook-batch-throttle-burst int                  Maximum number of requests sent at the same moment if ThrottleQPS was not utilized before. Only used in batch mode. (default 15)
      --audit-webhook-batch-throttle-enable                     Whether batching throttling is enabled. Only used in batch mode. (default true)
      --audit-webhook-batch-throttle-qps float32                Maximum average number of batches per second. Only used in batch mode. (default 10)
      --audit-webhook-config-file string                        Path to a kubeconfig formatted file that defines the audit webhook configuration.
      --audit-webhook-initial-backoff duration                  The amount of time to wait before retrying the first failed request. (default 10s)
      --audit-webhook-mode string                               Strategy for sending audit events. Blocking indicates sending events should block server responses. Batch causes the backend to buffer and write events asynchronously. Known modes are batch,blocking,blocking-strict. (default "batch")
      --audit-webhook-truncate-enabled                          Whether event and batch truncating is enabled.
      --audit-webhook-truncate-max-batch-size int               Maximum size of the batch sent to the underlying backend. Actual serialized size can be several hundreds of bytes greater. If a batch exceeds this limit, it is split into several batches of smaller size. (default 10485760)
      --audit-webhook-truncate-max-event-size int               Maximum size of the audit event sent to the underlying backend. If the size of an event is greater than this number, first request and response are removed, and if this doesn't reduce the size enough, event is discarded. (default 102400)
      --audit-webhook-version string                            API group and version used for serializing audit events written to webhook. (default "audit.k8s.io/v1")
      --authentication-kubeconfig string                        kubeconfig file pointing at the 'core' kubernetes server with enough rights to create tokenreviews.authentication.k8s.io.
      --authentication-skip-lookup                              If false, the authentication-kubeconfig will be used to lookup missing authentication configuration from the cluster.
      --authentication-token-webhook-cache-ttl duration         The duration to cache responses from the webhook token authenticator. (default 10s)
      --authentication-tolerate-lookup-failure                  If true, failures to look up missing authentication configuration from the cluster are not considered fatal. Note that this can result in authentication that treats all requests as anonymous.
      --authorization-always-allow-paths strings                A list of HTTP paths to skip during authorization, i.e. these are authorized without contacting the 'core' kubernetes server. (default [/healthz,/readyz,/livez])
      --authorization-kubeconfig string                         kubeconfig file pointing at the 'core' kubernetes server with enough rights to create subjectaccessreviews.authorization.k8s.io.
      --authorization-webhook-cache-authorized-ttl duration     The duration to cache 'authorized' responses from the webhook authorizer. (default 10s)
      --authorization-webhook-cache-unauthorized-ttl duration   The duration to cache 'unauthorized' responses from the webhook authorizer. (default 10s)
      --bind-address ip                                         The IP address on which to listen for the --secure-port port. The associated interface(s) must be reachable by the rest of the cluster, and by CLI/web clients. If blank or an unspecified address (0.0.0.0 or ::), all interfaces and IP address families will be used. (default 0.0.0.0)
      --cert-dir string                                         The directory where the TLS certs are located. If --tls-cert-file and --tls-private-key-file are provided, this flag will be ignored. (default "apiserver.local.config/certificates")
      --client-ca-file string                                   If set, any request presenting a client certificate signed by one of the authorities in the client-ca-file is authenticated with an identity corresponding to the CommonName of the client certificate.
      --contention-profiling                                    Enable block profiling, if profiling is enabled
      --debug-socket-path string                                Use an unprotected (no authn/authz) unix-domain socket for profiling with the given path
      --delete-collection-workers int                           Number of workers spawned for DeleteCollection call. These are used to speed up namespace cleanup. (default 1)
      --disable-http2-serving                                   If true, HTTP2 serving will be disabled [default=false]
      --enable-garbage-collector                                Enables the generic garbage collector. MUST be synced with the corresponding flag of the kube-controller-manager. (default true)
      --enable-pprof                                            Enable profiling via web interface host:port/debug/pprof/.
      --enable-priority-and-fairness                            If true, replace the max-in-flight handler with an enhanced one that queues and dispatches with priority and fairness (default true)
      --encryption-provider-config string                       The file containing configuration for encryption providers to be used for storing secrets in etcd
      --encryption-provider-config-automatic-reload             Determines if the file set by --encryption-provider-config should be automatically reloaded if the disk contents change. Setting this to true disables the ability to uniquely identify distinct KMS plugins via the API server healthz endpoints.
      --etcd-cafile string                                      SSL Certificate Authority file used to secure etcd communication.
      --etcd-certfile string                                    SSL certification file used to secure etcd communication.
      --etcd-compaction-interval duration                       The interval of compaction requests. If 0, the compaction request from apiserver is disabled. (default 5m0s)
      --etcd-count-metric-poll-period duration                  Frequency of polling etcd for number of resources per type. 0 disables the metric collection. (default 1m0s)
      --etcd-db-metric-poll-interval duration                   The interval of requests to poll etcd and update metric. 0 disables the metric collection (default 30s)
      --etcd-healthcheck-timeout duration                       The timeout to use when checking etcd health. (default 2s)
      --etcd-keyfile string                                     SSL key file used to secure etcd communication.
      --etcd-prefix string                                      The prefix to prepend to all resource paths in etcd. (default "/registry")
      --etcd-readycheck-timeout duration                        The timeout to use when checking etcd readiness (default 2s)
      --etcd-servers strings                                    List of etcd servers to connect with (scheme://ip:port), comma separated.
      --etcd-servers-overrides strings                          Per-resource etcd servers overrides, comma separated. The individual override format: group/resource#servers, where servers are URLs, semicolon separated. Note that this applies only to resources compiled into this server binary. e.g. "/pods#http://etcd4:2379;http://etcd5:2379,/events#http://etcd6:2379"
      --feature-gates mapStringBool                             A set of key=value pairs that describe feature gates for alpha/experimental features. Options are:
                                                                APIResponseCompression=true|false (BETA - default=true)
                                                                APIServerIdentity=true|false (BETA - default=true)
                                                                APIServingWithRoutine=true|false (ALPHA - default=false)
                                                                AllAlpha=true|false (ALPHA - default=false)
                                                                AllBeta=true|false (BETA - default=false)
                                                                AllowParsingUserUIDFromCertAuth=true|false (BETA - default=true)
                                                                AllowUnsafeMalformedObjectDeletion=true|false (ALPHA - default=false)
                                                                CBORServingAndStorage=true|false (ALPHA - default=false)
                                                                ComponentFlagz=true|false (ALPHA - default=false)
                                                                ComponentStatusz=true|false (ALPHA - default=false)
                                                                ConcurrentWatchObjectDecode=true|false (BETA - default=false)
                                                                ConstrainedImpersonation=true|false (ALPHA - default=false)
                                                                ContextualLogging=true|false (BETA - default=true)
                                                                ControllerPriorityQueue=true|false (BETA - default=true)
                                                                CoordinatedLeaderElection=true|false (BETA - default=false)
                                                                CustomizedClusterResourceModeling=true|false (BETA - default=true)
                                                                DeclarativeValidation=true|false (BETA - default=true)
                                                                DeclarativeValidationTakeover=true|false (BETA - default=false)
                                                                DetectCacheInconsistency=true|false (BETA - default=true)
                                                                Failover=true|false (BETA - default=false)
                                                                FederatedQuotaEnforcement=true|false (ALPHA - default=false)
                                                                GracefulEviction=true|false (BETA - default=true)
                                                                ListFromCacheSnapshot=true|false (BETA - default=true)
                                                                LoggingAlphaOptions=true|false (ALPHA - default=false)
                                                                LoggingBetaOptions=true|false (BETA - default=true)
                                                                MultiClusterService=true|false (ALPHA - default=false)
                                                                MultiplePodTemplatesScheduling=true|false (ALPHA - default=false)
                                                                MutatingAdmissionPolicy=true|false (BETA - default=false)
                                                                OpenAPIEnums=true|false (BETA - default=true)
                                                                PriorityBasedScheduling=true|false (ALPHA - default=false)
                                                                PropagateDeps=true|false (BETA - default=true)
                                                                PropagationPolicyPreemption=true|false (ALPHA - default=false)
                                                                RemoteRequestHeaderUID=true|false (BETA - default=true)
                                                                ResourceQuotaEstimate=true|false (ALPHA - default=false)
                                                                SizeBasedListCostEstimate=true|false (BETA - default=true)
                                                                StatefulFailoverInjection=true|false (ALPHA - default=false)
                                                                StorageVersionAPI=true|false (ALPHA - default=false)
                                                                StorageVersionHash=true|false (BETA - default=true)
                                                                StructuredAuthenticationConfigurationEgressSelector=true|false (BETA - default=true)
                                                                StructuredAuthenticationConfigurationJWKSMetrics=true|false (BETA - default=true)
                                                                TokenRequestServiceAccountUIDValidation=true|false (BETA - default=true)
                                                                UnauthenticatedHTTP2DOSMitigation=true|false (BETA - default=true)
                                                                UnknownVersionInteroperabilityProxy=true|false (ALPHA - default=false)
                                                                WatchCacheInitializationPostStartHook=true|false (BETA - default=false)
                                                                WatchList=true|false (BETA - default=true)
                                                                WorkloadAffinity=true|false (ALPHA - default=false)
      --http2-max-streams-per-connection int                    The limit that the server gives to clients for the maximum number of streams in an HTTP/2 connection. Zero means to use golang's default.
      --kube-api-burst int                                      Burst to use while talking with karmada-apiserver. (default 60)
      --kube-api-qps float32                                    QPS to use while talking with karmada-apiserver. (default 40)
      --kubeconfig string                                       Path to karmada control plane kubeconfig file.
      --lease-reuse-duration-seconds int                        The time in seconds that each lease is reused. A lower value could avoid large number of objects reusing the same lease. Notice that a too small value may cause performance problems at storage layer. (default 60)
      --permit-address-sharing                                  If true, SO_REUSEADDR will be used when binding the port. This allows binding to wildcard IPs like 0.0.0.0 and specific IPs in parallel, and it avoids waiting for the kernel to release sockets in TIME_WAIT state. [default=false]
      --permit-port-sharing                                     If true, SO_REUSEPORT will be used when binding the port, which allows more than one instance to bind on the same address and port. [default=false]
      --profiling                                               Enable profiling via web interface host:port/debug/pprof/ (default true)
      --profiling-bind-address string                           The TCP address for serving profiling(e.g. 127.0.0.1:6060, :6060). This is only applicable if profiling is enabled. (default ":6060")
      --requestheader-allowed-names strings                     List of client certificate common names to allow to provide usernames in headers specified by --requestheader-username-headers. If empty, any client certificate validated by the authorities in --requestheader-client-ca-file is allowed.
      --requestheader-client-ca-file string                     Root certificate bundle to use to verify client certificates on incoming requests before trusting usernames in headers specified by --requestheader-username-headers. WARNING: generally do not depend on authorization being already done for incoming requests.
      --requestheader-extra-headers-prefix strings              List of request header prefixes to inspect. X-Remote-Extra- is suggested. (default [x-remote-extra-])
      --requestheader-group-headers strings                     List of request headers to inspect for groups. X-Remote-Group is suggested. (default [x-remote-group])
      --requestheader-uid-headers strings                       List of request headers to inspect for UIDs. X-Remote-Uid is suggested. Requires the RemoteRequestHeaderUID feature to be enabled.
      --requestheader-username-headers strings                  List of request headers to inspect for usernames. X-Remote-User is common. (default [x-remote-user])
      --secure-port int                                         The port on which to serve HTTPS with authentication and authorization. If 0, don't serve HTTPS at all. (default 443)
      --storage-backend string                                  The storage backend for persistence. Options: 'etcd3' (default).
      --storage-media-type string                               The media type to use to store objects in storage. Some resources or storage backends may only support a specific media type and will ignore this setting. Supported media types: [application/json, application/yaml, application/vnd.kubernetes.protobuf] (default "application/json")
      --tls-cert-file string                                    File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert). If HTTPS serving is enabled, and --tls-cert-file and --tls-private-key-file are not provided, a self-signed certificate and key are generated for the public address and saved to the directory specified by --cert-dir.
      --tls-cipher-suites strings                               Comma-separated list of cipher suites for the server. If omitted, the default Go cipher suites will be used. 
                                                                Preferred values: TLS_AES_128_GCM_SHA256, TLS_AES_256_GCM_SHA384, TLS_CHACHA20_POLY1305_SHA256, TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA, TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA, TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256, TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384, TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305, TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256. 
                                                                Insecure values: TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256, TLS_ECDHE_ECDSA_WITH_RC4_128_SHA, TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA, TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256, TLS_ECDHE_RSA_WITH_RC4_128_SHA, TLS_RSA_WITH_3DES_EDE_CBC_SHA, TLS_RSA_WITH_AES_128_CBC_SHA, TLS_RSA_WITH_AES_128_CBC_SHA256, TLS_RSA_WITH_AES_128_GCM_SHA256, TLS_RSA_WITH_AES_256_CBC_SHA, TLS_RSA_WITH_AES_256_GCM_SHA384, TLS_RSA_WITH_RC4_128_SHA.
      --tls-min-version string                                  Minimum TLS version supported. Possible values: VersionTLS10, VersionTLS11, VersionTLS12, VersionTLS13
      --tls-private-key-file string                             File containing the default x509 private key matching --tls-cert-file.
      --tls-sni-cert-key namedCertKey                           A pair of x509 certificate and private key file paths, optionally suffixed with a list of domain patterns which are fully qualified domain names, possibly with prefixed wildcard segments. The domain patterns also allow IP addresses, but IPs should only be used if the apiserver has visibility to the IP address requested by a client. If no domain patterns are provided, the names of the certificate are extracted. Non-wildcard matches trump over wildcard matches, explicit domain patterns trump over extracted names. For multiple key/certificate pairs, use the --tls-sni-cert-key multiple times. Examples: "example.crt,example.key" or "foo.crt,foo.key:*.foo.com,foo.com". (default [])
      --watch-cache                                             Enable watch caching in the apiserver (default true)
      --watch-cache-sizes strings                               Watch cache size settings for some resources (pods, nodes, etc.), comma separated. The individual setting format: resource[.group]#size, where resource is lowercase plural (no version), group is omitted for resources of apiVersion v1 (the legacy core API) and included for others, and size is a number. This option is only meaningful for resources built into the apiserver, not ones defined by CRDs or aggregated from external servers, and is only consulted if the watch-cache is enabled. The only meaningful size setting to supply here is zero, which means to disable watch caching for the associated resource; all non-zero values are equivalent and mean to not disable watch caching for that resource
```

###### Auto generated by [spf13/cobra script in Karmada](https://github.com/karmada-io/karmada/tree/master/hack/tools/gencomponentdocs)