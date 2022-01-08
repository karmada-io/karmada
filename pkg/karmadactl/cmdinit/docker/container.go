package docker

import (
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
)

const (
	certsContainerTargetPath                       = "/etc/kubernetes/pki"
	kubeConfigContainerTargetPath                  = "/etc/kubeconfig"
	etcdContainerAndHostName                       = "karmada-etcd"
	karmadaAPIServerContainerAndHostName           = "karmada-apiserver"
	kubeControllerManagerContainerAndHostName      = "kube-controller-manager"
	karmadaSchedulerContainerAndHostName           = "karmada-scheduler"
	karmadaControllerManagerContainerAndHostName   = "karmada-controller-manager"
	karmadaWebhookContainerAndHostName             = "karmada-webhook"
	karmadaAggregatedAPIServerContainerAndHostName = "karmada-aggregated-apiserver"
	externalName                                   = "karmada-aggregated-apiserver.karmada-system.svc.cluster.local"
	webhookExternalName                            = "karmada-webhook.karmada-system.svc"
)

var (
	certsMount = mount.Mount{
		Type:     mount.TypeBind,
		ReadOnly: true,
		Target:   certsContainerTargetPath,
	}

	kubeConfigMount = mount.Mount{
		Type:   mount.TypeBind,
		Target: kubeConfigContainerTargetPath,
	}
)

// createEtcdContainer create etcd container
func (d *CommandInitDockerOption) createEtcdContainer() (string, error) {
	dataMount := mount.Mount{
		Type:   mount.TypeBind,
		Target: "/var/lib/etcd",
	}
	dataMount.Source = options.EtcdHostDataPath
	certsMount.Source = options.KarmadaDataPath

	body, err := d.cli.ContainerCreate(d.ctx, &container.Config{
		Image: options.EtcdImage,
		Cmd: []string{
			"/usr/local/bin/etcd",
			"--name=etcd-0",
			"--client-cert-auth=true",
			"--cert-file=" + certsContainerTargetPath + "/" + options.EtcdServerCertAndKeyName + ".crt",
			"--key-file=" + certsContainerTargetPath + "/" + options.EtcdServerCertAndKeyName + ".key",
			"--peer-client-cert-auth=true",
			"--peer-cert-file=" + certsContainerTargetPath + "/" + options.EtcdServerCertAndKeyName + ".crt",
			"--peer-key-file=" + certsContainerTargetPath + "/" + options.EtcdServerCertAndKeyName + ".key",
			"--peer-trusted-ca-file=" + certsContainerTargetPath + "/" + options.CaCertAndKeyName + ".crt",
			"--trusted-ca-file=" + certsContainerTargetPath + "/" + options.CaCertAndKeyName + ".crt",
			"--listen-peer-urls=http://0.0.0.0:2380",
			"--listen-client-urls=https://0.0.0.0:2379",
			"--advertise-client-urls=https://0.0.0.0:2379",
			"--initial-cluster=etcd-0=https://0.0.0.0:2380",
			"--initial-advertise-peer-urls=https://0.0.0.0:2380",
			"--initial-cluster-token=etcd-cluster",
			"--initial-cluster-state=new",
			"--data-dir=/var/lib/etcd",
		},
		Hostname: etcdContainerAndHostName,
		//ExposedPorts: map[nat.Port]struct{}{"2379/tcp": {}, "2380/tcp": {}},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			dataMount,
			certsMount,
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"karmada": {},
		},
	}, nil, etcdContainerAndHostName)
	if err != nil {
		return "", err
	}

	return body.ID, nil
}

