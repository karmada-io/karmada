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

package promote

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	coretesting "k8s.io/client-go/testing"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/generated/clientset/versioned/scheme"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestValidatePromoteOptions(t *testing.T) {
	tests := []struct {
		name        string
		promoteOpts *CommandPromoteOption
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "Validate_WithoutCluster_ClusterCanNotBeEmpty",
			promoteOpts: &CommandPromoteOption{Cluster: ""},
			wantErr:     true,
			errMsg:      "the cluster cannot be empty",
		},
		{
			name: "Validate_WithXMLOutputFormat_InvalidOutputFormat",
			promoteOpts: &CommandPromoteOption{
				Cluster:      "member1",
				OutputFormat: "xml",
			},
			wantErr: true,
			errMsg:  "invalid output format",
		},
		{
			name: "Validate_WithValidOptions_PromoteOptionsValidated",
			promoteOpts: &CommandPromoteOption{
				Cluster:      "member1",
				OutputFormat: "json",
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.promoteOpts.Validate()
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
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

func TestPromoteResourceInLegacyCluster(t *testing.T) {
	tests := []struct {
		name                      string
		promoteOpts               *CommandPromoteOption
		controlPlaneRestConfig    *rest.Config
		obj                       *unstructured.Unstructured
		controlPlaneDynamicClient dynamic.Interface
		karmadaClient             karmadaclientset.Interface
		gvr                       schema.GroupVersionResource
		prep                      func(dynamic.Interface, karmadaclientset.Interface, schema.GroupVersionResource) error
		verify                    func(controlPlaneDynamicClient dynamic.Interface, karmadaClient karmadaclientset.Interface, gvr schema.GroupVersionResource, namespace, resourceName, policyName string) error
		wantErr                   bool
		errMsg                    string
	}{
		{
			name: "Promote_PrintObjectAndPolicy_FailedToInitializeK8sPrinter",
			promoteOpts: &CommandPromoteOption{
				name:         "demo-crd",
				OutputFormat: "json",
				Printer: func(*meta.RESTMapping, *bool, bool, bool) (printers.ResourcePrinterFunc, error) {
					return nil, errors.New("unknown type in printer")
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name": "demo-crd",
					},
				},
			},
			prep: func(dynamic.Interface, karmadaclientset.Interface, schema.GroupVersionResource) error { return nil },
			verify: func(dynamic.Interface, karmadaclientset.Interface, schema.GroupVersionResource, string, string, string) error {
				return nil
			},
			wantErr: true,
			errMsg:  "unknown type in printer",
		},
		{
			name: "Promote_CreateClusterScopedResourceInControlPlane_FailedToCreateResource",
			promoteOpts: &CommandPromoteOption{
				name: "demo-crd",
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name": "demo-crd",
					},
				},
			},
			controlPlaneRestConfig:    &rest.Config{},
			controlPlaneDynamicClient: fakedynamic.NewSimpleDynamicClient(scheme.Scheme),
			gvr:                       schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			prep: func(controlPlaneDynamicClient dynamic.Interface, _ karmadaclientset.Interface, gvr schema.GroupVersionResource) error {
				controlPlaneDynamicClient.(*fakedynamic.FakeDynamicClient).Fake.PrependReactor("create", gvr.Resource, func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error; encountered network issue while creating resources")
				})
				dynamicClientBuilder = func(*rest.Config) dynamic.Interface {
					return controlPlaneDynamicClient
				}
				return nil
			},
			verify: func(dynamic.Interface, karmadaclientset.Interface, schema.GroupVersionResource, string, string, string) error {
				return nil
			},
			wantErr: true,
			errMsg:  "encountered network issue while creating resources",
		},
		{
			name: "Promote_CreateClusterScopedResourceInControlPlane_ResourceCreated",
			promoteOpts: &CommandPromoteOption{
				name:             "demo-crd",
				Cluster:          "member1",
				PolicyName:       "demo-crd-cluster-propagationpolicy",
				AutoCreatePolicy: true,
				Deps:             true,
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apiextensions.k8s.io/v1",
					"kind":       "CustomResourceDefinition",
					"metadata": map[string]interface{}{
						"name": "demo-crd",
					},
				},
			},
			controlPlaneRestConfig:    &rest.Config{},
			controlPlaneDynamicClient: fakedynamic.NewSimpleDynamicClient(scheme.Scheme),
			karmadaClient:             fakekarmadaclient.NewSimpleClientset(),
			gvr: schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			},
			prep: func(controlPlaneDynamicClient dynamic.Interface, karmadaClient karmadaclientset.Interface, _ schema.GroupVersionResource) error {
				dynamicClientBuilder = func(*rest.Config) dynamic.Interface {
					return controlPlaneDynamicClient
				}
				karmadaClientBuilder = func(*rest.Config) karmadaclientset.Interface {
					return karmadaClient
				}
				return nil
			},
			verify: func(controlPlaneDynamicClient dynamic.Interface, karmadaClient karmadaclientset.Interface, gvr schema.GroupVersionResource, _, resourceName, policyName string) error {
				if _, err := controlPlaneDynamicClient.Resource(gvr).Get(context.TODO(), resourceName, metav1.GetOptions{}); err != nil {
					return fmt.Errorf("failed to get resource %v, but got error: %v", resourceName, err)
				}
				if _, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Get(context.TODO(), policyName, metav1.GetOptions{}); err != nil {
					return fmt.Errorf("failed to get cluster propagation policy %s, got error: %v", policyName, err)
				}
				return nil
			},
			wantErr: false,
		},
		{
			name: "Promote_CreateNamespaceScopedResourceInControlPlane_ResourceCreated",
			promoteOpts: &CommandPromoteOption{
				name:             "demo-deployment",
				Cluster:          "member1",
				PolicyName:       "demo-deployment-propagationpolicy",
				Namespace:        names.NamespaceDefault,
				AutoCreatePolicy: true,
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deploymnet",
					"metadata": map[string]interface{}{
						"name":      "demo-deployment",
						"namespace": names.NamespaceDefault,
					},
				},
			},
			controlPlaneRestConfig:    &rest.Config{},
			controlPlaneDynamicClient: fakedynamic.NewSimpleDynamicClient(scheme.Scheme),
			karmadaClient:             fakekarmadaclient.NewSimpleClientset(),
			gvr: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			prep: func(controlPlaneDynamicClient dynamic.Interface, karmadaClient karmadaclientset.Interface, _ schema.GroupVersionResource) error {
				dynamicClientBuilder = func(*rest.Config) dynamic.Interface {
					return controlPlaneDynamicClient
				}
				karmadaClientBuilder = func(*rest.Config) karmadaclientset.Interface {
					return karmadaClient
				}
				return nil
			},
			verify: func(controlPlaneDynamicClient dynamic.Interface, karmadaClient karmadaclientset.Interface, gvr schema.GroupVersionResource, namespace, resourceName, policyName string) error {
				if _, err := controlPlaneDynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{}); err != nil {
					return fmt.Errorf("failed to get resource %v, but got error: %v", resourceName, err)
				}
				if _, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(namespace).Get(context.TODO(), policyName, metav1.GetOptions{}); err != nil {
					return fmt.Errorf("failed to get propagation policy %s in namespace %s, got error: %v", policyName, namespace, err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.controlPlaneDynamicClient, test.karmadaClient, test.gvr); err != nil {
				t.Fatalf("failed to prep test environment, got error: %v", err)
			}
			err := test.promoteOpts.promote(test.controlPlaneRestConfig, test.obj, test.gvr)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.controlPlaneDynamicClient, test.karmadaClient, test.gvr, test.obj.GetNamespace(), test.obj.GetName(), test.promoteOpts.PolicyName); err != nil {
				t.Errorf("failed to verify promoting the resource %s in the legacy cluster %s, got error: %v", test.gvr.Resource, test.promoteOpts.Cluster, err)
			}
		})
	}
}

