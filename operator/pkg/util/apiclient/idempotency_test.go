/*
Copyright 2024 The Karmada Authors.

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

package apiclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	crdsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	crdsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	fakeAggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"

	"github.com/karmada-io/karmada/test/helper"
)

func TestCreateNamespace(t *testing.T) {
	namespace := "test"
	tests := []struct {
		name    string
		client  clientset.Interface
		ns      *corev1.Namespace
		prep    func(clientset.Interface, *corev1.Namespace) error
		verify  func(clientset.Interface, *corev1.Namespace) error
		wantErr bool
		errMsg  string
	}{
		{
			name:   "CreateNamespace_WithNetworkIssue_FailedToGetNamespace",
			client: fakeclientset.NewSimpleClientset(),
			ns:     helper.NewNamespace(namespace),
			prep: func(c clientset.Interface, _ *corev1.Namespace) error {
				return simulateNetworkErrorOnOp(c, "get", "namespaces")
			},
			verify: func(c clientset.Interface, ns *corev1.Namespace) error {
				_, err := c.CoreV1().Namespaces().Get(context.TODO(), ns.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("namespace creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while get the namespaces",
		},
		{
			name:   "CreateNamespace_NotFound_NamespaceCreated",
			client: fakeclientset.NewSimpleClientset(),
			ns:     helper.NewNamespace(namespace),
			prep:   func(clientset.Interface, *corev1.Namespace) error { return nil },
			verify: func(c clientset.Interface, ns *corev1.Namespace) error {
				_, err := c.CoreV1().Namespaces().Get(context.TODO(), ns.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected namespace to be created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
		{
			name:   "CreateNamespace_Found_GetExistingNamespace",
			client: fakeclientset.NewSimpleClientset(),
			ns:     helper.NewNamespace(namespace),
			prep: func(c clientset.Interface, ns *corev1.Namespace) error {
				if _, err := c.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{}); err != nil {
					return fmt.Errorf("failed to create namespace: %v", err)
				}
				return nil
			},
			verify: func(c clientset.Interface, ns *corev1.Namespace) error {
				_, err := c.CoreV1().Namespaces().Get(context.TODO(), ns.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected namespace to be created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.ns); err != nil {
				t.Errorf("failed to prep creating namespace, got err: %v", err)
			}
			err := CreateNamespace(test.client, test.ns)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.ns); err != nil {
				t.Errorf("failed to verify creating namespace, got err: %v", err)
			}
		})
	}
}

func TestCreateOrUpdateSecret(t *testing.T) {
	name, namespace := "test-secret", "test"
	tests := []struct {
		name    string
		client  clientset.Interface
		secret  *corev1.Secret
		prep    func(clientset.Interface, *corev1.Secret) error
		verify  func(clientset.Interface, *corev1.Secret) error
		wantErr bool
		errMsg  string
	}{
		{
			name:   "CreateOrUpdateSecret_WithNetworkIssue_FailedToCreateSecret",
			client: fakeclientset.NewSimpleClientset(),
			secret: helper.NewSecret(namespace, name, map[string][]byte{}),
			prep: func(c clientset.Interface, _ *corev1.Secret) error {
				return simulateNetworkErrorOnOp(c, "create", "secrets")
			},
			verify: func(c clientset.Interface, s *corev1.Secret) error {
				_, err := c.CoreV1().Secrets(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("secret creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while create the secrets",
		},
		{
			name:   "CreateOrUpdateSecret_SecretAlreadyExists_SecretUpdated",
			client: fakeclientset.NewSimpleClientset(),
			secret: helper.NewSecret(namespace, name, map[string][]byte{}),
			prep: func(c clientset.Interface, s *corev1.Secret) error {
				s.UID = "640ab30c"
				defer func() {
					s.UID = "640ab30a"
				}()
				_, err := c.CoreV1().Secrets(s.GetNamespace()).Create(context.TODO(), s, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create secret, got err: %v", err)
				}
				return nil
			},
			verify:  verifySecretUpdated,
			wantErr: false,
		},
		{
			name:   "CreateOrUpdateSecret_NotExist_SecretCreated",
			client: fakeclientset.NewSimpleClientset(),
			secret: helper.NewSecret(namespace, name, map[string][]byte{}),
			prep:   func(clientset.Interface, *corev1.Secret) error { return nil },
			verify: func(c clientset.Interface, s *corev1.Secret) error {
				_, err := c.CoreV1().Secrets(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected secret to be already created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.secret); err != nil {
				t.Errorf("failed to prep creating/updating secret, got err: %v", err)
			}
			err := CreateOrUpdateSecret(test.client, test.secret)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.secret); err != nil {
				t.Errorf("failed to verify creating/updating secret, got err: %v", err)
			}
		})
	}
}

func TestCreateOrUpdateService(t *testing.T) {
	name, namespace := "test-service", "test"
	serviceType := corev1.ServiceTypeClusterIP
	tests := []struct {
		name    string
		client  clientset.Interface
		service *corev1.Service
		prep    func(clientset.Interface, *corev1.Service) error
		verify  func(clientset.Interface, *corev1.Service) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "CreateOrUpdateService_WithNetworkIssue_FailedToCreateService",
			client:  fakeclientset.NewSimpleClientset(),
			service: helper.NewService(namespace, name, serviceType),
			prep: func(c clientset.Interface, _ *corev1.Service) error {
				return simulateNetworkErrorOnOp(c, "create", "services")
			},
			verify: func(c clientset.Interface, s *corev1.Service) error {
				_, err := c.CoreV1().Services(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("service creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while create the services",
		},
		{
			name:    "CreateOrUpdateService_ServiceAlreadyExists_ServiceUpdated",
			client:  fakeclientset.NewSimpleClientset(),
			service: helper.NewService(namespace, name, serviceType),
			prep: func(c clientset.Interface, s *corev1.Service) error {
				s.ResourceVersion, s.Spec.ClusterIP, s.Spec.ClusterIPs = "value1", "10.12.84.156", []string{"10.12.84.156"}
				defer func() {
					s.ResourceVersion, s.Spec.ClusterIP, s.Spec.ClusterIPs = "", "", []string{}
				}()
				_, err := c.CoreV1().Services(s.GetNamespace()).Create(context.TODO(), s, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create service, got err: %v", err)
				}
				return nil
			},
			verify:  verifyServiceUpdated,
			wantErr: false,
		},
		{
			name:    "CreateOrUpdateService_NotExist_SecretCreated",
			client:  fakeclientset.NewSimpleClientset(),
			service: helper.NewService(namespace, name, serviceType),
			prep:    func(clientset.Interface, *corev1.Service) error { return nil },
			verify: func(c clientset.Interface, s *corev1.Service) error {
				_, err := c.CoreV1().Services(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected service to be already created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.service); err != nil {
				t.Errorf("failed to prep creating/updating service, got err: %v", err)
			}
			err := CreateOrUpdateService(test.client, test.service)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.service); err != nil {
				t.Errorf("failed to verify creating/updating service, got err: %v", err)
			}
		})
	}
}

func TestCreateOrUpdateDeployment(t *testing.T) {
	name, namespace := "test-deployment", "test"
	tests := []struct {
		name       string
		client     clientset.Interface
		deployment *appsv1.Deployment
		prep       func(clientset.Interface, *appsv1.Deployment) error
		verify     func(clientset.Interface, *appsv1.Deployment) error
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "CreateOrUpdateDeployment_WithNetworkIssue_FailedToCreateDeployment",
			client:     fakeclientset.NewSimpleClientset(),
			deployment: helper.NewDeployment(namespace, name),
			prep: func(c clientset.Interface, _ *appsv1.Deployment) error {
				return simulateNetworkErrorOnOp(c, "create", "deployments")
			},
			verify: func(c clientset.Interface, d *appsv1.Deployment) error {
				_, err := c.CoreV1().Secrets(d.GetNamespace()).Get(context.TODO(), d.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("deployment creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while create the deployments",
		},
		{
			name:       "CreateOrUpdateDeployment_DeploymentAlreadyExists_DeploymentUpdated",
			client:     fakeclientset.NewSimpleClientset(),
			deployment: helper.NewDeployment(namespace, name),
			prep: func(c clientset.Interface, d *appsv1.Deployment) error {
				d.UID = "640ab30c"
				defer func() {
					d.UID = "640ab30a"
				}()
				_, err := c.AppsV1().Deployments(d.GetNamespace()).Create(context.TODO(), d, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create deploymnet, got err: %v", err)
				}
				return nil
			},
			verify:  verifyDeploymentUpdated,
			wantErr: false,
		},
		{
			name:       "CreateOrUpdateDeployment_NotExist_DeploymentCreated",
			client:     fakeclientset.NewSimpleClientset(),
			deployment: helper.NewDeployment(namespace, name),
			prep:       func(clientset.Interface, *appsv1.Deployment) error { return nil },
			verify: func(c clientset.Interface, d *appsv1.Deployment) error {
				_, err := c.AppsV1().Deployments(d.GetNamespace()).Get(context.TODO(), d.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected deployment to be already created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.deployment); err != nil {
				t.Errorf("failed to prep creating/updating secret: %v", err)
			}
			err := CreateOrUpdateDeployment(test.client, test.deployment)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.deployment); err != nil {
				t.Errorf("failed to verify creating/updating secret, got err: %v", err)
			}
		})
	}
}

func TestCreateOrUpdateMutatingWebhookConfiguration(t *testing.T) {
	name := "test-mwc"
	tests := []struct {
		name    string
		client  clientset.Interface
		mwc     *admissionregistrationv1.MutatingWebhookConfiguration
		prep    func(clientset.Interface, *admissionregistrationv1.MutatingWebhookConfiguration) error
		verify  func(clientset.Interface, *admissionregistrationv1.MutatingWebhookConfiguration) error
		wantErr bool
		errMsg  string
	}{
		{
			name:   "CreateOrUpdateMutatingWebhookConfiguration_WithNetworkIssue_FailedToCreateMutatingWebhookConfiguration",
			client: fakeclientset.NewSimpleClientset(),
			mwc: &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{
					{
						Name: "imagepolicy.kubernetes.io",
					},
				},
			},
			prep: func(c clientset.Interface, _ *admissionregistrationv1.MutatingWebhookConfiguration) error {
				return simulateNetworkErrorOnOp(c, "create", "mutatingwebhookconfigurations")
			},
			verify: func(c clientset.Interface, mwc *admissionregistrationv1.MutatingWebhookConfiguration) error {
				_, err := c.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.TODO(), mwc.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("mutating wenhook configuration creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while create the mutatingwebhookconfigurations",
		},
		{
			name:   "CreateOrUpdateMutatingWebhookConfiguration_AlreadyExists_MutatingWebhookConfigurationUpdated",
			client: fakeclientset.NewSimpleClientset(),
			mwc: &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{
					{
						Name: "imagepolicy.kubernetes.io",
					},
				},
			},
			prep: func(c clientset.Interface, mwc *admissionregistrationv1.MutatingWebhookConfiguration) error {
				mwc.ResourceVersion = "value1"
				defer func() {
					mwc.ResourceVersion = ""
				}()
				_, err := c.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), mwc, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create mutating webhook configuration, got err: %v", err)
				}
				return nil
			},
			verify:  verifyMutatingWebhookConfigurationUpdated,
			wantErr: false,
		},
		{
			name:   "CreateOrUpdateMutatingWebhookConfiguration_NotExist_MutatingWebhookConfigurationCreated",
			client: fakeclientset.NewSimpleClientset(),
			mwc: &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{
					{
						Name: "imagepolicy.kubernetes.io",
					},
				},
			},
			prep: func(clientset.Interface, *admissionregistrationv1.MutatingWebhookConfiguration) error {
				return nil
			},
			verify: func(c clientset.Interface, mwc *admissionregistrationv1.MutatingWebhookConfiguration) error {
				_, err := c.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.TODO(), mwc.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected mutating webhook configuration to be already created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.mwc); err != nil {
				t.Errorf("failed to prep creating/updating mutating webhook configuration: %v", err)
			}
			err := CreateOrUpdateMutatingWebhookConfiguration(test.client, test.mwc)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.mwc); err != nil {
				t.Errorf("failed to verify creating/updating mutating webhook configuration, got err: %v", err)
			}
		})
	}
}

func TestCreateOrUpdateValidatingWebhookConfiguration(t *testing.T) {
	name := "test-vwc"
	tests := []struct {
		name    string
		client  clientset.Interface
		vwc     *admissionregistrationv1.ValidatingWebhookConfiguration
		prep    func(clientset.Interface, *admissionregistrationv1.ValidatingWebhookConfiguration) error
		verify  func(clientset.Interface, *admissionregistrationv1.ValidatingWebhookConfiguration) error
		wantErr bool
		errMsg  string
	}{
		{
			name:   "CreateOrUpdateValidatingWebhookConfiguration_WithNetworkIssue_FailedToCreateValidatingWebhookConfiguration",
			client: fakeclientset.NewSimpleClientset(),
			vwc: &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Webhooks: []admissionregistrationv1.ValidatingWebhook{
					{
						Name: "imagepolicy.kubernetes.io",
					},
				},
			},
			prep: func(c clientset.Interface, _ *admissionregistrationv1.ValidatingWebhookConfiguration) error {
				return simulateNetworkErrorOnOp(c, "create", "validatingwebhookconfigurations")
			},
			verify: func(c clientset.Interface, mwc *admissionregistrationv1.ValidatingWebhookConfiguration) error {
				_, err := c.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.TODO(), mwc.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("validating wenhook configuration creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while create the validatingwebhookconfigurations",
		},
		{
			name:   "CreateOrUpdateValidatingWebhookConfiguration_AlreadyExists_ValidatingWebhookConfigurationUpdated",
			client: fakeclientset.NewSimpleClientset(),
			vwc: &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Webhooks: []admissionregistrationv1.ValidatingWebhook{
					{
						Name: "imagepolicy.kubernetes.io",
					},
				},
			},
			prep: func(c clientset.Interface, vwc *admissionregistrationv1.ValidatingWebhookConfiguration) error {
				vwc.ResourceVersion = "value1"
				defer func() {
					vwc.ResourceVersion = ""
				}()
				_, err := c.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.TODO(), vwc, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create validating webhook configuration, got err: %v", err)
				}
				return nil
			},
			verify:  verifyValidatingWebhookConfigurationUpdated,
			wantErr: false,
		},
		{
			name:   "CreateOrUpdateValidatingWebhookConfiguration_NotExist_ValidatingWebhookConfigurationCreated",
			client: fakeclientset.NewSimpleClientset(),
			vwc: &admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Webhooks: []admissionregistrationv1.ValidatingWebhook{
					{
						Name: "imagepolicy.kubernetes.io",
					},
				},
			},
			prep: func(clientset.Interface, *admissionregistrationv1.ValidatingWebhookConfiguration) error {
				return nil
			},
			verify: func(c clientset.Interface, vwc *admissionregistrationv1.ValidatingWebhookConfiguration) error {
				_, err := c.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.TODO(), vwc.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected validating webhook configuration to be already created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.vwc); err != nil {
				t.Errorf("failed to prep creating/updating validating webhook configuration: %v", err)
			}
			err := CreateOrUpdateValidatingWebhookConfiguration(test.client, test.vwc)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.vwc); err != nil {
				t.Errorf("failed to verify creating/updating validating webhook configuration, got err: %v", err)
			}
		})
	}
}

func TestCreateOrUpdateAPIService(t *testing.T) {
	name := "test-apiservice"
	tests := []struct {
		name       string
		client     aggregator.Interface
		apiservice *apiregistrationv1.APIService
		prep       func(aggregator.Interface, *apiregistrationv1.APIService) error
		verify     func(aggregator.Interface, *apiregistrationv1.APIService) error
		wantErr    bool
		errMsg     string
	}{
		{
			name:   "CreateOrUpdateAPIService_WithNetworkIssue_FailedToCreateAPIService",
			client: fakeAggregator.NewSimpleClientset(),
			apiservice: &apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiregistrationv1.APIServiceSpec{},
			},
			prep: func(a aggregator.Interface, _ *apiregistrationv1.APIService) error {
				return simulateNetworkErrorAggregatorClientOnOp(a, "create", "apiservices")
			},
			verify: func(a aggregator.Interface, s *apiregistrationv1.APIService) error {
				_, err := a.ApiregistrationV1().APIServices().Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("api service creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while create the apiservices",
		},
		{
			name:   "CreateOrUpdateAPIService_APIServiceAlreadyExists_APIServiceUpdated",
			client: fakeAggregator.NewSimpleClientset(),
			apiservice: &apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiregistrationv1.APIServiceSpec{},
			},
			prep: func(a aggregator.Interface, s *apiregistrationv1.APIService) error {
				s.ResourceVersion = "value1"
				defer func() {
					s.ResourceVersion = ""
				}()
				_, err := a.ApiregistrationV1().APIServices().Create(context.TODO(), s, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create api service, got err: %v", err)
				}
				return nil
			},
			verify:  verifyAPIServiceUpdated,
			wantErr: false,
		},
		{
			name:   "CreateOrUpdateAPIService_NotExist_APIServiceCreated",
			client: fakeAggregator.NewSimpleClientset(),
			apiservice: &apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiregistrationv1.APIServiceSpec{},
			},
			prep: func(aggregator.Interface, *apiregistrationv1.APIService) error {
				return nil
			},
			verify: func(a aggregator.Interface, s *apiregistrationv1.APIService) error {
				_, err := a.ApiregistrationV1().APIServices().Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected api service to be already created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.apiservice); err != nil {
				t.Errorf("failed to prep creating/updating api service: %v", err)
			}
			err := CreateOrUpdateAPIService(test.client, test.apiservice)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.apiservice); err != nil {
				t.Errorf("failed to verify creating/updating the api service, got err: %v", err)
			}
		})
	}
}

func TestCreateCustomResourceDefinitionIfNeed(t *testing.T) {
	name := "test-crd"
	tests := []struct {
		name    string
		client  crdsclient.Interface
		crd     *apiextensionsv1.CustomResourceDefinition
		prep    func(crdsclient.Interface, *apiextensionsv1.CustomResourceDefinition) error
		verify  func(crdsclient.Interface, *apiextensionsv1.CustomResourceDefinition) error
		wantErr bool
		errMsg  string
	}{
		{
			name:   "CreateCustomResourceDefinitionIfNeed_WithNetworkIssue_FailedToCreateCRD",
			client: crdsfake.NewSimpleClientset(),
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{},
			},
			prep: func(c crdsclient.Interface, _ *apiextensionsv1.CustomResourceDefinition) error {
				return simulateNetworkErrorCRDClientOnOp(c, "create", "customresourcedefinitions")
			},
			verify: func(c crdsclient.Interface, crd *apiextensionsv1.CustomResourceDefinition) error {
				_, err := c.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crd.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("custom resource definition creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while create the customresourcedefinitions",
		},
		{
			name:   "CreateOrUpdateCustomResourceDefinition_AlreadyExists_CRDCreationSkipped",
			client: crdsfake.NewSimpleClientset(),
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{},
			},
			prep: func(c crdsclient.Interface, crd *apiextensionsv1.CustomResourceDefinition) error {
				_, err := c.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), crd, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create custom resource definition, got err: %v", err)
				}
				return nil
			},
			verify: func(crdsclient.Interface, *apiextensionsv1.CustomResourceDefinition) error {
				return nil
			},
			wantErr: false,
		},
		{
			name:   "CreateOrUpdateCustomResourceDefinition_AlreadyExists_CRDCreated",
			client: crdsfake.NewSimpleClientset(),
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{},
			},
			prep: func(crdsclient.Interface, *apiextensionsv1.CustomResourceDefinition) error {
				return nil
			},
			verify: func(c crdsclient.Interface, crd *apiextensionsv1.CustomResourceDefinition) error {
				_, err := c.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crd.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected api service to be already created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.crd); err != nil {
				t.Errorf("failed to prep creating custom resource definition: %v", err)
			}
			err := CreateCustomResourceDefinitionIfNeed(test.client, test.crd)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.crd); err != nil {
				t.Errorf("failed to verify creating custom resource definition, got err: %v", err)
			}
		})
	}
}

func TestPatchCustomResourceDefinition(t *testing.T) {
	name := "test-crd"
	tests := []struct {
		name      string
		client    crdsclient.Interface
		crd       *apiextensionsv1.CustomResourceDefinition
		patchData []byte
		prep      func(crdsclient.Interface, *apiextensionsv1.CustomResourceDefinition) error
		verify    func(crdsclient.Interface, *apiextensionsv1.CustomResourceDefinition) error
		wantErr   bool
	}{
		{
			name:   "PatchCustomResourceDefinition_PatchData",
			client: crdsfake.NewSimpleClientset(),
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{},
			},
			patchData: []byte(`{"metadata":{"labels":{"new-label":"new-value"}}}`),
			prep: func(c crdsclient.Interface, crd *apiextensionsv1.CustomResourceDefinition) error {
				_, err := c.ApiextensionsV1().CustomResourceDefinitions().Create(context.TODO(), crd, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create custom resource definition, got err: %v", err)
				}
				return nil
			},
			verify: func(c crdsclient.Interface, crd *apiextensionsv1.CustomResourceDefinition) error {
				gotCRD, err := c.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crd.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected api service to be already created, but got err: %v", err)
				}

				labelExpected, labelValueExpected := "new-label", "new-value"
				if _, ok := gotCRD.Labels[labelExpected]; !ok {
					return fmt.Errorf("expected label key '%s' to be in labels %v", labelExpected, gotCRD.Labels)
				}
				if gotCRD.Labels[labelExpected] != labelValueExpected {
					return fmt.Errorf("mismatch values, expected label value %s but got %s", labelValueExpected, gotCRD.Labels[labelExpected])
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.crd); err != nil {
				t.Errorf("failed to prep patching custom resource definition: %v", err)
			}
			err := PatchCustomResourceDefinition(test.client, test.crd.GetName(), test.patchData)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err := test.verify(test.client, test.crd); err != nil {
				t.Errorf("failed to verify patching custom resource definition, got err: %v", err)
			}
		})
	}
}

func TestCreateOrUpdateStatefulSet(t *testing.T) {
	name, namespace := "test-statefulset", "test"
	tests := []struct {
		name        string
		client      clientset.Interface
		statefulSet *appsv1.StatefulSet
		prep        func(clientset.Interface, *appsv1.StatefulSet) error
		verify      func(clientset.Interface, *appsv1.StatefulSet) error
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "CreateOrUpdateStatefulSet_WithNetworkIssue_FailedToCreateStatefulSet",
			client:      fakeclientset.NewSimpleClientset(),
			statefulSet: helper.NewStatefulSet(namespace, name),
			prep: func(c clientset.Interface, _ *appsv1.StatefulSet) error {
				return simulateNetworkErrorOnOp(c, "create", "statefulsets")
			},
			verify: func(c clientset.Interface, s *appsv1.StatefulSet) error {
				_, err := c.AppsV1().StatefulSets(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("statefulset creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while create the statefulsets",
		},
		{
			name:        "CreateOrUpdateStatefulSet_StatefulSetAlreadyExists_StatefulSetUpdated",
			client:      fakeclientset.NewSimpleClientset(),
			statefulSet: helper.NewStatefulSet(namespace, name),
			prep: func(c clientset.Interface, s *appsv1.StatefulSet) error {
				s.SetResourceVersion("value1")
				defer func() {
					s.SetResourceVersion("")
				}()
				_, err := c.AppsV1().StatefulSets(s.GetNamespace()).Create(context.TODO(), s, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create statefulset, got err: %v", err)
				}
				return nil
			},
			verify:  verifyStatefulSetUpdated,
			wantErr: false,
		},
		{
			name:        "CreateOrUpdateStatefulSet_NotExist_StatefulSetCreated",
			client:      fakeclientset.NewSimpleClientset(),
			statefulSet: helper.NewStatefulSet(namespace, name),
			prep:        func(clientset.Interface, *appsv1.StatefulSet) error { return nil },
			verify: func(c clientset.Interface, s *appsv1.StatefulSet) error {
				_, err := c.AppsV1().StatefulSets(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected statefulset to be already created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.statefulSet); err != nil {
				t.Errorf("failed to prep creating/updating statefulset: %v", err)
			}
			err := CreateOrUpdateStatefulSet(test.client, test.statefulSet)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.statefulSet); err != nil {
				t.Errorf("failed to verify creating/updating statefulset, got err: %v", err)
			}
		})
	}
}

func TestCreateOrUpdateClusterRole(t *testing.T) {
	name := "test-clusterrole"
	tests := []struct {
		name        string
		client      clientset.Interface
		clusterRole *rbacv1.ClusterRole
		prep        func(clientset.Interface, *rbacv1.ClusterRole) error
		verify      func(clientset.Interface, *rbacv1.ClusterRole) error
		wantErr     bool
		errMsg      string
	}{
		{
			name:   "CreateOrUpdateClusterRole_WithNetworkIssue_FailedToCreateClusterRole",
			client: fakeclientset.NewSimpleClientset(),
			clusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Verbs:     []string{"get"},
						Resources: []string{"nodes"},
					},
				},
			},
			prep: func(c clientset.Interface, _ *rbacv1.ClusterRole) error {
				return simulateNetworkErrorOnOp(c, "create", "clusterroles")
			},
			verify: func(c clientset.Interface, r *rbacv1.ClusterRole) error {
				_, err := c.RbacV1().ClusterRoles().Get(context.TODO(), r.GetName(), metav1.GetOptions{})
				if err == nil {
					return errors.New("clusterrole creation should have failed as expected due to a network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while create the clusterroles",
		},
		{
			name:   "CreateOrUpdateClusterRole_ClusterRoleAlreadyExists_ClusterRoleUpdated",
			client: fakeclientset.NewSimpleClientset(),
			clusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Verbs:     []string{"get"},
						Resources: []string{"nodes"},
					},
				},
			},
			prep: func(c clientset.Interface, r *rbacv1.ClusterRole) error {
				r.SetResourceVersion("value1")
				defer func() {
					r.SetResourceVersion("")
				}()
				_, err := c.RbacV1().ClusterRoles().Create(context.TODO(), r, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create clusterrole, got err: %v", err)
				}
				return nil
			},
			verify:  verifyClusterRoleUpdated,
			wantErr: false,
		},
		{
			name:   "CreateOrUpdateClusterRole_NotExist_ClusterRoleCreated",
			client: fakeclientset.NewSimpleClientset(),
			clusterRole: &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Verbs:     []string{"get"},
						Resources: []string{"nodes"},
					},
				},
			},
			prep: func(clientset.Interface, *rbacv1.ClusterRole) error { return nil },
			verify: func(c clientset.Interface, r *rbacv1.ClusterRole) error {
				_, err := c.RbacV1().ClusterRoles().Get(context.TODO(), r.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("expected clusterrole to be already created, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.clusterRole); err != nil {
				t.Errorf("failed to prep creating/updating clusterrole: %v", err)
			}
			err := CreateOrUpdateClusterRole(test.client, test.clusterRole)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.clusterRole); err != nil {
				t.Errorf("failed to verify creating/updating clusterrole, got err: %v", err)
			}
		})
	}
}

func TestDeleteDeploymentIfHasLabels(t *testing.T) {
	name, namespace := "test-deployment", "test"
	labelValues := labels.Set{"dev": "test-app"}
	tests := []struct {
		name       string
		client     clientset.Interface
		deployment *appsv1.Deployment
		prep       func(clientset.Interface, *appsv1.Deployment) error
		verify     func(clientset.Interface, *appsv1.Deployment) error
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "DeleteDeploymentIfHasLabels_WithNetworkIssue_FailedToGetDeployment",
			client:     fakeclientset.NewSimpleClientset(),
			deployment: helper.NewDeployment(namespace, name),
			prep: func(c clientset.Interface, _ *appsv1.Deployment) error {
				return simulateNetworkErrorOnOp(c, "get", "deployments")
			},
			verify: func(clientset.Interface, *appsv1.Deployment) error {
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while get the deployments",
		},
		{
			name:       "DeleteDeploymentIfHasLabels_NotFound_FailedToDeleteDeployment",
			client:     fakeclientset.NewSimpleClientset(),
			deployment: helper.NewDeployment(namespace, name),
			prep: func(clientset.Interface, *appsv1.Deployment) error {
				return nil
			},
			verify: func(clientset.Interface, *appsv1.Deployment) error {
				return nil
			},
			wantErr: false,
		},
		{
			name:       "DeleteDeploymentIfHasLabels_NoLabelsToMatch_NoLabelsMatched",
			client:     fakeclientset.NewSimpleClientset(),
			deployment: helper.NewDeployment(namespace, name),
			prep: func(c clientset.Interface, d *appsv1.Deployment) error {
				if err := CreateOrUpdateDeployment(c, d); err != nil {
					return fmt.Errorf("failed to create deployment, got err: %v", err)
				}
				return nil
			},
			verify: func(c clientset.Interface, d *appsv1.Deployment) error {
				_, err := c.AppsV1().Deployments(d.GetNamespace()).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labelValues.String(),
				})
				if err != nil {
					return fmt.Errorf("expected deployment %s name in %s namespace not exist with label %s", d.GetName(), d.GetNamespace(), labelValues)
				}
				return nil
			},
			wantErr: false,
		},
		{
			name:       "DeleteDeploymentIfHasLabels_LabelsMatched_DeploymentDeleted",
			client:     fakeclientset.NewSimpleClientset(),
			deployment: helper.NewDeployment(namespace, name),
			prep: func(c clientset.Interface, d *appsv1.Deployment) error {
				d.Labels = labelValues
				if err := CreateOrUpdateDeployment(c, d); err != nil {
					return fmt.Errorf("failed to create deployment, got err: %v", err)
				}
				return nil
			},
			verify: func(c clientset.Interface, d *appsv1.Deployment) error {
				_, err := c.AppsV1().Deployments(d.GetNamespace()).Get(context.TODO(), d.GetName(), metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("expected deployment to be deleted, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.deployment); err != nil {
				t.Errorf("failed to prep deleting deployment: %v", err)
			}
			err := DeleteDeploymentIfHasLabels(test.client, test.deployment.GetName(), test.deployment.GetNamespace(), test.deployment.GetLabels())
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.deployment); err != nil {
				t.Errorf("failed to verify deleting the deployment, got err: %v", err)
			}
		})
	}
}

func TestDeleteStatefulSetIfHasLabels(t *testing.T) {
	name, namespace := "test-statefulset", "test"
	labels := labels.Set{"dev": "test-app"}
	tests := []struct {
		name        string
		client      clientset.Interface
		statefulSet *appsv1.StatefulSet
		prep        func(clientset.Interface, *appsv1.StatefulSet) error
		verify      func(clientset.Interface, *appsv1.StatefulSet) error
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "DeleteStatefulSetIfHasLabels_WithNetworkIssue_FailedToGetStatefulSet",
			client:      fakeclientset.NewSimpleClientset(),
			statefulSet: helper.NewStatefulSet(namespace, name),
			prep: func(c clientset.Interface, _ *appsv1.StatefulSet) error {
				return simulateNetworkErrorOnOp(c, "get", "statefulsets")
			},
			verify: func(clientset.Interface, *appsv1.StatefulSet) error {
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while get the statefulsets",
		},
		{
			name:        "DeleteStatefulSetIfHasLabels_NotFound_FailedToDeleteStatefulSet",
			client:      fakeclientset.NewSimpleClientset(),
			statefulSet: helper.NewStatefulSet(namespace, name),
			prep: func(clientset.Interface, *appsv1.StatefulSet) error {
				return nil
			},
			verify: func(clientset.Interface, *appsv1.StatefulSet) error {
				return nil
			},
			wantErr: false,
		},
		{
			name:        "DeleteStatefulSetIfHasLabels_NoLabelsToMatch_NoLabelsMatched",
			client:      fakeclientset.NewSimpleClientset(),
			statefulSet: helper.NewStatefulSet(namespace, name),
			prep: func(c clientset.Interface, s *appsv1.StatefulSet) error {
				if err := CreateOrUpdateStatefulSet(c, s); err != nil {
					return fmt.Errorf("failed to create statefulset, got err: %v", err)
				}
				return nil
			},
			verify: func(c clientset.Interface, s *appsv1.StatefulSet) error {
				_, err := c.AppsV1().StatefulSets(s.GetNamespace()).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labels.String(),
				})
				if err != nil {
					return fmt.Errorf("expected statefulset %s name in %s namespace not exist with label %s", s.GetName(), s.GetNamespace(), labels.String())
				}
				return nil
			},
			wantErr: false,
		},
		{
			name:        "DeleteStatefulSetIfHasLabels_LabelsMatched_StatefulSetDeleted",
			client:      fakeclientset.NewSimpleClientset(),
			statefulSet: helper.NewStatefulSet(namespace, name),
			prep: func(c clientset.Interface, s *appsv1.StatefulSet) error {
				s.Labels = labels
				if err := CreateOrUpdateStatefulSet(c, s); err != nil {
					return fmt.Errorf("failed to create statefulset, got err: %v", err)
				}
				return nil
			},
			verify: func(c clientset.Interface, s *appsv1.StatefulSet) error {
				_, err := c.AppsV1().StatefulSets(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("expected statefulset to be deleted, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.statefulSet); err != nil {
				t.Errorf("failed to prep deleting statefulset: %v", err)
			}
			err := DeleteStatefulSetIfHasLabels(test.client, test.statefulSet.GetName(), test.statefulSet.GetNamespace(), test.statefulSet.GetLabels())
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.statefulSet); err != nil {
				t.Errorf("failed to verify deleting statefulset, got err: %v", err)
			}
		})
	}
}

func TestDeleteSecretIfHasLabels(t *testing.T) {
	name, namespace := "test-secret", "test"
	labelValues := labels.Set{"dev": "test-app"}
	tests := []struct {
		name    string
		client  clientset.Interface
		secret  *corev1.Secret
		prep    func(clientset.Interface, *corev1.Secret) error
		verify  func(clientset.Interface, *corev1.Secret) error
		wantErr bool
		errMsg  string
	}{
		{
			name:   "DeleteSecretIfHasLabels_WithNetworkIssue_FailedToGetSecret",
			client: fakeclientset.NewSimpleClientset(),
			secret: helper.NewSecret(namespace, name, map[string][]byte{}),
			prep: func(c clientset.Interface, _ *corev1.Secret) error {
				return simulateNetworkErrorOnOp(c, "get", "secrets")
			},
			verify: func(clientset.Interface, *corev1.Secret) error {
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while get the secrets",
		},
		{
			name:   "DeleteSecretIfHasLabels_NotFound_FailedToDeleteSecret",
			client: fakeclientset.NewSimpleClientset(),
			secret: helper.NewSecret(namespace, name, map[string][]byte{}),
			prep: func(clientset.Interface, *corev1.Secret) error {
				return nil
			},
			verify: func(clientset.Interface, *corev1.Secret) error {
				return nil
			},
			wantErr: false,
		},
		{
			name:   "DeleteSecretIfHasLabels_NoLabelsToMatch_NoLabelsMatched",
			client: fakeclientset.NewSimpleClientset(),
			secret: helper.NewSecret(namespace, name, map[string][]byte{}),
			prep: func(c clientset.Interface, s *corev1.Secret) error {
				if err := CreateOrUpdateSecret(c, s); err != nil {
					return fmt.Errorf("failed to create secret, got err: %v", err)
				}
				return nil
			},
			verify: func(c clientset.Interface, s *corev1.Secret) error {
				_, err := c.CoreV1().Secrets(s.GetNamespace()).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labelValues.String(),
				})
				if err != nil {
					return fmt.Errorf("expected secret %s name in %s namespace not exist with label %s", s.GetName(), s.GetNamespace(), labelValues.String())
				}
				return nil
			},
			wantErr: false,
		},
		{
			name:   "DeleteStatefulSetIfHasLabels_LabelsMatched_StatefulSetDeleted",
			client: fakeclientset.NewSimpleClientset(),
			secret: helper.NewSecret(namespace, name, map[string][]byte{}),
			prep: func(c clientset.Interface, s *corev1.Secret) error {
				s.Labels = labelValues
				if err := CreateOrUpdateSecret(c, s); err != nil {
					return fmt.Errorf("failed to create statefulset, got err: %v", err)
				}
				return nil
			},
			verify: func(c clientset.Interface, s *corev1.Secret) error {
				_, err := c.AppsV1().StatefulSets(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("expected secret to be deleted, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.secret); err != nil {
				t.Errorf("failed to prep deleting secret: %v", err)
			}
			err := DeleteSecretIfHasLabels(test.client, test.secret.GetName(), test.secret.GetNamespace(), test.secret.GetLabels())
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.secret); err != nil {
				t.Errorf("failed to verify deleting secret, got err: %v", err)
			}
		})
	}
}

func TestDeleteServiceIfHasLabels(t *testing.T) {
	name, namespace := "test-service", "test"
	labelValues := labels.Set{"dev": "test-app"}
	serviceType := corev1.ServiceTypeClusterIP
	tests := []struct {
		name    string
		client  clientset.Interface
		service *corev1.Service
		prep    func(clientset.Interface, *corev1.Service) error
		verify  func(clientset.Interface, *corev1.Service) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "DeleteServiceIfHasLabels_WithNetworkIssue_FailedToGetService",
			client:  fakeclientset.NewSimpleClientset(),
			service: helper.NewService(namespace, name, serviceType),
			prep: func(c clientset.Interface, _ *corev1.Service) error {
				return simulateNetworkErrorOnOp(c, "get", "services")
			},
			verify: func(clientset.Interface, *corev1.Service) error {
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while get the services",
		},
		{
			name:    "DeleteServiceIfHasLabelsNotFound_FailedToDeleteService",
			client:  fakeclientset.NewSimpleClientset(),
			service: helper.NewService(namespace, name, serviceType),
			prep: func(clientset.Interface, *corev1.Service) error {
				return nil
			},
			verify: func(clientset.Interface, *corev1.Service) error {
				return nil
			},
			wantErr: false,
		},
		{
			name:    "DeleteServiceIfHasLabels_NoLabelsToMatch_NoLabelsMatched",
			client:  fakeclientset.NewSimpleClientset(),
			service: helper.NewService(namespace, name, serviceType),
			prep: func(c clientset.Interface, s *corev1.Service) error {
				if err := CreateOrUpdateService(c, s); err != nil {
					return fmt.Errorf("failed to create service, got err: %v", err)
				}
				return nil
			},
			verify: func(c clientset.Interface, s *corev1.Service) error {
				_, err := c.CoreV1().Services(s.GetNamespace()).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labelValues.String(),
				})
				if err != nil {
					return fmt.Errorf("expected service %s name in %s namespace not exist with label %s", s.GetName(), s.GetNamespace(), labelValues.String())
				}
				return nil
			},
			wantErr: false,
		},
		{
			name:    "DeleteServiceIfHasLabels_LabelsMatched_ServiceDeleted",
			client:  fakeclientset.NewSimpleClientset(),
			service: helper.NewService(namespace, name, serviceType),
			prep: func(c clientset.Interface, s *corev1.Service) error {
				s.Labels = labelValues
				if err := CreateOrUpdateService(c, s); err != nil {
					return fmt.Errorf("failed to create service, got err: %v", err)
				}
				return nil
			},
			verify: func(c clientset.Interface, s *corev1.Service) error {
				_, err := c.CoreV1().Services(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
				if err == nil {
					return fmt.Errorf("expected service to be deleted, but got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.service); err != nil {
				t.Errorf("failed to prep deleting service, got err: %v", err)
			}
			err := DeleteServiceIfHasLabels(test.client, test.service.GetName(), test.service.GetNamespace(), test.service.GetLabels())
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.service); err != nil {
				t.Errorf("failed to verify deleting service, got err: %v", err)
			}
		})
	}
}

func TestGetService(t *testing.T) {
	name, namespace := "test-service", "test"
	serviceType := corev1.ServiceTypeClusterIP
	tests := []struct {
		name    string
		client  clientset.Interface
		service *corev1.Service
		prep    func(clientset.Interface, *corev1.Service) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "GetService_NotFound_FailedToGetService",
			client:  fakeclientset.NewSimpleClientset(),
			service: helper.NewService(namespace, name, serviceType),
			prep: func(c clientset.Interface, _ *corev1.Service) error {
				return simulateNetworkErrorOnOp(c, "get", "services")
			},
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while get the services",
		},
		{
			name:    "GetService_Exist_ServiceRetrieved",
			client:  fakeclientset.NewSimpleClientset(),
			service: helper.NewService(namespace, name, serviceType),
			prep: func(c clientset.Interface, s *corev1.Service) error {
				if err := CreateOrUpdateService(c, s); err != nil {
					return fmt.Errorf("failed to create service, got err: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.service); err != nil {
				t.Errorf("failed to prep getting service, got err: %v", err)
			}
			_, err := GetService(test.client, test.service.GetName(), test.service.GetNamespace())
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}

// simulateNetworkErrorOnOp simulates a network error during the specified operation on a resource
// by prepending a reactor to the fake client.
func simulateNetworkErrorOnOp(c clientset.Interface, operation, resource string) error {
	c.(*fakeclientset.Clientset).Fake.PrependReactor(operation, resource, func(coretesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("unexpected error: encountered a network issue while %s the %s", operation, resource)
	})
	return nil
}

// simulateNetworkErrorCRDClientOnOp simulates a network error during the specified operation on a CRD resource
// by prepending a reactor to the fake CRD client.
func simulateNetworkErrorCRDClientOnOp(c crdsclient.Interface, operation, resource string) error {
	c.(*crdsfake.Clientset).Fake.PrependReactor(operation, resource, func(coretesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("unexpected error: encountered a network issue while %s the %s", operation, resource)
	})
	return nil
}

// simulateNetworkErrorAggregatorClientOnOp simulates a network error during the specified operation on a resource
// using the aggregator client by prepending a reactor.
func simulateNetworkErrorAggregatorClientOnOp(c aggregator.Interface, operation, resource string) error {
	c.(*fakeAggregator.Clientset).Fake.PrependReactor(operation, resource, func(coretesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("unexpected error: encountered a network issue while %s the %s", operation, resource)
	})
	return nil
}

// verifyClusterRoleUpdated checks if the ClusterRole was updated successfully by comparing resource versions.
func verifyClusterRoleUpdated(c clientset.Interface, r *rbacv1.ClusterRole) error {
	clusterRoleUpdated, err := c.RbacV1().ClusterRoles().Get(context.TODO(), r.GetName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected clusterrole to be already created, but got err: %v", err)
	}
	if r.GetResourceVersion() == "" || clusterRoleUpdated.GetResourceVersion() != r.GetResourceVersion() {
		return fmt.Errorf("mutating clusterrole resource version was not reflected")
	}
	return nil
}

// verifyServiceUpdated checks if the Service was updated successfully by verifying resource version, ClusterIP,
// and ClusterIPs values.
func verifyServiceUpdated(c clientset.Interface, s *corev1.Service) error {
	rvExpected, clusterIPExpected := "value1", "10.12.84.156"
	service, err := c.CoreV1().Services(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected service to be already created, but got err: %v", err)
	}
	if service.ResourceVersion == "" || service.ResourceVersion != rvExpected {
		return fmt.Errorf("ResourceVersion uid was not reflected, expected %s resource version but got %s", rvExpected, service.ResourceVersion)
	}
	if service.ResourceVersion == "" || service.Spec.ClusterIP != clusterIPExpected {
		return fmt.Errorf("ClusterIP was not reflected, expected %s cluster ip but got %s", clusterIPExpected, service.Spec.ClusterIP)
	}
	if service.ResourceVersion == "" || service.Spec.ClusterIPs[0] != clusterIPExpected {
		return fmt.Errorf("ClusterIPs was not reflected, expected %s cluster ip to be in cluster ips %v", clusterIPExpected, service.Spec.ClusterIPs)
	}
	return nil
}

// verifyStatefulSetUpdated checks if the StatefulSet was updated successfully by comparing resource versions.
func verifyStatefulSetUpdated(c clientset.Interface, s *appsv1.StatefulSet) error {
	statefulSetUpdated, err := c.AppsV1().StatefulSets(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected statefulset to be already created, but got err: %v", err)
	}
	if s.GetResourceVersion() == "" || statefulSetUpdated.GetResourceVersion() != s.GetResourceVersion() {
		return fmt.Errorf("mutating statefulset resource version was not reflected, expected %s resource version but got %s", s.ResourceVersion, statefulSetUpdated.ResourceVersion)
	}
	return nil
}

// verifyAPIServiceUpdated checks if the APIService was updated successfully by comparing resource versions.
func verifyAPIServiceUpdated(a aggregator.Interface, s *apiregistrationv1.APIService) error {
	apiServiceUpdated, err := a.ApiregistrationV1().APIServices().Get(context.TODO(), s.GetName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected api service to be already created, but got err: %v", err)
	}
	if s.ResourceVersion == "" || s.ResourceVersion != apiServiceUpdated.ResourceVersion {
		return fmt.Errorf("api service resource version was not reflected, expected %s resource version but got %s", s.ResourceVersion, apiServiceUpdated.ResourceVersion)
	}
	return nil
}

// verifyValidatingWebhookConfigurationUpdated checks if the ValidatingWebhookConfiguration was updated
// by comparing resource versions.
func verifyValidatingWebhookConfigurationUpdated(c clientset.Interface, vwc *admissionregistrationv1.ValidatingWebhookConfiguration) error {
	vwcUpdated, err := c.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.TODO(), vwc.GetName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected validating webhook configuration to be already created, but got err: %v", err)
	}
	if vwc.ResourceVersion == "" || vwc.ResourceVersion != vwcUpdated.ResourceVersion {
		return fmt.Errorf("validating webhook configuration resource version was not reflected, expected %s resource version but got %s", vwc.ResourceVersion, vwcUpdated.ResourceVersion)
	}
	return nil
}

// verifyMutatingWebhookConfigurationUpdated checks if the MutatingWebhookConfiguration was updated
// by comparing resource versions.
func verifyMutatingWebhookConfigurationUpdated(c clientset.Interface, mwc *admissionregistrationv1.MutatingWebhookConfiguration) error {
	mwcUpdated, err := c.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.TODO(), mwc.GetName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected mutating webhook configuration to be already created, but got err: %v", err)
	}
	if mwc.ResourceVersion == "" || mwc.ResourceVersion != mwcUpdated.ResourceVersion {
		return fmt.Errorf("mutating webhook configuration resource version was not reflected, expected %s resource version but got %s", mwc.ResourceVersion, mwcUpdated.ResourceVersion)
	}
	return nil
}

// verifyDeploymentUpdated checks if the Deployment was updated successfully by verifying the UID.
func verifyDeploymentUpdated(c clientset.Interface, d *appsv1.Deployment) error {
	deploymentUpdated, err := c.AppsV1().Deployments(d.GetNamespace()).Get(context.TODO(), d.GetName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected deployment to be already created, but got err: %v", err)
	}
	if d.UID != types.UID(deploymentUpdated.UID) {
		return fmt.Errorf("deployment uid was not reflected, expected %s uid but got %s", d.UID, deploymentUpdated.UID)
	}
	return nil
}

// verifySecretUpdated checks if the Secret was updated successfully by verifying the UID.
func verifySecretUpdated(c clientset.Interface, s *corev1.Secret) error {
	serviceUpdated, err := c.CoreV1().Secrets(s.GetNamespace()).Get(context.TODO(), s.GetName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected secret to be already created, but got err: %v", err)
	}
	if s.UID != types.UID(serviceUpdated.UID) {
		return fmt.Errorf("secret uid was not reflected, expected %s uid but got %s", serviceUpdated.UID, s.UID)
	}
	return nil
}
