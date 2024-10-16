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

package tasks

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	crdsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	coretesting "k8s.io/client-go/testing"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	fakeAggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"

	"github.com/karmada-io/karmada/operator/pkg/certs"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

func TestRunKarmadaResources(t *testing.T) {
	tests := []struct {
		name    string
		runData workflow.RunData
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunKarmadaResources_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			wantErr: true,
			errMsg:  "karmadaResources task invoked with an invalid data struct",
		},
		{
			name: "RunKarmadaResources_ValidTypeAssertion_TypeAssertionIsValid",
			runData: &TestInitData{
				Name:      "karmada-demo",
				Namespace: "test",
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runKarmadaResources(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
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

func TestRunSystemNamespaces(t *testing.T) {
	name, namespace := "karmada-demo", constants.KarmadaSystemNamespace
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		verify  func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunSystemNamespace_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "systemName task invoked with an invalid data struct",
		},
		{
			name: "RunSystemNamespace_WithNetworkIssue_FailedToCreateNamespace",
			runData: &TestInitData{
				Name:                   name,
				Namespace:              namespace,
				KarmadaClientConnector: fakeclientset.NewSimpleClientset(),
			},
			prep: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				data.KarmadaClient().(*fakeclientset.Clientset).Fake.PrependReactor("get", "namespaces", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while getting the namespaces")
				})
				return nil
			},
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while getting the namespaces",
		},
		{
			name: "RunSystemNamespace_NamespaceNotFound_NamespaceCreated",
			runData: &TestInitData{
				Name:                   name,
				Namespace:              namespace,
				KarmadaClientConnector: fakeclientset.NewSimpleClientset(),
			},
			prep: func(workflow.RunData) error { return nil },
			verify: func(rd workflow.RunData) error {
				data := rd.(*TestInitData)
				_, err := data.KarmadaClient().CoreV1().Namespaces().Get(context.TODO(), data.GetNamespace(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to get namespace, got: %v", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep before running system namespace init task, got: %v", err)
			}
			err := runSystemNamespace(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.runData); err != nil {
				t.Errorf("failed to verify the running success status of system namespace init task, got: %v", err)
			}
		})
	}
}

func TestRunCrds(t *testing.T) {
	tests := []struct {
		name    string
		client  crdsclient.Interface
		runData workflow.RunData
		prep    func(workflow.RunData) error
		verify  func(crdsclient.Interface) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunCrds_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			verify:  func(crdsclient.Interface) error { return nil },
			wantErr: true,
			errMsg:  "crds task invoked with an invalid data struct",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep before running crds init task, got: %v", err)
			}
			err := runCrds(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client); err != nil {
				t.Errorf("failed to verify the running crds init task, got: %v", err)
			}
		})
	}
}

func TestRunWebhookConfiguration(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name    string
		runData workflow.RunData
		prep    func(workflow.RunData) error
		verify  func(workflow.RunData) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "RunWebhookConfiguration_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData) error { return nil },
			verify:  func(workflow.RunData) error { return nil },
			wantErr: true,
			errMsg:  "[webhookConfiguration] task invoked with an invalid data struct",
		},
		{
			name: "RunWebhookConfiguration_WithCA_WebhookConfigurationsEnsured",
			runData: &TestInitData{
				Name:                   name,
				Namespace:              namespace,
				KarmadaClientConnector: fakeclientset.NewSimpleClientset(),
			},
			prep: func(rd workflow.RunData) error {
				caCert, err := certs.NewCertificateAuthority(certs.KarmadaCertRootCA())
				if err != nil {
					return fmt.Errorf("failed to create karmada root certificate authority, got: %v", err)
				}
				rd.(*TestInitData).AddCert(caCert)
				return nil
			},
			verify:  verifyWebhookConfiguration,
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData); err != nil {
				t.Errorf("failed to prep before running webhook configurations init task, got: %v", err)
			}
			err := runWebhookConfiguration(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.runData); err != nil {
				t.Errorf("failed to verify running webhook configurations init task, got: %v", err)
			}
		})
	}
}