func TestCreatePropagationPolicy(t *testing.T) {
	tests := []struct {
		name        string
		promoteOpts *CommandPromoteOption
		client      karmadaclientset.Interface
		gvr         schema.GroupVersionResource
		prep        func(karmadaclientset.Interface, *CommandPromoteOption) error
		verify      func(karmadaclientset.Interface, *CommandPromoteOption) error
		wantErr     bool
		errMsg      string
	}{
		{
			name: "CreatePropagationPolicy_PropagationPolicyAlreadyExists_ReturnTheExistingOne",
			promoteOpts: &CommandPromoteOption{
				Namespace:  names.NamespaceDefault,
				PolicyName: "webserver-propagation",
			},
			client: fakekarmadaclient.NewSimpleClientset(),
			prep: func(client karmadaclientset.Interface, promoteOpts *CommandPromoteOption) error {
				pp := &policyv1alpha1.PropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      promoteOpts.PolicyName,
						Namespace: promoteOpts.Namespace,
					},
				}
				if _, err := client.PolicyV1alpha1().PropagationPolicies(promoteOpts.Namespace).Create(context.TODO(), pp, metav1.CreateOptions{}); err != nil {
					return fmt.Errorf("failed to create propagation policy %s, got error: %v", promoteOpts.PolicyName, err)
				}
				return nil
			},
			verify:  func(karmadaclientset.Interface, *CommandPromoteOption) error { return nil },
			wantErr: true,
			errMsg:  fmt.Sprintf("PropagationPolicy(%s/%s) already exist", names.NamespaceDefault, "webserver-propagation"),
		},
		{
			name: "CreatePropagationPolicy_GetPropagationPolicyFromK8s_GotNetworkIssue",
			promoteOpts: &CommandPromoteOption{
				Namespace:  names.NamespaceDefault,
				PolicyName: "webserver-propagation",
			},
			client: fakekarmadaclient.NewSimpleClientset(),
			prep: func(client karmadaclientset.Interface, _ *CommandPromoteOption) error {
				client.(*fakekarmadaclient.Clientset).Fake.PrependReactor("get", "propagationpolicies", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while getting the propagationpolicies")
				})
				return nil
			},
			verify:  func(karmadaclientset.Interface, *CommandPromoteOption) error { return nil },
			wantErr: true,
			errMsg:  "encountered a network issue while getting the propagationpolicies",
		},
		{
			name: "CreatePropagationPolicy_CreatePropagationPolicy_FailedToCreatePropagationPolicy",
			promoteOpts: &CommandPromoteOption{
				Namespace:  names.NamespaceDefault,
				PolicyName: "webserver-propagation",
				Cluster:    "member1",
			},
			client: fakekarmadaclient.NewSimpleClientset(),
			prep: func(client karmadaclientset.Interface, _ *CommandPromoteOption) error {
				client.(*fakekarmadaclient.Clientset).Fake.PrependReactor("get", "propagationpolicies", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while creating the propagationpolicies")
				})
				return nil
			},
			verify:  func(karmadaclientset.Interface, *CommandPromoteOption) error { return nil },
			gvr:     schema.GroupVersionResource{},
			wantErr: true,
			errMsg:  "encountered a network issue while creating the propagationpolicies",
		},
		{
			name: "CreatePropagationPolicy_CreatePropagationPolicy_PropagationPolicyCreated",
			promoteOpts: &CommandPromoteOption{
				name:       "nginx-deployment",
				Namespace:  names.NamespaceDefault,
				PolicyName: "webserver-propagation",
				Cluster:    "member1",
				gvk: schema.GroupVersionKind{
					Kind: "Deployment",
				},
				Deps: true,
			},
			client: fakekarmadaclient.NewSimpleClientset(),
			prep:   func(karmadaclientset.Interface, *CommandPromoteOption) error { return nil },
			verify: func(client karmadaclientset.Interface, promoteOpts *CommandPromoteOption) error {
				if _, err := client.PolicyV1alpha1().PropagationPolicies(promoteOpts.Namespace).Get(context.TODO(), promoteOpts.PolicyName, metav1.GetOptions{}); err != nil {
					return fmt.Errorf("failed to create propagation policy %s in namespace %s, got error: %v", promoteOpts.PolicyName, promoteOpts.Namespace, err)
				}
				return nil
			},
			gvr: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.promoteOpts); err != nil {
				t.Fatalf("failed to prep test environment, got error: %v", err)
			}
			_, err := test.promoteOpts.createPropagationPolicy(test.client, test.gvr)
			if err == nil && test.wantErr {
				t.Fatal("expcted an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.promoteOpts); err != nil {
				t.Errorf("failed to verify creating propagation policy, got error: %v", err)
			}
		})
	}
}

