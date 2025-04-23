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

package tasks

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/certs"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/controlplane/apiserver"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestNewUploadKubeconfigTask(t *testing.T) {
	tests := []struct {
		name     string
		wantTask workflow.Task
	}{
		{
			name: "NewUploadKubeconfigTask",
			wantTask: workflow.Task{
				Name:        "upload-config",
				RunSubTasks: true,
				Run:         runUploadKubeconfig,
				Tasks: []workflow.Task{
					{
						Name: "UploadAdminKubeconfig",
						Run:  runUploadAdminKubeconfig,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uploadKubeconfigTask := NewUploadKubeconfigTask()
			err := util.DeepEqualTasks(uploadKubeconfigTask, test.wantTask)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunUploadKubeconfig(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "upload-config task invoked with an invalid data struct",
		},
		{
			name: "ValidTypeAssertion_TypeAssertionSucceeded",
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runUploadKubeconfig(test.runData)
			if err == nil && test.wantErr {
				t.Errorf("expected error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
		})
	}
}

func TestRunUploadAdminKubeconfig(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	controlPlaneAddress := "10.96.1.5"
	apiServerComponentName := util.KarmadaAPIServerName(name)
	apiServerPort := constants.KarmadaAPIserverListenClientPort
	tests := []struct {
		name             string
		runData          workflow.RunData
		endpoingExpected string
		prep             func(workflow.RunData) error
		verify           func(workflow.RunData, string) error
		wantErr          bool
		errMsg           string
	}{
		{
			name:    "InvalidTypeAssertion_TypeAssertionFailed",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			verify:  func(workflow.RunData, string) error { return nil },
			wantErr: true,
			errMsg:  "UploadAdminKubeconfig task invoked with an invalid data struct",
		},
		{
			name: "WithKarmadaAPIServerClusterIPServiceType_SecretCreated",
			runData: &TestInitData{
				Name:      name,
				Namespace: namespace,
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
						ServiceType: corev1.ServiceTypeClusterIP,
					},
				},
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
			},
			endpoingExpected: fmt.Sprintf("https://%s.%s.svc.cluster.local:%d", apiServerComponentName, namespace, apiServerPort),
			prep:             createCA,
			verify: func(rd workflow.RunData, endpoint string) error {
				data := rd.(*TestInitData)
				client := data.RemoteClient().(*fakeclientset.Clientset)
				if err := verifySecret(client, name, namespace, endpoint); err != nil {
					return fmt.Errorf("failed to verify secret: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "WithKarmadaAPIServerNodePortServiceType_SecretCreated",
			runData: &TestInitData{
				Name:      name,
				Namespace: namespace,
				ComponentsUnits: &operatorv1alpha1.KarmadaComponents{
					KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
						CommonSettings: operatorv1alpha1.CommonSettings{
							Image:           operatorv1alpha1.Image{ImageTag: "karmada-apiserver-image"},
							Replicas:        ptr.To[int32](3),
							Annotations:     map[string]string{"annotationKey": "annotationValue"},
							Labels:          map[string]string{"labelKey": "labelValue"},
							Resources:       corev1.ResourceRequirements{},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
						ServiceSubnet: ptr.To("10.96.0.0/12"),
						ExtraArgs:     map[string]string{"cmd1": "arg1", "cmd2": "arg2"},
						ServiceType:   corev1.ServiceTypeNodePort,
					},
					Etcd: &operatorv1alpha1.Etcd{
						Local: &operatorv1alpha1.LocalEtcd{},
					},
				},
				RemoteClientConnector: fakeclientset.NewSimpleClientset(),
				ControlplaneAddr:      controlPlaneAddress,
			},
			endpoingExpected: fmt.Sprintf("https://%s:0", controlPlaneAddress),
			prep: func(rd workflow.RunData) error {
				// Create a certificate authority to generate the Karmada admin kubeconfig.
				if err := createCA(rd); err != nil {
					return err
				}

				// Install the Karmada API Server. This is necessary to ensure
				// the API Server is available when uploading the admin kubeconfig secret,
				// particularly when the service type is set to NodePort.
				data := rd.(*TestInitData)
				client := data.RemoteClient()
				if err := apiserver.EnsureKarmadaAPIServer(client, data.Components(), name, namespace, map[string]bool{}); err != nil {
					return fmt.Errorf("failed to install karmada api server: %v", err)
				}

				return nil
			},
			verify: func(rd workflow.RunData, endpoint string) error {
				data := rd.(*TestInitData)
				client := data.RemoteClient().(*fakeclientset.Clientset)
				if err := verifySecret(client, name, namespace, endpoint); err != nil {
					return fmt.Errorf("failed to verify secret: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed tp prep uploading admin kubeconfig: %v", err)
			}
			err := runUploadAdminKubeconfig(test.runData)
			if err == nil && test.wantErr {
				t.Errorf("expected error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
			if err := test.verify(test.runData, test.endpoingExpected); err != nil {
				t.Errorf("failed to run upload admin kubeconfig task: %v", err)
			}
		})
	}
}

// verifySecret checks whether the admin kubeconfig secret for the specified Karmada instance
// has been created in the given namespace, and verifies its fields against expected values.
//
// It ensures that:
//   - The secret exists and has the correct name.
//   - The Karmada operator label is present and has the correct value.
//   - The kubeconfig stored in the secret contains the expected API server endpoint.
//
// Parameters:
//
//	client:           The fake Kubernetes clientset to use for retrieving the secret.
//	karmadaInstanceName:  The name of the Karmada instance whose admin kubeconfig secret should be verified.
//	namespace:        The namespace in which the secret is expected to exist.
//	expectedEndpoint: The expected API server endpoint URL in the kubeconfig.
//
// Returns:
//
//	An error if the secret does not exist, or any of the checks fail, otherwise nil.
func verifySecret(client *fakeclientset.Clientset, karmadaInstanceName, namespace, expectedEndpoint string) error {
	// Get Karmada admin config secret.
	secretName := util.AdminKarmadaConfigSecretName(karmadaInstanceName)
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get %s secret in %s namespace: %v", secretName, namespace, err)
	}

	// Check for the Karmada operator label and verify its value.
	value, ok := secret.Labels[constants.KarmadaOperatorLabelKeyName]
	if !ok {
		return fmt.Errorf("expected %s label to exist in %s secret in %s namespace", constants.KarmadaOperatorLabelKeyName, secretName, namespace)
	}
	if value != constants.KarmadaOperator {
		return fmt.Errorf("expected %s secret label value to be %s", value, constants.KarmadaOperator)
	}

	// Extract and load the kubeconfig from the secret.
	karmadaConfig := []byte(secret.StringData["karmada.config"])
	config, err := clientcmd.Load(karmadaConfig)
	if err != nil {
		return fmt.Errorf("failed to load secret kubeconfig data: %v", err)
	}

	// Get the Karmada APIServer endpoint from the loaded kubeconfig and verify it.
	gotEndpoint := config.Clusters[config.Contexts[config.CurrentContext].Cluster].Server
	if gotEndpoint != expectedEndpoint {
		return fmt.Errorf("expected endpoint %s, but got %s", expectedEndpoint, gotEndpoint)
	}

	return nil
}

// createCA creates a new certificate authority and append it to the cert store.
func createCA(rd workflow.RunData) error {
	data := rd.(*TestInitData)
	newCA, err := certs.NewCertificateAuthority(certs.KarmadaCertRootCA())
	if err != nil {
		return fmt.Errorf("failed to create certificate authority: %v", err)
	}
	data.AddCert(newCA)
	return nil
}
