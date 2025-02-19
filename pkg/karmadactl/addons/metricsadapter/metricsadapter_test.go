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

package metricsadapter

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

func TestStatus(t *testing.T) {
	name, namespace := names.KarmadaMetricsAdapterComponentName, "test"
	var replicas int32 = 2
	var priorityClassName = "system-node-critical"
	tests := []struct {
		name       string
		listOpts   *addoninit.CommandAddonsListOption
		prep       func(*addoninit.CommandAddonsListOption) error
		wantStatus string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Status_WithoutKarmadaMetricsAdapter_AddonDisabledStatus",
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
			name: "Status_ForKarmadaMetricsAdapterNotFullyAvailable_AddonUnhealthyStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:     namespace,
					KubeClientSet: fakeclientset.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				if err := createKarmadaMetricsDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClassName); err != nil {
					return fmt.Errorf("failed to create karmada metrics deployment, got error: %v", err)
				}
				return addonutils.SimulateDeploymentUnready(listOpts.KubeClientSet, name, listOpts.Namespace)
			},
			wantStatus: addoninit.AddonUnhealthyStatus,
		},
		{
			name: "Status_WithoutAAAPIService_AddonDisabledStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:                  namespace,
					KubeClientSet:              fakeclientset.NewSimpleClientset(),
					KarmadaAggregatorClientSet: fakeAggregator.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				return createKarmadaMetricsDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClassName)
			},
			wantStatus: addoninit.AddonDisabledStatus,
		},
		{
			name: "Status_WithoutAvailableAPIService_AddonUnhealthyStatus",
			listOpts: &addoninit.CommandAddonsListOption{
				GlobalCommandOptions: addoninit.GlobalCommandOptions{
					Namespace:                  namespace,
					KubeClientSet:              fakeclientset.NewSimpleClientset(),
					KarmadaAggregatorClientSet: fakeAggregator.NewSimpleClientset(),
				},
			},
			prep: func(listOpts *addoninit.CommandAddonsListOption) error {
				if err := createKarmadaMetricsDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClassName); err != nil {
					return fmt.Errorf("failed to create karmada metrics deployment, got error: %v", err)
				}

				if _, err := createAAAPIServices(listOpts.KarmadaAggregatorClientSet); err != nil {
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
				if err := createKarmadaMetricsDeployment(listOpts.KubeClientSet, replicas, listOpts.Namespace, priorityClassName); err != nil {
					return fmt.Errorf("failed to create karmada metrics deployment, got error: %v", err)
				}
				return createAndMarkAAAPIServicesAvailable(listOpts.KarmadaAggregatorClientSet)
			},
			wantStatus: addoninit.AddonEnabledStatus,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.listOpts); err != nil {
				t.Fatalf("failed to prep test env before checking on karmada addon statuses, got error: %v", err)
			}
			addonStatus, err := status(test.listOpts)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Fatalf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if addonStatus != test.wantStatus {
				t.Errorf("expected addon status to be %s, but got %s", test.wantStatus, addonStatus)
			}
		})
	}
}

// createKarmadaMetricsDeployment creates or updates a Deployment for the Karmada metrics adapter
// in the specified namespace with the provided number of replicas.
// It parses and decodes the template for the Deployment before applying it to the cluster.
func createKarmadaMetricsDeployment(c clientset.Interface, replicas int32, namespace, priorityClass string) error {
	karmadaMetricsAdapterDeploymentBytes, err := addonutils.ParseTemplate(karmadaMetricsAdapterDeployment, DeploymentReplace{
		Namespace:         namespace,
		Replicas:          ptr.To[int32](replicas),
		PriorityClassName: priorityClass,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada metrics adapter deployment template :%v", err)
	}

	karmadaMetricsAdapterDeployment := &appsv1.Deployment{}
	if err = kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaMetricsAdapterDeploymentBytes, karmadaMetricsAdapterDeployment); err != nil {
		return fmt.Errorf("decode karmada metrics adapter deployment error: %v", err)
	}
	if err = cmdutil.CreateOrUpdateDeployment(c, karmadaMetricsAdapterDeployment); err != nil {
		return fmt.Errorf("create karmada metrics adapter deployment error: %v", err)
	}
	return nil
}

// createAAAPIServices creates a set of APIService resources for the specified AA API services
// using the provided aggregator client. It returns a list of created APIService objects or an error if creation fails.
func createAAAPIServices(a aggregator.Interface) ([]*apiregistrationv1.APIService, error) {
	var services []*apiregistrationv1.APIService
	for _, aaAPIService := range aaAPIServices {
		apiServiceCreated, err := a.ApiregistrationV1().APIServices().Create(context.TODO(), &apiregistrationv1.APIService{
			ObjectMeta: metav1.ObjectMeta{
				Name: aaAPIService,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create api service, got error: %v", err)
		}
		services = append(services, apiServiceCreated)
	}
	return services, nil
}

// updateAAAPIServicesCondition updates the specified condition type and status
// for each APIService in the provided list using the aggregator client.
// This helps set conditions such as Availability for API services.
func updateAAAPIServicesCondition(services []*apiregistrationv1.APIService, a aggregator.Interface,
	conditionType apiregistrationv1.APIServiceConditionType, conditionStatus apiregistrationv1.ConditionStatus) error {
	for _, service := range services {
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
	}
	return nil
}

// createAndMarkAAAPIServicesAvailable creates the specified AA API services and then
// updates their conditions to mark them as available, setting a "ConditionTrue" status.
// This function is a combination of the creation and condition-setting operations for convenience.
func createAndMarkAAAPIServicesAvailable(a aggregator.Interface) error {
	var aaAPIServicesCreated []*apiregistrationv1.APIService
	aaAPIServicesCreated, err := createAAAPIServices(a)
	if err != nil {
		return err
	}

	return updateAAAPIServicesCondition(
		aaAPIServicesCreated, a, apiregistrationv1.Available,
		apiregistrationv1.ConditionTrue,
	)
}