func TestCreateClusterPropagationPolicy(t *testing.T) {
	tests := []struct {
		name        string
		promoteOpts *CommandPromoteOption
		client      karmadaclientset.Interface
		gvr         schema.GroupVersionResource
		prep        func(karmadaclientset.Interface, *CommandPromoteOption) error
		verify      func(karmadaclientset.Interface, *CommandPromoteOption) error
		wantErr     bool
		errMsg      string
	}{
		{
			name: "CreateClusterPropagationPolicy_ClusterPropagationPolicyAlreadyExists_ReturnTheExistingOne",
			promoteOpts: &CommandPromoteOption{
				PolicyName: "crd-cluster-propagation",
			},
			client: fakekarmadaclient.NewSimpleClientset(),
			prep: func(client karmadaclientset.Interface, promoteOpts *CommandPromoteOption) error {
				cpp := &policyv1alpha1.ClusterPropagationPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: promoteOpts.PolicyName,
					},
				}
				if _, err := client.PolicyV1alpha1().ClusterPropagationPolicies().Create(context.TODO(), cpp, metav1.CreateOptions{}); err != nil {
					return fmt.Errorf("failed to create cluster propagation policy %s, got error: %v", promoteOpts.PolicyName, err)
				}
				return nil
			},
			verify:  func(karmadaclientset.Interface, *CommandPromoteOption) error { return nil },
			wantErr: true,
			errMsg:  "ClusterPropagationPolicy(crd-cluster-propagation) already exist",
		},
		{
			name: "CreateClusterPropagationPolicy_GetClusterPropagationPolicyFromK8s_GotNetworkIssue",
			promoteOpts: &CommandPromoteOption{
				PolicyName: "crd-cluster-propagation",
			},
			client: fakekarmadaclient.NewSimpleClientset(),
			prep: func(client karmadaclientset.Interface, _ *CommandPromoteOption) error {
				client.(*fakekarmadaclient.Clientset).Fake.PrependReactor("get", "clusterpropagationpolicies", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while getting the cluster propagationpolicies")
				})
				return nil
			},
			verify:  func(karmadaclientset.Interface, *CommandPromoteOption) error { return nil },
			wantErr: true,
			errMsg:  "encountered a network issue while getting the cluster propagationpolicies",
		},
		{
			name: "CreateClusterPropagationPolicy_CreateClusterPropagationPolicy_FailedToCreateClusterPropagationPolicy",
			promoteOpts: &CommandPromoteOption{
				PolicyName: "crd-cluster-propagation",
				Cluster:    "member1",
			},
			client: fakekarmadaclient.NewSimpleClientset(),
			prep: func(client karmadaclientset.Interface, _ *CommandPromoteOption) error {
				client.(*fakekarmadaclient.Clientset).Fake.PrependReactor("get", "clusterpropagationpolicies", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while creating the cluster propagationpolicies")
				})
				return nil
			},
			verify:  func(karmadaclientset.Interface, *CommandPromoteOption) error { return nil },
			gvr:     schema.GroupVersionResource{},
			wantErr: true,
			errMsg:  "encountered a network issue while creating the cluster propagationpolicies",
		},
		{
			name: "CreateClusterPropagationPolicy_CreateClusterPropagationPolicy_ClusterPropagationPolicyCreated",
			promoteOpts: &CommandPromoteOption{
				name:       "crd",
				PolicyName: "crd-cluster-propagation",
				Cluster:    "member1",
				gvk: schema.GroupVersionKind{
					Kind: "CustomResourceDefinition",
				},
				Deps: true,
			},
			client: fakekarmadaclient.NewSimpleClientset(),
			prep:   func(karmadaclientset.Interface, *CommandPromoteOption) error { return nil },
			verify: func(client karmadaclientset.Interface, promoteOpts *CommandPromoteOption) error {
				if _, err := client.PolicyV1alpha1().ClusterPropagationPolicies().Get(context.TODO(), promoteOpts.PolicyName, metav1.GetOptions{}); err != nil {
					return fmt.Errorf("failed to create propagation policy %s for the legacy cluster %s, got error: %v", promoteOpts.PolicyName, promoteOpts.Cluster, err)
				}
				return nil
			},
			gvr: schema.GroupVersionResource{
				Group:    "apiextensions.k8s.io",
				Version:  "v1",
				Resource: "customresourcedefinitions",
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.promoteOpts); err != nil {
				t.Fatalf("failed to prep test environment, got error: %v", err)
			}
			_, err := test.promoteOpts.createClusterPropagationPolicy(test.client, test.gvr)
			if err == nil && test.wantErr {
				t.Fatal("expcted an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client, test.promoteOpts); err != nil {
				t.Errorf("failed to verify creating cluster propagation policy, got error: %v", err)
			}
		})
	}
}
