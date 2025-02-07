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

package tasks

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/certs"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// NewUploadKubeconfigTask init a task to upload karmada kubeconfig and
// all of karmada certs to secret
func NewUploadKubeconfigTask() workflow.Task {
	return workflow.Task{
		Name:        "upload-config",
		RunSubTasks: true,
		Run:         runUploadKubeconfig,
		Tasks: []workflow.Task{
			{
				Name: "UploadAdminKubeconfig",
				Run:  runUploadAdminKubeconfig,
			},
		},
	}
}

func runUploadKubeconfig(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-config task invoked with an invalid data struct")
	}

	klog.V(4).InfoS("[upload-config] Running task", "karmada", klog.KObj(data))
	return nil
}

func runUploadAdminKubeconfig(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("UploadAdminKubeconfig task invoked with an invalid data struct")
	}

	var endpoint string
	switch data.Components().KarmadaAPIServer.ServiceType {
	case corev1.ServiceTypeClusterIP:
		apiserverName := util.KarmadaAPIServerName(data.GetName())
		endpoint = fmt.Sprintf("https://%s.%s.svc.cluster.local:%d", apiserverName, data.GetNamespace(), constants.KarmadaAPIserverListenClientPort)
	case corev1.ServiceTypeNodePort:
		service, err := apiclient.GetService(data.RemoteClient(), util.KarmadaAPIServerName(data.GetName()), data.GetNamespace())
		if err != nil {
			return err
		}
		nodePort := getNodePortFromAPIServerService(service)
		endpoint = fmt.Sprintf("https://%s:%d", data.ControlplaneAddress(), nodePort)
	case corev1.ServiceTypeLoadBalancer:
		service, err := apiclient.GetService(data.RemoteClient(), util.KarmadaAPIServerName(data.GetName()), data.GetNamespace())
		if err != nil {
			return err
		}
		if len(service.Status.LoadBalancer.Ingress) == 0 {
			return fmt.Errorf("no loadbalancer ingress found in service (%s/%s)", data.GetName(), data.GetNamespace())
		}
		loadbalancerAddress := getLoadbalancerAddress(service.Status.LoadBalancer.Ingress)
		if loadbalancerAddress == "" {
			return fmt.Errorf("can not find loadbalancer ip or hostname in service (%s/%s)", data.GetName(), data.GetNamespace())
		}
		endpoint = fmt.Sprintf("https://%s:%d", loadbalancerAddress, constants.KarmadaAPIserverListenClientPort)
	default:
		return errors.New("not supported service type for Karmada API server")
	}

	kubeconfig, err := buildKubeConfigFromSpec(data, endpoint)
	if err != nil {
		return err
	}

	configBytes, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return err
	}

	secretList := generateComponentKubeconfigSecrets(data, string(configBytes))

	for _, secret := range secretList {
		err = apiclient.CreateOrUpdateSecret(data.RemoteClient(), secret)
		if err != nil {
			return fmt.Errorf("failed to create/update karmada-config secret '%s', err: %w", secret.Name, err)
		}
	}

	// store rest config to RunData.
	config, err := clientcmd.RESTConfigFromKubeConfig(configBytes)
	if err != nil {
		return err
	}
	data.SetControlplaneConfig(config)

	klog.V(2).InfoS("[UploadAdminKubeconfig] Successfully created secret of karmada apiserver kubeconfig", "karmada", klog.KObj(data))
	return nil
}

func getNodePortFromAPIServerService(service *corev1.Service) int32 {
	var nodePort int32
	if service.Spec.Type == corev1.ServiceTypeNodePort {
		for _, port := range service.Spec.Ports {
			if port.Name != "client" {
				continue
			}
			nodePort = port.NodePort
		}
	}

	return nodePort
}

func getLoadbalancerAddress(ingress []corev1.LoadBalancerIngress) string {
	for _, in := range ingress {
		if in.Hostname != "" {
			return in.Hostname
		} else if in.IP != "" {
			return in.IP
		}
	}
	return ""
}

func buildKubeConfigFromSpec(data InitData, serverURL string) (*clientcmdapi.Config, error) {
	ca := data.GetCert(constants.CaCertAndKeyName)
	if ca == nil {
		return nil, errors.New("unable build karmada admin kubeconfig, CA cert is empty")
	}

	cc := certs.KarmadaCertClient()

	if err := mutateCertConfig(data, cc); err != nil {
		return nil, fmt.Errorf("error when mutate cert altNames for %s, err: %w", cc.Name, err)
	}
	client, err := certs.CreateCertAndKeyFilesWithCA(cc, ca.CertData(), ca.KeyData())
	if err != nil {
		return nil, fmt.Errorf("failed to generate karmada apiserver client certificate for kubeconfig, err: %w", err)
	}

	return util.CreateWithCerts(
		serverURL,
		constants.ClusterName,
		constants.UserName,
		ca.CertData(),
		client.KeyData(),
		client.CertData(),
	), nil
}

func generateKubeconfigSecret(name, namespace, configString string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    constants.KarmadaOperatorLabel,
		},
		StringData: map[string]string{"karmada.config": configString},
	}
}