// createEtcdContainer create api container
func (d *CommandInitDockerOption) createKarmadaAPIServerContainer() (string, error) {
	certsMount.Source = options.KarmadaDataPath

	body, err := d.cli.ContainerCreate(d.ctx, &container.Config{
		Image: options.KarmadaAPIServerImage,
		Cmd: []string{
			"kube-apiserver",
			"--allow-privileged=true",
			"--authorization-mode=Node,RBAC",
			fmt.Sprintf("--client-ca-file=%s/%s.crt", certsContainerTargetPath, options.CaCertAndKeyName),
			"--enable-admission-plugins=NodeRestriction",
			"--enable-bootstrap-token-auth=true",
			fmt.Sprintf("--etcd-cafile=%s/%s.crt", certsContainerTargetPath, options.CaCertAndKeyName),
			fmt.Sprintf("--etcd-certfile=%s/%s.crt", certsContainerTargetPath, options.EtcdClientCertAndKeyName),
			fmt.Sprintf("--etcd-keyfile=%s/%s.key", certsContainerTargetPath, options.EtcdClientCertAndKeyName),
			fmt.Sprintf("--etcd-servers=https://%s:2379", etcdContainerAndHostName),
			"--bind-address=0.0.0.0",
			"--insecure-port=0",
			fmt.Sprintf("--kubelet-client-certificate=%s/%s.crt", certsContainerTargetPath, options.KarmadaCertAndKeyName),
			fmt.Sprintf("--kubelet-client-key=%s/%s.key", certsContainerTargetPath, options.KarmadaCertAndKeyName),
			"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
			"--disable-admission-plugins=StorageObjectInUseProtection,ServiceAccount",
			"--runtime-config=",
			"--apiserver-count=1",
			"--secure-port=5443",
			"--service-account-issuer=https://kubernetes.default.svc.cluster.local",
			fmt.Sprintf("--service-account-key-file=%s/%s.key", certsContainerTargetPath, options.KarmadaCertAndKeyName),
			fmt.Sprintf("--service-account-signing-key-file=%s/%s.key", certsContainerTargetPath, options.KarmadaCertAndKeyName),
			"--service-cluster-ip-range=10.96.0.0/12",
			fmt.Sprintf("--proxy-client-cert-file=%s/%s.crt", certsContainerTargetPath, options.FrontProxyClientCertAndKeyName),
			fmt.Sprintf("--proxy-client-key-file=%s/%s.key", certsContainerTargetPath, options.FrontProxyClientCertAndKeyName),
			"--requestheader-allowed-names=front-proxy-client",
			fmt.Sprintf("--requestheader-client-ca-file=%s/%s.crt", certsContainerTargetPath, options.FrontProxyCaCertAndKeyName),
			"--requestheader-extra-headers-prefix=X-Remote-Extra-",
			"--requestheader-group-headers=X-Remote-Group",
			"--requestheader-username-headers=X-Remote-User",
			fmt.Sprintf("--tls-cert-file=%s/%s.crt", certsContainerTargetPath, options.KarmadaCertAndKeyName),
			fmt.Sprintf("--tls-private-key-file=%s/%s.key", certsContainerTargetPath, options.KarmadaCertAndKeyName),
		},
		Hostname:     karmadaAPIServerContainerAndHostName,
		ExposedPorts: map[nat.Port]struct{}{"5443/tcp": {}},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			certsMount,
		},
		PortBindings: map[nat.Port][]nat.PortBinding{
			"5443/tcp": {
				{
					HostIP:   "0.0.0.0",
					HostPort: d.KarmadaAPIServerHostPort,
				},
			},
		},
		Links: []string{
			etcdContainerAndHostName,
		},
		ExtraHosts: []string{
			externalName + ":166.233.0.167",
			webhookExternalName + ":166.233.0.166",
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"karmada": {},
		},
	}, nil, karmadaAPIServerContainerAndHostName)
	if err != nil {
		return "", err
	}

	return body.ID, nil
}

