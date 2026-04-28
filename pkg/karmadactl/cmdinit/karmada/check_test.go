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

package karmada

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	fakeAggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"
)

func TestWaitAPIServiceReady(t *testing.T) {
	aaAPIServiceName := "karmada-search"
	tests := []struct {
		name             string
		aaAPIServiceName string
		client           aggregator.Interface
		timeout          time.Duration
		prep             func(aggregator.Interface) error
		wantErr          bool
		errMsg           string
	}{
		{
			name:             "WaitAPIServiceReady_AAAPIServiceDoesNotExist_Timeout",
			aaAPIServiceName: aaAPIServiceName,
			client:           fakeAggregator.NewSimpleClientset(),
			timeout:          time.Millisecond * 50,
			prep:             func(aggregator.Interface) error { return nil },
			wantErr:          true,
			errMsg:           "context deadline exceeded",
		},
		{
			name:             "WaitAPIServiceReady_AAAPIServiceIsNotReady_Timeout",
			aaAPIServiceName: aaAPIServiceName,
			client:           fakeAggregator.NewSimpleClientset(),
			timeout:          time.Millisecond * 100,
			prep: func(client aggregator.Interface) error {
				if _, err := createAAAPIService(client, aaAPIServiceName); err != nil {
					return fmt.Errorf("failed to create %s aaAPIService, got: %v", aaAPIServiceName, err)
				}
				return nil
			},
			wantErr: true,
			errMsg:  "context deadline exceeded",
		},
		{
			name:             "WaitAPIServiceReady_AAAPIServiceIsReady_ItIsNowReadyToUse",
			aaAPIServiceName: aaAPIServiceName,
			client:           fakeAggregator.NewSimpleClientset(),
			timeout:          time.Millisecond * 50,
			prep: func(client aggregator.Interface) error {
				if err := createAndMarkAAAPIServiceAvailable(client, aaAPIServiceName); err != nil {
					return fmt.Errorf("failed to create and mark availability status of %s aaAPIService, got: %v", aaAPIServiceName, err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client); err != nil {
				t.Fatalf("failed to prep before waiting for API service to be ready, got: %v", err)
			}
			err := WaitAPIServiceReady(test.client, test.aaAPIServiceName, test.timeout)
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

// createAndMarkAAAPIServiceAvailable creates the specified AA APIService and then
// updates its condition status to "Available" by setting the condition status to "ConditionTrue".
// This function simplifies the combined process of creation and availability marking.
func createAndMarkAAAPIServiceAvailable(a aggregator.Interface, aaAPIServiceName string) error {
	aaAPIServerCreated, err := createAAAPIService(a, aaAPIServiceName)
	if err != nil {
		return err
	}

	return updateAAAPIServiceCondition(
		aaAPIServerCreated, a, apiregistrationv1.Available,
		apiregistrationv1.ConditionTrue,
	)
}

// createAAAPIService creates a single APIService resource for the specified AA API
// using the provided aggregator client. It returns the created APIService object or an error
// if the creation fails.
func createAAAPIService(a aggregator.Interface, aaAPIServiceName string) (*apiregistrationv1.APIService, error) {
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