func TestRunAPIService(t *testing.T) {
	name, namespace := "karmada-demo", "test"
	tests := []struct {
		name       string
		runData    workflow.RunData
		apiService *apiregistrationv1.APIService
		prep       func(workflow.RunData, *apiregistrationv1.APIService) error
		wantErr    bool
		errMsg     string
	}{
		{
			name:    "RunAPIService_InvalidTypeAssertion_TypeAssertionIsInvalid",
			runData: &MyTestData{Data: "test"},
			prep:    func(workflow.RunData, *apiregistrationv1.APIService) error { return nil },
			wantErr: true,
			errMsg:  "[apiService] task invoked with an invalid data struct",
		},
		{
			name: "RunAPIService_WithCAAndReadyAPIService_APIServiceIsUpAndRunning",
			runData: &TestInitData{
				Name:                   name,
				Namespace:              namespace,
				KarmadaClientConnector: fakeclientset.NewSimpleClientset(),
			},
			apiService: &apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: constants.APIServiceName,
				},
				Spec: apiregistrationv1.APIServiceSpec{
					Service: &apiregistrationv1.ServiceReference{},
					Version: "v1beta1",
				},
			},
			prep:    prepAPIServiceInitTask,
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.runData, test.apiService); err != nil {
				t.Errorf("failed to prep before running api service init task, got: %v", err)
			}
			err := runAPIService(test.runData)
			if err == nil && test.wantErr {
				t.Error("expected an error, but got none")
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

// verifyWebhookConfiguration verifies that the correct webhook configuration actions were taken
// during the execution of a test. It expects exactly two actions: one for creating a mutating
// webhook configuration and one for creating a validating webhook configuration.
func verifyWebhookConfiguration(rd workflow.RunData) error {
	data := rd.(*TestInitData)
	client := data.KarmadaClient()
	actions := client.(*fakeclientset.Clientset).Actions()
	if len(actions) != 2 {
		return fmt.Errorf("expected %d actions, but got %d", 2, len(actions))
	}
	// Verify that both actions are creation actions for the webhook configurations.
	for _, action := range actions {
		_, ok := action.(coretesting.CreateAction)
		if !ok {
			return fmt.Errorf("expected action %T to be of type coretesting.CreateAction", action)
		}
	}
	// Verify that actions are of type validating/mutating webhook configurations.
	var foundM, foundV bool
	for _, action := range actions {
		createAction := action.(coretesting.CreateAction)
		got := createAction.GetObject()
		mType, vType := &admissionregistrationv1.MutatingWebhookConfiguration{}, &admissionregistrationv1.ValidatingWebhookConfiguration{}
		if reflect.TypeOf(got) == reflect.TypeOf(mType) {
			foundM = true
		}
		if reflect.TypeOf(got) == reflect.TypeOf(vType) {
			foundV = true
		}
	}
	if !foundM || !foundV {
		return fmt.Errorf("expected validating and mutating webhook configuration creation actions to be both existed in actions %v", actions)
	}
	return nil
}

// prepAPIServiceInitTask prepares and initializes an APIService resource during a test.
// It sets up a certificate authority, mocks an aggregator client, and ensures the APIService
// is created with an available status. Additionally, it mocks necessary functions for further
// test interactions.
func prepAPIServiceInitTask(rd workflow.RunData, apiService *apiregistrationv1.APIService) error {
	data := rd.(*TestInitData)
	// Create certificate authority.
	caCert, err := certs.NewCertificateAuthority(certs.KarmadaCertRootCA())
	if err != nil {
		return fmt.Errorf("failed to create karmada root certificate authority, got: %v", err)
	}
	data.AddCert(caCert)

	// Mock aggregator client factory global function.
	aggClient := fakeAggregator.NewSimpleClientset()

	// Create API Service with available status.
	apiServiceCreated, err := aggClient.ApiregistrationV1().APIServices().Create(context.TODO(), apiService, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("faield to create api service %s, got err: %v", apiService.Name, err)
	}

	// Mock functions.
	aggregatorClientFactory = func(*rest.Config) (aggregator.Interface, error) {
		return aggClient, nil
	}
	apiclient.AggregateClientFromConfigBuilder = func(*rest.Config) (aggregator.Interface, error) {
		apiServiceCreated.Status = apiregistrationv1.APIServiceStatus{
			Conditions: []apiregistrationv1.APIServiceCondition{
				{
					Type:   apiregistrationv1.Available,
					Status: apiregistrationv1.ConditionTrue,
				},
			},
		}
		if _, err = aggClient.ApiregistrationV1().APIServices().UpdateStatus(context.TODO(), apiServiceCreated, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("failed to update api service with available status, got err: %v", err)
		}
		return aggClient, nil
	}

	componentBeReadyTimeout = time.Second

	return nil
}
