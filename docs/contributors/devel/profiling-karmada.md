# Profiling Karmada

## Enable profiling

To profile Karmada components running inside a Kubernetes pod, set --enable-pprof flag to true in the yaml of Karmada components. 
The default profiling address is 127.0.0.1:6060, and it can be configured via `--profiling-bind-address`.
The components which are compiled by the Karmada source code support the flag above, including `Karmada-agent`, `Karmada-aggregated-apiserver`, `Karmada-controller-manager`, `Karmada-descheduler`, `Karmada-search`, `Karmada-scheduler`, `Karmada-scheduler-estimator`, `Karmada-webhook`.

```
--enable-pprof                                                                                                                                                                
                Enable profiling via web interface host:port/debug/pprof/.
--profiling-bind-address string                                                                                                                                               
                The TCP address for serving profiling(e.g. 127.0.0.1:6060, :6060). This is only applicable if profiling is enabled. (default ":6060")

```

## Expose the endpoint at the local port

You can get at the application in the pod by port forwarding with kubectl, for example:

```shell
$ kubectl -n karmada-system get pod
NAME                                          READY   STATUS    RESTARTS   AGE
karmada-controller-manager-7567b44b67-8kt59   1/1     Running   0          19s
...
```

```shell
$ kubectl -n karmada-system port-forward karmada-controller-manager-7567b44b67-8kt59 6060
Forwarding from 127.0.0.1:6060 -> 6060
Forwarding from [::1]:6060 -> 6060
```

The HTTP endpoint will now be available as a local port.

## Generate the data

You can then generate the file for the memory profile with curl and pipe the data to a file:

```shell
$ curl http://localhost:6060/debug/pprof/heap  > heap.pprof
```

Generate the file for the CPU profile with curl and pipe the data to a file (7200 seconds is two hours):

```shell
curl "http://localhost:6060/debug/pprof/profile?seconds=7200" > cpu.pprof
```

## Analyze the data

To analyze the data:

```shell
go tool pprof heap.pprof
```

## Read more about profiling

1. [Profiling Golang Programs on Kubernetes](https://danlimerick.wordpress.com/2017/01/24/profiling-golang-programs-on-kubernetes/)
2. [Official Go blog](https://blog.golang.org/pprof)