/*
Copyright 2025 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	globaloptions "github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

func (i *CommandInitOption) defaultEtcdContainerCommand() []string {
	etcdClusterConfig := ""
	for v := int32(0); v < i.EtcdReplicas; v++ {
		etcdClusterConfig += fmt.Sprintf("%s-%v=http://%s-%v.%s.%s.svc.%s:%v", etcdStatefulSetAndServiceName, v, etcdStatefulSetAndServiceName, v, etcdStatefulSetAndServiceName, i.Namespace, i.HostClusterDomain, etcdContainerServerPort) + ","
	}

	command := []string{
		"/usr/local/bin/etcd",
		fmt.Sprintf("--name=$(%s)", etcdEnvPodName),
		fmt.Sprintf("--listen-peer-urls=http://$(%s):%v", etcdEnvPodIP, etcdContainerServerPort),
		fmt.Sprintf("--listen-client-urls=https://$(%s):%v,http://127.0.0.1:%v", etcdEnvPodIP, etcdContainerClientPort, etcdContainerClientPort),
		fmt.Sprintf("--advertise-client-urls=https://$(%s).%s.%s.svc.%s:%v", etcdEnvPodName, etcdStatefulSetAndServiceName, i.Namespace, i.HostClusterDomain, etcdContainerClientPort),
		fmt.Sprintf("--initial-cluster=%s", strings.TrimRight(etcdClusterConfig, ",")),
		"--initial-cluster-state=new",
		"--client-cert-auth=true",
		fmt.Sprintf("--cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdServerCertAndKeyName),
		fmt.Sprintf("--key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.EtcdServerCertAndKeyName),
		fmt.Sprintf("--trusted-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdCaCertAndKeyName),
		fmt.Sprintf("--data-dir=%s", etcdContainerDataVolumeMountPath),
		fmt.Sprintf("--cipher-suites=%s", etcdCipherSuites),
		"--initial-cluster-token=etcd-cluster",
		fmt.Sprintf("--initial-advertise-peer-urls=http://$(%s):%v", etcdEnvPodIP, etcdContainerServerPort),
		"--peer-client-cert-auth=false",
		fmt.Sprintf("--peer-trusted-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdCaCertAndKeyName),
		fmt.Sprintf("--peer-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.EtcdServerCertAndKeyName),
		fmt.Sprintf("--peer-cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdServerCertAndKeyName),
	}
	return command
}

func (i *CommandInitOption) defaultKarmadaAPIServerContainerCommand() []string {
	var etcdServers string
	if etcdServers = i.ExternalEtcdServers; etcdServers == "" {
		etcdServers = strings.TrimRight(i.etcdServers(), ",")
	}
	command := []string{
		"kube-apiserver",
		"--allow-privileged=true",
		"--authorization-mode=Node,RBAC",
		fmt.Sprintf("--client-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
		"--enable-bootstrap-token-auth=true",
		fmt.Sprintf("--etcd-cafile=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdCaCertAndKeyName),
		fmt.Sprintf("--etcd-certfile=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdClientCertAndKeyName),
		fmt.Sprintf("--etcd-keyfile=%s/%s.key", karmadaCertsVolumeMountPath, options.EtcdClientCertAndKeyName),
		fmt.Sprintf("--etcd-servers=%s", etcdServers),
		"--bind-address=0.0.0.0",
		"--disable-admission-plugins=StorageObjectInUseProtection,ServiceAccount",
		"--runtime-config=",
		fmt.Sprintf("--apiserver-count=%v", i.KarmadaAPIServerReplicas),
		fmt.Sprintf("--secure-port=%v", karmadaAPIServerContainerPort),
		fmt.Sprintf("--service-account-issuer=https://kubernetes.default.svc.%s", i.HostClusterDomain),
		fmt.Sprintf("--service-account-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
		fmt.Sprintf("--service-account-signing-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
		fmt.Sprintf("--service-cluster-ip-range=%s", serviceClusterIP),
		fmt.Sprintf("--proxy-client-cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.FrontProxyClientCertAndKeyName),
		fmt.Sprintf("--proxy-client-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.FrontProxyClientCertAndKeyName),
		"--requestheader-allowed-names=front-proxy-client",
		fmt.Sprintf("--requestheader-client-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.FrontProxyCaCertAndKeyName),
		"--requestheader-extra-headers-prefix=X-Remote-Extra-",
		"--requestheader-group-headers=X-Remote-Group",
		"--requestheader-username-headers=X-Remote-User",
		fmt.Sprintf("--tls-cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.ApiserverCertAndKeyName),
		fmt.Sprintf("--tls-private-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.ApiserverCertAndKeyName),
		"--tls-min-version=VersionTLS13",
	}
	if i.ExternalEtcdKeyPrefix != "" {
		command = append(command, fmt.Sprintf("--etcd-prefix=%s", i.ExternalEtcdKeyPrefix))
	}
	return command
}

func (i *CommandInitOption) defaultKarmadaSchedulerContainerCommand() []string {
	return []string{
		"/bin/karmada-scheduler",
		fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
		"--metrics-bind-address=$(POD_IP):8080",
		"--health-probe-bind-address=$(POD_IP):10351",
		"--enable-scheduler-estimator=true",
		"--leader-elect=true",
		"--scheduler-estimator-ca-file=/etc/karmada/pki/ca.crt",
		"--scheduler-estimator-cert-file=/etc/karmada/pki/karmada.crt",
		"--scheduler-estimator-key-file=/etc/karmada/pki/karmada.key",
		fmt.Sprintf("--leader-elect-resource-namespace=%s", i.Namespace),
		"--v=4",
	}
}

// default command line arguments for kube-controller-manager
func (i *CommandInitOption) defaultKarmadaKubeControllerManagerContainerCommand() []string {
	return []string{
		"kube-controller-manager",
		"--allocate-node-cidrs=true",
		fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
		fmt.Sprintf("--authentication-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
		fmt.Sprintf("--authorization-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
		"--bind-address=0.0.0.0",
		fmt.Sprintf("--client-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
		"--cluster-cidr=10.244.0.0/16",
		fmt.Sprintf("--cluster-name=%s", options.ClusterName),
		fmt.Sprintf("--cluster-signing-cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
		fmt.Sprintf("--cluster-signing-key-file=%s/%s.key", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
		"--controllers=namespace,garbagecollector,serviceaccount-token,ttl-after-finished,bootstrapsigner,tokencleaner,csrcleaner,csrsigning,clusterrole-aggregation",
		"--leader-elect=true",
		fmt.Sprintf("--leader-elect-resource-namespace=%s", i.Namespace),
		"--node-cidr-mask-size=24",
		fmt.Sprintf("--root-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, globaloptions.CaCertAndKeyName),
		fmt.Sprintf("--service-account-private-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
		fmt.Sprintf("--service-cluster-ip-range=%s", serviceClusterIP),
		"--use-service-account-credentials=true",
		"--v=4",
	}
}

func (i *CommandInitOption) defaultKarmadaControllerManagerContainerCommand() []string {
	return []string{
		"/bin/karmada-controller-manager",
		fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
		"--metrics-bind-address=$(POD_IP):8080",
		"--health-probe-bind-address=$(POD_IP):10357",
		"--cluster-status-update-frequency=10s",
		fmt.Sprintf("--leader-elect-resource-namespace=%s", i.Namespace),
		"--v=4",
	}
}

func (i *CommandInitOption) defaultKarmadaWebhookContainerCommand() []string {
	return []string{
		"/bin/karmada-webhook",
		fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
		"--bind-address=$(POD_IP)",
		"--metrics-bind-address=$(POD_IP):8080",
		"--health-probe-bind-address=$(POD_IP):8000",
		fmt.Sprintf("--secure-port=%v", webhookTargetPort),
		fmt.Sprintf("--cert-dir=%s", webhookCertVolumeMountPath),
		"--v=4",
	}
}

func (i *CommandInitOption) defaultKarmadaAggregatedAPIServerContainerCommand() []string {
	var etcdServers string
	if etcdServers = i.ExternalEtcdServers; etcdServers == "" {
		etcdServers = strings.TrimRight(i.etcdServers(), ",")
	}
	var command []string
	if strings.ToLower(i.SecretLayout) == secretLayoutSplit {
		command = []string{
			"/bin/karmada-aggregated-apiserver",
			fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
			fmt.Sprintf("--authentication-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
			fmt.Sprintf("--authorization-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
			fmt.Sprintf("--etcd-servers=%s", etcdServers),
			fmt.Sprintf("--etcd-cafile=%s/ca.crt", etcdClientCertVolumeMountPath),
			fmt.Sprintf("--etcd-certfile=%s/tls.crt", etcdClientCertVolumeMountPath),
			fmt.Sprintf("--etcd-keyfile=%s/tls.key", etcdClientCertVolumeMountPath),
			fmt.Sprintf("--tls-cert-file=%s/tls.crt", serverCertVolumeMountPath),
			fmt.Sprintf("--tls-private-key-file=%s/tls.key", serverCertVolumeMountPath),
			"--tls-min-version=VersionTLS13",
			"--audit-log-path=-",
			"--audit-log-maxage=0",
			"--audit-log-maxbackup=0",
			"--bind-address=$(POD_IP)",
		}
	} else {
		command = []string{
			"/bin/karmada-aggregated-apiserver",
			fmt.Sprintf("--kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
			fmt.Sprintf("--authentication-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
			fmt.Sprintf("--authorization-kubeconfig=%s", filepath.Join(karmadaConfigVolumeMountPath, util.KarmadaConfigFieldName)),
			fmt.Sprintf("--etcd-servers=%s", etcdServers),
			fmt.Sprintf("--etcd-cafile=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdCaCertAndKeyName),
			fmt.Sprintf("--etcd-certfile=%s/%s.crt", karmadaCertsVolumeMountPath, options.EtcdClientCertAndKeyName),
			fmt.Sprintf("--etcd-keyfile=%s/%s.key", karmadaCertsVolumeMountPath, options.EtcdClientCertAndKeyName),
			fmt.Sprintf("--tls-cert-file=%s/%s.crt", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
			fmt.Sprintf("--tls-private-key-file=%s/%s.key", karmadaCertsVolumeMountPath, options.KarmadaCertAndKeyName),
			"--tls-min-version=VersionTLS13",
			"--audit-log-path=-",
			"--audit-log-maxage=0",
			"--audit-log-maxbackup=0",
			"--bind-address=$(POD_IP)",
		}
	}
	if i.ExternalEtcdKeyPrefix != "" {
		command = append(command, fmt.Sprintf("--etcd-prefix=%s", i.ExternalEtcdKeyPrefix))
	}
	
	return command
}
