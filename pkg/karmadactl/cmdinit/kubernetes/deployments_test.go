/*
Copyright 2023 The Karmada Authors.

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
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	certconst "github.com/karmada-io/karmada/pkg/cert"
	initopt "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	globalopt "github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestCommandInitOption_etcdServers(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	if got := cmdOpt.etcdServers(); got == "" {
		t.Errorf("CommandInitOption.etcdServers() = %v, want none empty", got)
	}
}

func TestCommandInitOption_karmadaAPIServerContainerCommand(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	flags := cmdOpt.defaultKarmadaAPIServerContainerCommand()
	if len(flags) == 0 {
		t.Errorf("CommandInitOption.defaultKarmadaAPIServerContainerCommand() returns empty")
	}
}

func TestCommandInitOption_makeKarmadaAPIServerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaAPIServerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaAPIServerDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaKubeControllerManagerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaKubeControllerManagerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaKubeControllerManagerDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaSchedulerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaSchedulerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaSchedulerDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaControllerManagerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaControllerManagerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaControllerManagerDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaWebhookDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaWebhookDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaWebhookDeployment() returns nil")
	}
}

func TestCommandInitOption_makeKarmadaAggregatedAPIServerDeployment(t *testing.T) {
	cmdOpt := CommandInitOption{EtcdReplicas: 1, Namespace: "karmada"}
	deployment := cmdOpt.makeKarmadaAggregatedAPIServerDeployment()
	if deployment == nil {
		t.Error("CommandInitOption.makeKarmadaAggregatedAPIServerDeployment() returns nil")
	}
}

// helpers
func hasFlag(cmd []string, want string) bool {
	for _, c := range cmd {
		if c == want {
			return true
		}
	}
	return false
}

func TestKarmadaAPIServerContainerCommand_Flags_SplitAndLegacy(t *testing.T) {
	// split
	split := CommandInitOption{
		SecretLayout: "split",
		Namespace:    "karmada",
		EtcdReplicas: 1,
	}
	scmd := split.karmadaAPIServerContainerCommand()
	if !hasFlag(scmd, fmt.Sprintf("--client-ca-file=%s/ca.crt", serverCertVolumeMountPath)) {
		t.Fatalf("split: missing --client-ca-file=%s/ca.crt", serverCertVolumeMountPath)
	}
	if !hasFlag(scmd, fmt.Sprintf("--etcd-cafile=%s/ca.crt", etcdClientCertVolumeMountPath)) ||
		!hasFlag(scmd, fmt.Sprintf("--etcd-certfile=%s/tls.crt", etcdClientCertVolumeMountPath)) ||
		!hasFlag(scmd, fmt.Sprintf("--etcd-keyfile=%s/tls.key", etcdClientCertVolumeMountPath)) {
		t.Fatalf("split: etcd client flags not using etcd-client mount path")
	}
	if !hasFlag(scmd, fmt.Sprintf("--tls-cert-file=%s/tls.crt", serverCertVolumeMountPath)) ||
		!hasFlag(scmd, fmt.Sprintf("--tls-private-key-file=%s/tls.key", serverCertVolumeMountPath)) {
		t.Fatalf("split: server tls flags not using server mount path")
	}

	// legacy
	legacy := CommandInitOption{
		Namespace:    "karmada",
		EtcdReplicas: 1,
	}
	lcmd := legacy.karmadaAPIServerContainerCommand()
	if !hasFlag(lcmd, fmt.Sprintf("--client-ca-file=%s/%s.crt", karmadaCertsVolumeMountPath, globalopt.CaCertAndKeyName)) {
		t.Fatalf("legacy: missing client-ca-file from karmada-cert")
	}
	if !hasFlag(lcmd, fmt.Sprintf("--etcd-cafile=%s/%s.crt", karmadaCertsVolumeMountPath, initopt.EtcdCaCertAndKeyName)) ||
		!hasFlag(lcmd, fmt.Sprintf("--etcd-certfile=%s/%s.crt", karmadaCertsVolumeMountPath, initopt.EtcdClientCertAndKeyName)) ||
		!hasFlag(lcmd, fmt.Sprintf("--etcd-keyfile=%s/%s.key", karmadaCertsVolumeMountPath, initopt.EtcdClientCertAndKeyName)) {
		t.Fatalf("legacy: missing etcd client flags from karmada-cert")
	}
}

func TestMakeKarmadaAPIServerDeployment_SplitVolumesAndSecrets(t *testing.T) {
	opt := CommandInitOption{Namespace: "karmada", SecretLayout: "split"}
	dep := opt.makeKarmadaAPIServerDeployment()
	ps := dep.Spec.Template.Spec

	// volume mounts present with expected mount paths
	wantMounts := map[string]string{
		serverCertVolumeName:           serverCertVolumeMountPath,
		etcdClientCertVolumeName:       etcdClientCertVolumeMountPath,
		frontProxyClientCertVolumeName: frontProxyClientCertVolumeMountPath,
		saKeyPairVolumeName:            saKeyPairVolumeMountPath,
	}
	for name, path := range wantMounts {
		found := false
		for _, m := range ps.Containers[0].VolumeMounts {
			if m.Name == name && m.MountPath == path {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("split: missing VolumeMount %q at %q", name, path)
		}
	}

	// volumes should reference expected secret names
	wantSecrets := map[string]string{
		serverCertVolumeName:           certconst.SecretApiserverServer,
		etcdClientCertVolumeName:       certconst.SecretApiserverEtcdClient,
		frontProxyClientCertVolumeName: certconst.SecretApiserverFrontProxyClient,
		saKeyPairVolumeName:            certconst.SecretApiserverServiceAccountKeys,
	}
	for vname, sname := range wantSecrets {
		found := false
		for _, v := range ps.Volumes {
			if v.Name == vname && v.VolumeSource.Secret != nil && v.VolumeSource.Secret.SecretName == sname {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("split: volume %q missing or secretName != %q", vname, sname)
		}
	}
}

func TestCreateSplitCertsSecrets_CreatesExpectedSecrets(t *testing.T) {
	opt := CommandInitOption{
		Namespace:     "karmada",
		KubeClientSet: fake.NewClientset(),
	}
	// Minimal fake data for required certs/keys
	opt.CertAndKeyFileData = map[string][]byte{
		fmt.Sprintf("%s.crt", globalopt.CaCertAndKeyName):             []byte("CA"),
		fmt.Sprintf("%s.key", globalopt.CaCertAndKeyName):             []byte("CAK"),
		fmt.Sprintf("%s.crt", initopt.KarmadaCertAndKeyName):          []byte("KAR"),
		fmt.Sprintf("%s.key", initopt.KarmadaCertAndKeyName):          []byte("KARK"),
		fmt.Sprintf("%s.crt", initopt.ApiserverCertAndKeyName):        []byte("APS"),
		fmt.Sprintf("%s.key", initopt.ApiserverCertAndKeyName):        []byte("APSK"),
		fmt.Sprintf("%s.crt", initopt.FrontProxyCaCertAndKeyName):     []byte("FPCA"),
		fmt.Sprintf("%s.crt", initopt.FrontProxyClientCertAndKeyName): []byte("FPC"),
		fmt.Sprintf("%s.key", initopt.FrontProxyClientCertAndKeyName): []byte("FPCK"),
		fmt.Sprintf("%s.crt", initopt.EtcdCaCertAndKeyName):           []byte("ECA"),
		fmt.Sprintf("%s.crt", initopt.EtcdServerCertAndKeyName):       []byte("ESC"),
		fmt.Sprintf("%s.key", initopt.EtcdServerCertAndKeyName):       []byte("ESK"),
		fmt.Sprintf("%s.crt", initopt.EtcdClientCertAndKeyName):       []byte("ECC"),
		fmt.Sprintf("%s.key", initopt.EtcdClientCertAndKeyName):       []byte("ECK"),
	}

	if err := opt.createSplitCertsSecrets(); err != nil {
		t.Fatalf("createSplitCertsSecrets error: %v", err)
	}

	// pick representative secrets to assert
	secretNames := []string{
		certconst.SecretApiserverServer,
		certconst.SecretApiserverEtcdClient,
		certconst.SecretAggregatedAPIServerServer,
		certconst.SecretEtcdServer,
		certconst.SecretKubeControllerManagerCA,
		certconst.SecretWebhook,
		certconst.SecretSchedulerEstimatorClient,
		globalopt.KarmadaCertsName, // CA-only compat
	}
	for _, n := range secretNames {
		s, err := opt.KubeClientSet.CoreV1().Secrets(opt.Namespace).Get(context.Background(), n, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("expected secret %q to exist: %v", n, err)
		}
		switch n {
		case globalopt.KarmadaCertsName:
			if _, ok := s.StringData[certconst.KeyCACrt]; !ok {
				t.Fatalf("compat secret %q missing %q", n, certconst.KeyCACrt)
			}
		case certconst.SecretApiserverServiceAccountKeys, certconst.SecretKubeControllerManagerSAKeys:
			if _, ok := s.StringData[certconst.KeySAPrivate]; !ok {
				t.Fatalf("secret %q missing %q", n, certconst.KeySAPrivate)
			}
			if _, ok := s.StringData[certconst.KeySAPublic]; !ok {
				t.Fatalf("secret %q missing %q", n, certconst.KeySAPublic)
			}
		default:
			if _, ok := s.StringData[certconst.KeyTLSCrt]; !ok {
				t.Fatalf("secret %q missing %q", n, certconst.KeyTLSCrt)
			}
			if _, ok := s.StringData[certconst.KeyTLSKey]; !ok {
				t.Fatalf("secret %q missing %q", n, certconst.KeyTLSKey)
			}
		}
	}

	// also ensure a few config secrets exist (created from karmadaConfigList)
	confNames := []string{
		util.KarmadaConfigName(names.KarmadaAggregatedAPIServerComponentName),
		util.KarmadaConfigName(names.KubeControllerManagerComponentName),
	}
	for _, cn := range confNames {
		if _, err := opt.KubeClientSet.CoreV1().Secrets(opt.Namespace).Get(context.Background(), cn, metav1.GetOptions{}); err != nil {
			t.Fatalf("expected config secret %q to exist: %v", cn, err)
		}
	}
}