func generateComponentKubeconfigSecrets(data InitData, configString string) []*corev1.Secret {
	var secrets []*corev1.Secret

	secrets = append(secrets, generateKubeconfigSecret(util.AdminKarmadaConfigSecretName(data.GetName()), data.GetNamespace(), configString))

	if data.Components() == nil {
		return secrets
	}

	componentList := map[string]interface{}{
		util.KarmadaAggregatedAPIServerName(data.GetName()): data.Components().KarmadaAggregatedAPIServer,
		util.KarmadaControllerManagerName(data.GetName()):   data.Components().KarmadaControllerManager,
		util.KubeControllerManagerName(data.GetName()):      data.Components().KubeControllerManager,
		util.KarmadaSchedulerName(data.GetName()):           data.Components().KarmadaScheduler,
		util.KarmadaDeschedulerName(data.GetName()):         data.Components().KarmadaDescheduler,
		util.KarmadaMetricsAdapterName(data.GetName()):      data.Components().KarmadaMetricsAdapter,
		util.KarmadaSearchName(data.GetName()):              data.Components().KarmadaSearch,
		util.KarmadaWebhookName(data.GetName()):             data.Components().KarmadaWebhook,
	}

	for karmadaComponentName, component := range componentList {
		if component != nil {
			secrets = append(secrets, generateKubeconfigSecret(util.ComponentKarmadaConfigSecretName(karmadaComponentName), data.GetNamespace(), configString))
		}
	}

	return secrets
}

// NewUploadCertsTask init a Upload-Certs task
func NewUploadCertsTask(karmada *operatorv1alpha1.Karmada) workflow.Task {
	tasks := []workflow.Task{
		{
			Name: "Upload-KarmadaCert",
			Run:  runUploadKarmadaCert,
		},
		{
			Name: "Upload-WebHookCert",
			Run:  runUploadWebHookCert,
		},
	}
	if karmada.Spec.Components.Etcd.Local != nil {
		uploadEtcdTask := workflow.Task{
			Name: "Upload-EtcdCert",
			Run:  runUploadEtcdCert,
		}
		tasks = append(tasks, uploadEtcdTask)
	}
	return workflow.Task{
		Name:        "Upload-Certs",
		Run:         runUploadCerts,
		RunSubTasks: true,
		Tasks:       tasks,
	}
}

func runUploadCerts(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-certs task invoked with an invalid data struct")
	}
	klog.V(4).InfoS("[upload-certs] Running upload-certs task", "karmada", klog.KObj(data))

	if len(data.CertList()) == 0 {
		return errors.New("there is no certs in store, please reload certs to store")
	}
	return nil
}

func runUploadKarmadaCert(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-KarmadaCert task invoked with an invalid data struct")
	}

	certList := data.CertList()
	certsData := make(map[string][]byte, len(certList))
	for _, c := range certList {
		certsData[c.KeyName()] = c.KeyData()
		certsData[c.CertName()] = c.CertData()
	}

	err := apiclient.CreateOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.KarmadaCertSecretName(data.GetName()),
			Namespace: data.GetNamespace(),
			Labels:    constants.KarmadaOperatorLabel,
		},
		Data: certsData,
	})
	if err != nil {
		return fmt.Errorf("failed to upload karmada cert to secret, err: %w", err)
	}

	klog.V(2).InfoS("[upload-KarmadaCert] Successfully uploaded karmada certs to secret", "karmada", klog.KObj(data))
	return nil
}

func runUploadEtcdCert(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-etcdCert task invoked with an invalid data struct")
	}

	ca := data.GetCert(constants.EtcdCaCertAndKeyName)
	server := data.GetCert(constants.EtcdServerCertAndKeyName)
	client := data.GetCert(constants.EtcdClientCertAndKeyName)

	err := apiclient.CreateOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: data.GetNamespace(),
			Name:      util.EtcdCertSecretName(data.GetName()),
			Labels:    constants.KarmadaOperatorLabel,
		},

		Data: map[string][]byte{
			ca.CertName():     ca.CertData(),
			ca.KeyName():      ca.KeyData(),
			server.CertName(): server.CertData(),
			server.KeyName():  server.KeyData(),
			client.CertName(): client.CertData(),
			client.KeyName():  client.KeyData(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upload etcd certs to secret, err: %w", err)
	}

	klog.V(2).InfoS("[upload-etcdCert] Successfully uploaded etcd certs to secret", "karmada", klog.KObj(data))
	return nil
}

func runUploadWebHookCert(r workflow.RunData) error {
	data, ok := r.(InitData)
	if !ok {
		return errors.New("upload-webhookCert task invoked with an invalid data struct")
	}

	cert := data.GetCert(constants.KarmadaCertAndKeyName)
	err := apiclient.CreateOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.WebhookCertSecretName(data.GetName()),
			Namespace: data.GetNamespace(),
			Labels:    constants.KarmadaOperatorLabel,
		},

		Data: map[string][]byte{
			"tls.key": cert.KeyData(),
			"tls.crt": cert.CertData(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upload webhook certs to secret, err: %w", err)
	}

	klog.V(2).InfoS("[upload-webhookCert] Successfully uploaded webhook certs to secret", "karmada", klog.KObj(data))
	return nil
}
