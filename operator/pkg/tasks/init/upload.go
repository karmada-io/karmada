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
				Run:  runUploadKarmadaKubeconfig,
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

func runUploadKarmadaKubeconfig(r workflow.RunData) error {
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
	}

	kubeconfig, err := buildKubeConfigFromSpec(data, endpoint)
	if err != nil {
		return err
	}

	configBytes, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		return err
	}

	err = apiclient.CreateOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: data.GetNamespace(),
			Name:      util.KarmadaKubeconfigName,
			Labels:    constants.KarmadaOperatorLabel,
		},
		Data: map[string][]byte{constants.KarmadaKubeconfigSecretSubpath: configBytes},
	})
	if err != nil {
		return fmt.Errorf("failed to create secret of kubeconfig, err: %w", err)
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

func buildKubeConfigFromSpec(data InitData, serverURL string) (*clientcmdapi.Config, error) {
	ca := data.GetCert(constants.CaCertAndKeyName)
	if ca == nil {
		return nil, errors.New("unable build karmada kubeconfig, CA cert is empty")
	}

	client := data.GetCert(constants.KarmadaClientCertAndKeyName)
	if client == nil {
		return nil, errors.New("unable build karmada kubeconfig, karmada-client cert is empty")
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

// NewUploadCertsTask init a Upload-Certs task
func NewUploadCertsTask() workflow.Task {
	return workflow.Task{
		Name:        "Upload-Certs",
		Run:         runUploadCerts,
		RunSubTasks: true,
		Tasks: []workflow.Task{
			{
				Name: "Upload-KarmadaCert",
				Run:  runUploadKarmadaCert,
			},
			{
				Name: "Upload-EtcdCert",
				Run:  runUploadEtcdCert,
			},
			{
				Name: "Upload-WebHookCert",
				Run:  runUploadWebHookCert,
			},
		},
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

	ca := data.GetCert(constants.CaCertAndKeyName)
	karmadaServer := data.GetCert(constants.KarmadaServerCertAndKeyName)
	karmadaClient := data.GetCert(constants.KarmadaClientCertAndKeyName)
	frontProxyCa := data.GetCert(constants.FrontProxyCaCertAndKeyName)
	frontProxyClient := data.GetCert(constants.FrontProxyClientCertAndKeyName)

	err := apiclient.CreateOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.KarmadaCertsName,
			Namespace: data.GetNamespace(),
			Labels:    constants.KarmadaOperatorLabel,
		},
		Data: map[string][]byte{
			ca.CertName():               ca.CertData(),
			ca.KeyName():                ca.KeyData(),
			karmadaServer.CertName():    karmadaServer.CertData(),
			karmadaServer.KeyName():     karmadaServer.KeyData(),
			karmadaClient.CertName():    karmadaClient.CertData(),
			karmadaClient.KeyName():     karmadaClient.KeyData(),
			frontProxyCa.CertName():     frontProxyCa.CertData(),
			frontProxyCa.KeyName():      frontProxyCa.KeyData(),
			frontProxyClient.CertName(): frontProxyClient.CertData(),
			frontProxyClient.KeyName():  frontProxyClient.KeyData(),
		},
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
			Name:      util.KarmadaEtcdCertName,
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

	cert := data.GetCert(constants.KarmadaServerCertAndKeyName)
	err := apiclient.CreateOrUpdateSecret(data.RemoteClient(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.KarmadaWebhookCertName,
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
