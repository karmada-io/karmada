package cert

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	certutil "k8s.io/client-go/util/cert"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

const (
	TestCertsTmp = "./test-certs-tmp"
)

func TestGenCerts(t *testing.T) {
	defer os.RemoveAll(TestCertsTmp)

	notAfter := time.Now().Add(Duration365d * 10).UTC()
	namespace := "kube-karmada"
	flagsExternalIP := ""
	masterIP := "192.168.1.1,192.168.1.2"

	var etcdServerCertDNS = []string{
		"localhost",
	}

	for i := 0; i < 3; i++ {
		etcdServerCertDNS = append(etcdServerCertDNS, fmt.Sprintf("%s-%v.%s.%s.svc.cluster.local", "etcd", i, "etcd", namespace))
	}

	etcdServerAltNames := certutil.AltNames{
		DNSNames: etcdServerCertDNS,
		IPs:      []net.IP{utils.StringToNetIP("127.0.0.1")},
	}
	etcdServerCertConfig := NewCertConfig("karmada-etcd-server", []string{}, etcdServerAltNames, &notAfter)

	etcdClientCertCfg := NewCertConfig("karmada-etcd-client", []string{}, certutil.AltNames{}, &notAfter)

	var karmadaDNS = []string{
		"localhost",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"karmada-apiserver",
		"karmada-webhook",
		fmt.Sprintf("%s.%s.svc.cluster.local", "karmada-apiserver", namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", "karmada-webhook", namespace),
		fmt.Sprintf("*.%s.svc.cluster.local", namespace),
		fmt.Sprintf("*.%s.svc", namespace),
	}
	if hostName, err := os.Hostname(); err != nil {
		fmt.Println("Failed to get the current hostname.", err)
	} else {
		karmadaDNS = append(karmadaDNS, hostName)
	}

	ips := utils.FlagsIP(flagsExternalIP)
	ips = append(ips, utils.FlagsIP(masterIP)...)

	internetIP, err := utils.InternetIP()
	if err != nil {
		fmt.Println("Failed to obtain internet IP. ", err)
	} else {
		ips = append(ips, internetIP)
	}

	ips = append(
		ips,
		utils.StringToNetIP("127.0.0.1"),
		utils.StringToNetIP("10.254.0.1"),
	)

	fmt.Println("karmada certificate ip ", ips)

	karmadaAltNames := certutil.AltNames{
		DNSNames: karmadaDNS,
		IPs:      ips,
	}

	karmadaCertCfg := NewCertConfig("system:admin", []string{"system:masters"}, karmadaAltNames, &notAfter)
	apiserverCertCfg := NewCertConfig("karmada-apiserver", []string{""}, karmadaAltNames, &notAfter)
	frontProxyClientCertCfg := NewCertConfig("front-proxy-client", []string{}, certutil.AltNames{}, &notAfter)

	if err := GenCerts(TestCertsTmp, etcdServerCertConfig, etcdClientCertCfg, karmadaCertCfg, apiserverCertCfg, frontProxyClientCertCfg); err != nil {
		fmt.Println(err)
	}
}
