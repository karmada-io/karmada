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
package search

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	fakeAggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"
	"k8s.io/utils/ptr"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	addonutils "github.com/karmada-io/karmada/pkg/karmadactl/addons/utils"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestKarmadaSearchAddonStatus(t *testing.T) {
	name, namespace := names.KarmadaSearchComponentName, "test"
	var replicas int32 = 2
	var priorityClass = "system-node-critical"
	tests := []struct {
		name       string
		listOpts   *addoninit.CommandAddonsListOption
		prep       func(*addoninit.CommandAddonsListOption) error
		wantErr    bool
		wantStatus string
		errMsg     string
	}{
		{
			name: "Status_WithoutKarmadaSearch_AddonDisabledStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep:       func(*addoninit.CommandAddonsListOption) error { return nil },
			wantStatus: addoninit.AddonDisabledStatus,
		},
		{
			name: "Status_WithNetworkIssue_AddonUnknownStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				return addonutils.SimulateNetworkErrorOnOp(listOpts.KubeClientSet, "get", "deployments")
			},
			wantStatus: addoninit.AddonUnknownStatus,
			wantErr:    true,
			errMsg:     "unexpected error: encountered a network issue while get the deployments",
		},
		{
			name: "Status_WithKarmadaSearchNotFullyAvailable_AddonUnhealthyStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				if err := createKarmadaSearchDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClass); err != nil {
					return fmt.Errorf("failed to create karmada search deployment, got error: %v", err)
				}
				return addonutils.SimulateDeploymentUnready(listOpts.KubeClientSet, name, listOpts.Namespace)
			},
			wantStatus: addoninit.AddonUnhealthyStatus,
			wantErr:    false,
		},
		{
			name: "Status_WithoutAAAPIServiceOnKarmadaControlplane_AddonDisabledStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:                  namespace,
					KubeClientSet:              fakeclientset.NewSimpleClientset(),
					KarmadaAggregatorClientSet: fakeAggregator.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				return createKarmadaSearchDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClass)
			},
			wantStatus: addoninit.AddonDisabledStatus,
		},
		{
			name: "Status_WithoutAvailableAAAPIServiceServiceOnKarmadaControlPlane_AddonUnhealthyStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:                  namespace,
					KubeClientSet:              fakeclientset.NewSimpleClientset(),
					KarmadaAggregatorClientSet: fakeAggregator.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				if err := createKarmadaSearchDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClass); err != nil {
					return fmt.Errorf("failed to create karmada search deployment, got error: %v", err)
				}

				if _, err := createAAAPIService(listOpts.KarmadaAggregatorClientSet); err != nil {
					return err
				}

				return nil
			},
			wantStatus: addoninit.AddonUnhealthyStatus,
		},
		{
			name: "Status_WithAllAPIServicesAreAvailable_AddonEnabledStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:                  namespace,
					KubeClientSet:              fakeclientset.NewSimpleClientset(),
					KarmadaAggregatorClientSet: fakeAggregator.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				if err := createKarmadaSearchDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClass); err != nil {
					return fmt.Errorf("failed to create karmada search deployment, got error: %v", err)
				}
				return createAndMarkAAAPIServiceAvailable(listOpts.KarmadaAggregatorClientSet)
			},
			wantStatus: addoninit.AddonEnabledStatus,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.listOpts); err != nil {
				t.Fatalf("failed to prep test environment before getting status of karmada search, got: %v", err)
			}
			searchAddonStatus, err := status(test.listOpts)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Fatalf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if searchAddonStatus != test.wantStatus {
				t.Errorf("expected karmada search addon status to be %s, but got %s", test.wantStatus, searchAddonStatus)
			}
		})
	}
}

// createKarmadaSearchDeployment creates or updates a Deployment for the Karmada search deployment
// in the specified namespace with the provided number of replicas.
// It parses and decodes the template for the Deployment before applying it to the cluster.
func createKarmadaSearchDeployment(c clientset.Interface, replicas int32, namespace, priorityClass string) error {
	karmadaSearchDeploymentBytes, err := addonutils.ParseTemplate(karmadaSearchDeployment, DeploymentReplace{
		Namespace:         namespace,
		Replicas:          ptr.To(replicas),
		PriorityClassName: priorityClass,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada search deployment template :%v", err)
	}

	karmadaSearchDeployment := &appsv1.Deployment{}
	if err = kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaSearchDeploymentBytes, karmadaSearchDeployment); err != nil {
		return fmt.Errorf("decode karmada search deployment error: %v", err)
	}
	if err = cmdutil.CreateOrUpdateDeployment(c, karmadaSearchDeployment); err != nil {
		return fmt.Errorf("create karmada search deployment error: %v", err)
	}
	return nil
}

// createAAAPIService creates a single APIService resource for the specified AA API
// using the provided aggregator client. It returns the created APIService object or an error
// if the creation fails.
func createAAAPIService(a aggregator.Interface) (*apiregistrationv1.APIService, error) {
	apiServiceCreated, err := a.ApiregistrationV1().APIServices().Create(context.TODO(), &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: aaAPIServiceName,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create api service, got error: %v", err)
	}
	return apiServiceCreated, nil
}

// createAndMarkAAAPIServiceAvailable creates the specified AA APIService and then
// updates its condition status to "Available" by setting the condition status to "ConditionTrue".
// This function simplifies the combined process of creation and availability marking.
func createAndMarkAAAPIServiceAvailable(a aggregator.Interface) error {
	aaAPIServerCreated, err := createAAAPIService(a)
	if err != nil {
		return err
	}

	return updateAAAPIServiceCondition(
		aaAPIServerCreated, a, apiregistrationv1.Available,
		apiregistrationv1.ConditionTrue,
	)
}

// updateAAAPIServiceCondition updates the specified condition type and status
// for the provided APIService resource using the aggregator client.
// This function sets conditions like "Available" on the APIService to reflect its current state.
func updateAAAPIServiceCondition(service *apiregistrationv1.APIService, a aggregator.Interface,
	conditionType apiregistrationv1.APIServiceConditionType, conditionStatus apiregistrationv1.ConditionStatus) error {
	service.Status.Conditions = []apiregistrationv1.APIServiceCondition{
		{
			Type:   conditionType,
			Status: conditionStatus,
		},
	}
	_, err := a.ApiregistrationV1().APIServices().UpdateStatus(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update status of apiservice, got error: %v", err)
	}
	return nil
}
