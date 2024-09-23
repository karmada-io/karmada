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

package apiservice

import (
	"encoding/base64"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	fakeAggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
)

func TestEnsureAggregatedAPIService(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	// Base64 encoding of the dummy certificate data.
	caTestData := "test-ca-data"
	caBundle := base64.StdEncoding.EncodeToString([]byte(caTestData))

	fakeAggregatorClient := fakeAggregator.NewSimpleClientset()
	fakeClient := fakeclientset.NewSimpleClientset()
	err := EnsureAggregatedAPIService(
		fakeAggregatorClient, fakeClient, name, namespace, name, namespace, caBundle,
	)
	if err != nil {
		t.Fatalf("failed to ensure aggregated api service: %v", err)
	}

	// Ensure the expected action (aggregated api service creation) occurred on the fake aggregator clientset.
	actions := fakeAggregatorClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Ensure the expected action (aggregated apiserver service creation) occurred on the fake clientset.
	actions = fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}
}

func TestAggregatedAPIService(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	// Base64 encoding of the dummy certificate data.
	caTestData := "test-ca-data"
	caBundle := base64.StdEncoding.EncodeToString([]byte(caTestData))

	fakeClient := fakeAggregator.NewSimpleClientset()
	err := aggregatedAPIService(fakeClient, name, namespace, caBundle)
	if err != nil {
		t.Fatalf("failed to create aggregated api service: %v", err)
	}

	// Ensure the expected action (service creation) occurred.
	actions := fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (apiregistrationv1.APIService).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	if createAction.GetResource().Resource != "apiservices" {
		t.Fatalf("expected action on 'apiservices', but got '%s'", createAction.GetResource().Resource)
	}

	// Validate the created apiregistrationv1.APIService object.
	service := createAction.GetObject().(*apiregistrationv1.APIService)
	expectedServiceName := util.KarmadaAggregatedAPIServerName(name)
	if service.Spec.Service.Name != expectedServiceName {
		t.Fatalf("expected service name '%s', but got '%s'", expectedServiceName, service.Name)
	}

	if service.Spec.Service.Namespace != namespace {
		t.Fatalf("expected service namespace '%s', but got '%s'", namespace, service.Namespace)
	}

	if string(service.Spec.CABundle) != caTestData {
		t.Fatalf("expected service caBundle %s, but got %s", caTestData, string(service.Spec.CABundle))
	}
}

func TestAggregatedAPIServerService(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	fakeClient := fakeclientset.NewSimpleClientset()
	err := aggregatedApiserverService(fakeClient, name, namespace, name, namespace)
	if err != nil {
		t.Fatalf("failed to create aggregated apiserver service: %v", err)
	}

	// Ensure the expected action (service creation) occurred.
	actions := fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (Service).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	if createAction.GetResource().Resource != "services" {
		t.Fatalf("expected action on 'services', but got '%s'", createAction.GetResource().Resource)
	}

	// Validate the created service object.
	service := createAction.GetObject().(*corev1.Service)
	expectedServiceName := util.KarmadaAggregatedAPIServerName(name)
	if service.Name != expectedServiceName {
		t.Fatalf("expected service name '%s', but got '%s'", expectedServiceName, service.Name)
	}

	if service.Namespace != namespace {
		t.Fatalf("expected service namespace '%s', but got '%s'", namespace, service.Namespace)
	}

	serviceExternalNameExpected := fmt.Sprintf("%s.%s.svc", expectedServiceName, namespace)
	if service.Spec.ExternalName != serviceExternalNameExpected {
		t.Fatalf("expected service external name '%s', but got '%s'", serviceExternalNameExpected, service.Spec.ExternalName)
	}
}

func TestEnsureMetricsAdapterAPIService(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	// Base64 encoding of the dummy certificate data.
	caTestData := "test-ca-data"
	caBundle := base64.StdEncoding.EncodeToString([]byte(caTestData))

	fakeAggregatorClient := fakeAggregator.NewSimpleClientset()
	fakeClient := fakeclientset.NewSimpleClientset()
	err := EnsureMetricsAdapterAPIService(
		fakeAggregatorClient, fakeClient, name, namespace, name, namespace, caBundle,
	)
	if err != nil {
		t.Fatalf("failed to ensure metrics adapter api service: %v", err)
	}

	// Ensure the expected action (metrics adapter api service creation) occurred on the fake aggregator clientset.
	actions := fakeAggregatorClient.Actions()
	if len(actions) != len(constants.KarmadaMetricsAdapterAPIServices) {
		t.Fatalf("expected %d action, but got %d", len(constants.KarmadaMetricsAdapterAPIServices), len(actions))
	}

	// Ensure the expected action (metrics adapter service creation) occurred on the fake clientset.
	actions = fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}
}