// createKubeControllerManagerContainer create k8s ControllerManager container
func (d *CommandInitDockerOption) createKubeControllerManagerContainer() (string, error) {
	certsMount.Source = options.KarmadaDataPath
	kubeConfigMount.Source = options.KarmadaDataPath + "/kubeconfig"

	body, err := d.cli.ContainerCreate(d.ctx, &container.Config{
		Image: options.KubeControllerManagerImage,
		Cmd: []string{
			"kube-controller-manager",
			"--allocate-node-cidrs=true",
			"--authentication-kubeconfig=/etc/kubeconfig",
			"--authorization-kubeconfig=/etc/kubeconfig",
			"--bind-address=0.0.0.0",
			fmt.Sprintf("--client-ca-file=%s/%s.crt", certsContainerTargetPath, options.CaCertAndKeyName),
			"--cluster-cidr=10.244.0.0/16",
			fmt.Sprintf("--cluster-name=%s", options.ClusterName),
			fmt.Sprintf("--cluster-signing-cert-file=%s/%s.crt", certsContainerTargetPath, options.CaCertAndKeyName),
			fmt.Sprintf("--cluster-signing-key-file=%s/%s.key", certsContainerTargetPath, options.CaCertAndKeyName),
			"--controllers=namespace,garbagecollector,serviceaccount-token",
			"--kubeconfig=/etc/kubeconfig",
			"--leader-elect=true",
			"--node-cidr-mask-size=24",
			"--port=0",
			fmt.Sprintf("--root-ca-file=%s/%s.crt", certsContainerTargetPath, options.CaCertAndKeyName),
			fmt.Sprintf("--service-account-private-key-file=%s/%s.key", certsContainerTargetPath, options.KarmadaCertAndKeyName),
			"--service-cluster-ip-range=10.96.0.0/12",
			"--use-service-account-credentials=true",
			"--v=4",
		},
		Hostname: kubeControllerManagerContainerAndHostName,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			certsMount,
			kubeConfigMount,
		},
		Links: []string{
			karmadaAPIServerContainerAndHostName,
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"karmada": {},
		},
	}, nil, kubeControllerManagerContainerAndHostName)
	if err != nil {
		return "", err
	}

	return body.ID, nil
}

// createKarmadaSchedulerContainer create karmada scheduler container
func (d *CommandInitDockerOption) createKarmadaSchedulerContainer() (string, error) {
	certsMount.Source = options.KarmadaDataPath
	kubeConfigMount.Source = options.KarmadaDataPath + "/kubeconfig"

	body, err := d.cli.ContainerCreate(d.ctx, &container.Config{
		Image: options.KarmadaSchedulerImage,
		Cmd: []string{
			"/bin/karmada-scheduler",
			"--kubeconfig=/etc/kubeconfig",
			"--bind-address=0.0.0.0",
			"--secure-port=10351",
			"--feature-gates=Failover=true",
			"--enable-scheduler-estimator=true",
			"--leader-elect-resource-namespace=karmada-system",
			"--leader-elect=true",
			"--v=4",
		},
		Hostname: karmadaSchedulerContainerAndHostName,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			certsMount,
			kubeConfigMount,
		},
		Links: []string{
			karmadaAPIServerContainerAndHostName,
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"karmada": {},
		},
	}, nil, karmadaSchedulerContainerAndHostName)
	if err != nil {
		return "", err
	}

	return body.ID, nil
}

// createKarmadaControllerManagerContainer create karmada ControllerManager container
func (d *CommandInitDockerOption) createKarmadaControllerManagerContainer() (string, error) {
	certsMount.Source = options.KarmadaDataPath
	kubeConfigMount.Source = options.KarmadaDataPath + "/kubeconfig"

	body, err := d.cli.ContainerCreate(d.ctx, &container.Config{
		Image: options.KarmadaControllerManagerImage,
		Cmd: []string{
			"/bin/karmada-controller-manager",
			"--kubeconfig=/etc/kubeconfig",
			"--bind-address=0.0.0.0",
			"--cluster-status-update-frequency=10s",
			"--secure-port=10357",
			"--v=4",
		},
		Hostname: karmadaControllerManagerContainerAndHostName,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			certsMount,
			kubeConfigMount,
		},
		Links: []string{
			karmadaAPIServerContainerAndHostName,
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"karmada": {},
		},
	}, nil, karmadaControllerManagerContainerAndHostName)
	if err != nil {
		return "", err
	}

	return body.ID, nil
}

