package docker

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/cert"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/karmada"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

// CommandInitDockerOption holds all flags options for init.
type CommandInitDockerOption struct {
	hostIP                   net.IP
	KarmadaAPIServerHostPort string
	cli                      *client.Client
	ctx                      context.Context
}

// Validate Check that there are enough flags to run the command.
func (d *CommandInitDockerOption) Validate(parentCommand string) error {
	if strings.HasPrefix(options.EtcdHostDataPath, "./") {
		return fmt.Errorf("absolute path to the etcdHostData")
	}
	return nil
}

// Complete Initialize docker client
func (d *CommandInitDockerOption) Complete() error {
	d.ctx = context.Background()
	// cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation(),client.WithHost("tcp://192.168.59.3:2375"))
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	d.cli = cli

	if !utils.PathIsExist(options.EtcdHostDataPath) {
		return fmt.Errorf("failed to create '%s' directory", options.EtcdHostDataPath)
	}

	localIP, err := utils.LocalIP()
	if err != nil {
		return err
	}
	d.hostIP = localIP
	klog.Infof("karmada apiserver ip: %s", d.hostIP)

	if err := d.networkCreate(); err != nil {
		return err
	}

	return nil
}

//  genCerts create ca etcd karmada cert
func (d *CommandInitDockerOption) genCerts() error {
	notAfter := time.Now().Add(cert.Duration365d * 10).UTC()

	var etcdServerCertDNS = []string{
		"localhost",
		etcdContainerAndHostName,
	}

	etcdServerAltNames := certutil.AltNames{
		DNSNames: etcdServerCertDNS,
		IPs:      []net.IP{utils.StringToNetIP("127.0.0.1")},
	}
	etcdServerCertConfig := cert.NewCertConfig("karmada-etcd-server", []string{"karmada"}, etcdServerAltNames, &notAfter)
	etcdClientCertCfg := cert.NewCertConfig("karmada-etcd-client", []string{"karmada"}, certutil.AltNames{}, &notAfter)

	var karmadaDNS = []string{
		"localhost",
		karmadaAPIServerContainerAndHostName,
		karmadaWebhookContainerAndHostName,
		karmadaAggregatedAPIServerContainerAndHostName,
		externalName,
		webhookExternalName,
	}
	karmadaDNS = append(karmadaDNS, utils.FlagsDNS(options.ExternalDNS)...)

	karmadaIPs := utils.FlagsIP(options.ExternalIP)
	karmadaIPs = append(
		karmadaIPs,
		d.hostIP,
		utils.StringToNetIP("127.0.0.1"),
		utils.StringToNetIP("10.254.0.1"),
	)

	internetIP, err := utils.InternetIP()
	if err != nil {
		klog.Warningln("Failed to obtain internet IP. ", err)
	} else {
		karmadaIPs = append(karmadaIPs, internetIP)
	}

	karmadaAltNames := certutil.AltNames{
		DNSNames: karmadaDNS,
		IPs:      karmadaIPs,
	}
	karmadaCertCfg := cert.NewCertConfig("system:admin", []string{"system:masters"}, karmadaAltNames, &notAfter)

	frontProxyClientCertCfg := cert.NewCertConfig("front-proxy-client", []string{"karmada"}, certutil.AltNames{}, &notAfter)
	if err = cert.GenCerts(options.KarmadaDataPath, etcdServerCertConfig, etcdClientCertCfg, karmadaCertCfg, frontProxyClientCertCfg); err != nil {
		return err
	}
	return nil
}