func TestKarmadaMetricsAdapterAPIService(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	// Base64 encoding of the dummy certificate data.
	caTestData := "test-ca-data"
	caBundle := base64.StdEncoding.EncodeToString([]byte(caTestData))

	fakeAggregatorClient := fakeAggregator.NewSimpleClientset()
	err := karmadaMetricsAdapterAPIService(fakeAggregatorClient, name, namespace, caBundle)
	if err != nil {
		t.Fatalf("failed to create karmada metrics adapter api service: %v", err)
	}

	// Ensure the expected actions (service creation) occurred.
	actions := fakeAggregatorClient.Actions()
	expectedActionsCount := len(constants.KarmadaMetricsAdapterAPIServices)
	if len(actions) != expectedActionsCount {
		t.Fatalf("expected %d actions, but got %d", expectedActionsCount, len(actions))
	}

	for i, action := range actions {
		// Validate the action is a CreateAction and it's for the correct resource (Service).
		createAction, ok := action.(coretesting.CreateAction)
		if !ok {
			t.Fatalf("expected CreateAction, but got %T", action)
		}

		if createAction.GetResource().Resource != "apiservices" {
			t.Fatalf("expected action on 'apiservices', but got '%s'", createAction.GetResource().Resource)
		}

		// Validate the created apiregistrationv1.APIService object.
		service := createAction.GetObject().(*apiregistrationv1.APIService)
		karmadaMetricsAdapterGV := constants.KarmadaMetricsAdapterAPIServices[i]
		apiServiceNameExpected := fmt.Sprintf("%s.%s", karmadaMetricsAdapterGV.Version, karmadaMetricsAdapterGV.Group)
		if service.Name != apiServiceNameExpected {
			t.Fatalf("expected APIService name %s, but got %s", apiServiceNameExpected, service.ObjectMeta.Name)
		}

		if service.Spec.Service.Namespace != namespace {
			t.Fatalf("expected APIService namespace %s, but got %s", namespace, service.Spec.Service.Namespace)
		}

		if service.Spec.Group != karmadaMetricsAdapterGV.Group {
			t.Fatalf("expected APIService group %s, but got %s", karmadaMetricsAdapterGV.Group, service.Spec.Group)
		}

		if service.Spec.Version != karmadaMetricsAdapterGV.Version {
			t.Fatalf("expected APIService version %s, but got %s", karmadaMetricsAdapterGV.Version, service.Spec.Version)
		}

		serviceNameExpected := util.KarmadaMetricsAdapterName(name)
		if service.Spec.Service.Name != serviceNameExpected {
			t.Fatalf("expected APIService service name %s, but got %s", serviceNameExpected, service.Spec.Service.Name)
		}

		if string(service.Spec.CABundle) != caTestData {
			t.Fatalf("expected service CABundle %s, but got %s", caTestData, string(service.Spec.CABundle))
		}
	}
}

func TestKarmadaMetricsAdapterService(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	fakeClient := fakeclientset.NewSimpleClientset()
	err := karmadaMetricsAdapterService(fakeClient, name, namespace, name, namespace)
	if err != nil {
		t.Fatalf("failed to create karmada metrics adapter service: %v", err)
	}

	// Ensure the expected action (service creation) occurred.
	actions := fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (Service).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	if createAction.GetResource().Resource != "services" {
		t.Fatalf("expected action on 'services', but got '%s'", createAction.GetResource().Resource)
	}

	// Validate the created service object.
	service := createAction.GetObject().(*corev1.Service)
	expectedServiceName := util.KarmadaMetricsAdapterName(name)
	if service.Name != expectedServiceName {
		t.Fatalf("expected service name '%s', but got '%s'", expectedServiceName, service.Name)
	}

	if service.Namespace != namespace {
		t.Fatalf("expected service namespace '%s', but got '%s'", namespace, service.Namespace)
	}

	serviceExternalNameExpected := fmt.Sprintf("%s.%s.svc", expectedServiceName, namespace)
	if service.Spec.ExternalName != serviceExternalNameExpected {
		t.Fatalf("expected service external name '%s', but got '%s'", serviceExternalNameExpected, service.Spec.ExternalName)
	}
}