// createKarmadaWebhookContainer create karmada webhook container
func (d *CommandInitDockerOption) createKarmadaWebhookContainer() (string, error) {
	webhookCertsMount := mount.Mount{
		Type:     mount.TypeBind,
		ReadOnly: true,
		Target:   "/var/serving-cert/" + "tls.crt",
	}
	webhookKeyMount := mount.Mount{
		Type:     mount.TypeBind,
		ReadOnly: true,
		Target:   "/var/serving-cert/" + "tls.key",
	}
	kubeConfigMount.Source = options.KarmadaDataPath + "/kubeconfig"
	webhookCertsMount.Source = options.KarmadaDataPath + fmt.Sprintf("/%s.crt", options.KarmadaCertAndKeyName)
	webhookKeyMount.Source = options.KarmadaDataPath + fmt.Sprintf("/%s.key", options.KarmadaCertAndKeyName)

	body, err := d.cli.ContainerCreate(d.ctx, &container.Config{
		Image: options.KarmadaWebhookImage,
		Cmd: []string{
			"/bin/karmada-webhook",
			"--kubeconfig=/etc/kubeconfig",
			"--bind-address=0.0.0.0",
			"--secure-port=443",
			"--cert-dir=/var/serving-cert",
			"--v=4",
		},
		Hostname: karmadaWebhookContainerAndHostName,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			webhookCertsMount,
			webhookKeyMount,
			kubeConfigMount,
		},
		Links: []string{
			karmadaAPIServerContainerAndHostName,
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"karmada": {
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: "166.233.0.166",
				},
			},
		},
	}, nil, karmadaWebhookContainerAndHostName)
	if err != nil {
		return "", err
	}

	return body.ID, nil
}

// createKarmadaAggregatedAPIServerContainer create karmada aa container
func (d *CommandInitDockerOption) createKarmadaAggregatedAPIServerContainer() (string, error) {
	certsMount.Source = options.KarmadaDataPath
	kubeConfigMount.Source = options.KarmadaDataPath + "/kubeconfig"
	body, err := d.cli.ContainerCreate(d.ctx, &container.Config{
		Image: options.KarmadaAggregatedAPIServerImage,
		Cmd: []string{
			"/bin/karmada-aggregated-apiserver",
			"--kubeconfig=/etc/kubeconfig",
			"--authentication-kubeconfig=/etc/kubeconfig",
			"--authorization-kubeconfig=/etc/kubeconfig",
			"--karmada-config=/etc/kubeconfig",
			fmt.Sprintf("--etcd-servers=https://%s:2379", etcdContainerAndHostName),
			fmt.Sprintf("--etcd-cafile=%s/%s.crt", certsContainerTargetPath, options.CaCertAndKeyName),
			fmt.Sprintf("--etcd-certfile=%s/%s.crt", certsContainerTargetPath, options.EtcdClientCertAndKeyName),
			fmt.Sprintf("--etcd-keyfile=%s/%s.key", certsContainerTargetPath, options.EtcdClientCertAndKeyName),
			fmt.Sprintf("--tls-cert-file=%s/%s.crt", certsContainerTargetPath, options.KarmadaCertAndKeyName),
			fmt.Sprintf("--tls-private-key-file=%s/%s.key", certsContainerTargetPath, options.KarmadaCertAndKeyName),
			"--audit-log-path=-",
			"--feature-gates=APIPriorityAndFairness=false",
			"--audit-log-maxage=0",
			"--audit-log-maxbackup=0",
		},
		Hostname: karmadaAggregatedAPIServerContainerAndHostName,
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			certsMount,
			kubeConfigMount,
		},
		Links: []string{
			karmadaAPIServerContainerAndHostName,
			etcdContainerAndHostName,
		},
		ExtraHosts: []string{
			webhookExternalName + ":166.233.0.166",
		},
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"karmada": {
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: "166.233.0.167",
				},
			},
		},
	}, nil, karmadaAggregatedAPIServerContainerAndHostName)
	if err != nil {
		return "", err
	}

	return body.ID, nil
}