func (d *CommandInitDockerOption) createKubeConfig() error {
	// Container
	karmadaContainerServerURL := fmt.Sprintf("https://%s:5443", karmadaAPIServerContainerAndHostName)
	// local
	karmadaServerURL := fmt.Sprintf("https://%s:%s", d.hostIP, d.KarmadaAPIServerHostPort)

	ca, err := utils.FileToBytes(options.KarmadaDataPath, fmt.Sprintf("%s.crt", options.CaCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.crt' conversion failed. %v", options.CaCertAndKeyName, err)
	}
	karmadaCert, err := utils.FileToBytes(options.KarmadaDataPath, fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.crt' conversion failed. %v", options.KarmadaCertAndKeyName, err)
	}
	karmadaKey, err := utils.FileToBytes(options.KarmadaDataPath, fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.key' conversion failed. %v", options.KarmadaCertAndKeyName, err)
	}
	if err := utils.WriteKubeConfigFromSpec(karmadaContainerServerURL, options.UserName, options.ClusterName, options.KarmadaDataPath, "kubeconfig",
		ca, karmadaKey, karmadaCert); err != nil {
		return fmt.Errorf("failed to create karmada kubeconfig file. %v", err)
	}
	if err := utils.WriteKubeConfigFromSpec(karmadaServerURL, options.UserName, options.ClusterName, options.KarmadaDataPath, options.KarmadaKubeConfigName,
		ca, karmadaKey, karmadaCert); err != nil {
		return fmt.Errorf("failed to create karmada kubeconfig file. %v", err)
	}
	klog.Info("Create karmada kubeconfig success.")
	return nil
}

// RunDockerInit Deploy karmada in dokcer
func (d *CommandInitDockerOption) RunDockerInit(_ io.Writer, parentCommand string) error {
	if err := d.startPullImage(); err != nil {
		return err
	}

	if err := d.genCerts(); err != nil {
		return err
	}

	// download karmada CRD resources
	if err := utils.DownloadCRD(options.CRDs); err != nil {
		return fmt.Errorf("download karmada crd resources failed. %v", err)
	}
	if err := d.createKubeConfig(); err != nil {
		return err
	}

	if err := d.initKarmadaAPIServer(); err != nil {
		return err
	}

	// Create CRDs in karmada
	ca, err := utils.FileToBytes(options.KarmadaDataPath, fmt.Sprintf("%s.crt", options.CaCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.crt' conversion failed. %v", options.CaCertAndKeyName, err)
	}
	caBase64 := base64.StdEncoding.EncodeToString(ca)
	if err := karmada.InitKarmadaResources(options.KarmadaDataPath, caBase64); err != nil {
		return err
	}

	if err := d.initKarmadaComponent(); err != nil {
		return err
	}

	utils.GenExamples(options.KarmadaDataPath, parentCommand)
	return nil
}

func (d *CommandInitDockerOption) initKarmadaAPIServer() error {
	etcdContainerID, err := d.createEtcdContainer()
	if err != nil {
		return err
	}
	if err := d.cli.ContainerStart(d.ctx, etcdContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	if err := d.WaitContainerReady(etcdContainerID, 10*time.Second, 30*time.Second); err != nil {
		return err
	}

	apiContainerID, err := d.createKarmadaAPIServerContainer()
	if err != nil {
		return err
	}
	if err := d.cli.ContainerStart(d.ctx, apiContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	if err := d.WaitContainerReady(apiContainerID, 15*time.Second, 60*time.Second); err != nil {
		return err
	}
	return nil
}

func (d *CommandInitDockerOption) initKarmadaComponent() error {
	aaContainerID, err := d.createKarmadaAggregatedAPIServerContainer()
	if err != nil {
		return err
	}
	if err := d.cli.ContainerStart(d.ctx, aaContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	if err := d.WaitContainerReady(aaContainerID, 10*time.Second, 15*time.Second); err != nil {
		return err
	}

	kubeControllerManagerContainerID, err := d.createKubeControllerManagerContainer()
	if err != nil {
		return err
	}
	if err := d.cli.ContainerStart(d.ctx, kubeControllerManagerContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	karmadaSchedulerContainerID, err := d.createKarmadaSchedulerContainer()
	if err != nil {
		return err
	}
	if err := d.cli.ContainerStart(d.ctx, karmadaSchedulerContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	karmadaControllerManagerContainerID, err := d.createKarmadaControllerManagerContainer()
	if err != nil {
		return err
	}
	if err := d.cli.ContainerStart(d.ctx, karmadaControllerManagerContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	karmadaWebhookContainerID, err := d.createKarmadaWebhookContainer()
	if err != nil {
		return err
	}
	if err := d.cli.ContainerStart(d.ctx, karmadaWebhookContainerID, types.ContainerStartOptions{}); err != nil {
		return err
	}
	return nil
}