func TestEnsureSearchAPIService(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	// Base64 encoding of the dummy certificate data.
	caTestData := "test-ca-data"
	caBundle := base64.StdEncoding.EncodeToString([]byte(caTestData))

	fakeAggregatorClient := fakeAggregator.NewSimpleClientset()
	fakeClient := fakeclientset.NewSimpleClientset()
	err := EnsureSearchAPIService(
		fakeAggregatorClient, fakeClient, name, namespace, name, namespace, caBundle,
	)
	if err != nil {
		t.Fatalf("failed to ensure search api service service: %v", err)
	}

	// Ensure the expected action (search api service creation) occurred on the fake aggregator clientset.
	actions := fakeAggregatorClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected %d action, but got %d", 1, len(actions))
	}

	// Ensure the expected action (search service creation) occurred on the fake clientset.
	actions = fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}
}

func TestKarmadaSearchAPIService(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	// Base64 encoding of the dummy certificate data.
	caTestData := "test-ca-data"
	caBundle := base64.StdEncoding.EncodeToString([]byte(caTestData))

	fakeClient := fakeAggregator.NewSimpleClientset()
	err := karmadaSearchAPIService(fakeClient, name, namespace, caBundle)
	if err != nil {
		t.Fatalf("failed to ensure metrics adapter api service: %v", err)
	}

	// Ensure the expected action (apiservice creation) occurred.
	actions := fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (apiregistrationv1.APIService).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	if createAction.GetResource().Resource != "apiservices" {
		t.Fatalf("expected action on 'apiservices', but got '%s'", createAction.GetResource().Resource)
	}

	// Validate the created service object.
	service := createAction.GetObject().(*apiregistrationv1.APIService)
	serviceNameExpected := util.KarmadaSearchAPIServerName(name)
	if service.Spec.Service.Name != serviceNameExpected {
		t.Fatalf("expected APIService name %s, but got %s", serviceNameExpected, service.Spec.Service.Name)
	}

	if service.Spec.Service.Namespace != namespace {
		t.Fatalf("expected APIService namespace %s, but got %s", namespace, service.Spec.Service.Namespace)
	}

	if string(service.Spec.CABundle) != caTestData {
		t.Fatalf("expected service CABundle %s, but got %s", caTestData, string(service.Spec.CABundle))
	}
}

func TestKarmadaSearchService(t *testing.T) {
	name, namespace := "karmada-demo", "test"

	fakeClient := fakeclientset.NewSimpleClientset()
	err := karmadaSearchService(fakeClient, name, namespace, name, namespace)
	if err != nil {
		t.Fatalf("failed to create karmada search service: %v", err)
	}

	// Ensure the expected action (service creation) occurred.
	actions := fakeClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, but got %d", len(actions))
	}

	// Validate the action is a CreateAction and it's for the correct resource (Service).
	createAction, ok := actions[0].(coretesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, but got %T", actions[0])
	}

	if createAction.GetResource().Resource != "services" {
		t.Fatalf("expected action on 'services', but got '%s'", createAction.GetResource().Resource)
	}

	// Validate the created service object.
	service := createAction.GetObject().(*corev1.Service)
	serviceNameExpected := util.KarmadaSearchName(name)
	if service.Name != serviceNameExpected {
		t.Fatalf("expected service name '%s', but got '%s'", serviceNameExpected, service.Name)
	}

	if service.Namespace != namespace {
		t.Fatalf("expected service namespace %s, but got %s", namespace, service.Namespace)
	}

	serviceExternalNameExpected := fmt.Sprintf("%s.%s.svc", util.KarmadaSearchName(name), namespace)
	if service.Spec.ExternalName != serviceExternalNameExpected {
		t.Fatalf("expected service external name '%s', but got '%s'", serviceExternalNameExpected, service.Spec.ExternalName)
	}
}
