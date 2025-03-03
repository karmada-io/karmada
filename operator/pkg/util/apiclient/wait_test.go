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
	"net/http"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	fakeAggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"

	"github.com/karmada-io/karmada/operator/pkg/constants"
)

func TestWaitForAPI(t *testing.T) {
	tests := []struct {
		name          string
		karmadaWriter *KarmadaWaiter
		wantErr       bool
	}{
		{
			name: "WaitForAPI_WaitingForAPIServerHealthyStatus_Timeout",
			karmadaWriter: &KarmadaWaiter{
				karmadaConfig: &rest.Config{},
				client: &MockK8SRESTClient{
					RESTClientConnector: &fakerest.RESTClient{
						NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
						Client: fakerest.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
							return nil, fmt.Errorf("unexpected error, endpoint %s does not exist", req.URL.Path)
						}),
					},
				},
				timeout: time.Second,
			},
			wantErr: true,
		},
		{
			name: "WaitForAPI_WaitingForAPIServerHealthyStatus_APIServerIsHealthy",
			karmadaWriter: &KarmadaWaiter{
				karmadaConfig: &rest.Config{},
				client: &MockK8SRESTClient{
					RESTClientConnector: &fakerest.RESTClient{
						NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
						Client: fakerest.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
							if req.URL.Path == "/healthz" {
								// Return a fake 200 OK response.
								return &http.Response{
									StatusCode: http.StatusOK,
									Body:       http.NoBody,
								}, nil
							}
							return nil, fmt.Errorf("unexpected error, endpoint %s does not exist", req.URL.Path)
						}),
					},
				},
				timeout: time.Millisecond,
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.karmadaWriter.WaitForAPI()
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestWaitForAPIService(t *testing.T) {
	name := "karmada-demo-apiservice"
	tests := []struct {
		name          string
		karmadaWriter *KarmadaWaiter
		apiService    *apiregistrationv1.APIService
		client        aggregator.Interface
		prep          func(aggregator.Interface, *apiregistrationv1.APIService) error
		wantErr       bool
	}{
		{
			name: "WaitForAPIService_WaitingForKarmadaAPIServiceAvailableStatus_Timeout",
			karmadaWriter: &KarmadaWaiter{
				karmadaConfig: &rest.Config{},
				timeout:       time.Second,
			},
			apiService: &apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiregistrationv1.APIServiceSpec{
					Service: &apiregistrationv1.ServiceReference{
						Name:      "karmada-demo-service",
						Namespace: "test",
					},
					Version: "v1beta1",
				},
			},
			client: fakeAggregator.NewSimpleClientset(),
			prep: func(client aggregator.Interface, _ *apiregistrationv1.APIService) error {
				aggregateClientFromConfigBuilder = func(*rest.Config) (aggregator.Interface, error) {
					return client, nil
				}
				return nil
			},
			wantErr: true,
		},
		{
			name: "WaitForAPIService_WaitingForKarmadaAPIServiceAvailableStatus_KarmadaAPIServiceIsAvailable",
			karmadaWriter: &KarmadaWaiter{
				karmadaConfig: &rest.Config{},
				timeout:       time.Millisecond,
			},
			apiService: &apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiregistrationv1.APIServiceSpec{
					Service: &apiregistrationv1.ServiceReference{
						Name:      "karmada-demo-service",
						Namespace: "test",
					},
					Version: "v1beta1",
				},
			},
			client: fakeAggregator.NewSimpleClientset(),
			prep: func(client aggregator.Interface, apiService *apiregistrationv1.APIService) error {
				apiServiceCreated, err := client.ApiregistrationV1().APIServices().Create(context.TODO(), apiService, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create api service %s, got err: %v", apiService.Name, err)
				}
				apiServiceCreated.Status = apiregistrationv1.APIServiceStatus{
					Conditions: []apiregistrationv1.APIServiceCondition{
						{
							Type:   apiregistrationv1.Available,
							Status: apiregistrationv1.ConditionTrue,
						},
					},
				}
				if _, err = client.ApiregistrationV1().APIServices().Update(context.TODO(), apiServiceCreated, metav1.UpdateOptions{}); err != nil {
					return fmt.Errorf("failed to update api service with available status, got err: %v", err)
				}
				aggregateClientFromConfigBuilder = func(*rest.Config) (aggregator.Interface, error) {
					return client, nil
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client, test.apiService); err != nil {
				t.Errorf("failed to prep waiting for Karmada API Service, got err: %v", err)
			}
			err := test.karmadaWriter.WaitForAPIService(name)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestWaitForPods(t *testing.T) {
	name, namespace := "karmada-demo-apiserver", "test"
	karmadaAPIServerLabels := labels.Set{constants.AppNameLabel: constants.KarmadaAPIServer}
	var replicas int32 = 2
	tests := []struct {
		name          string
		karmadaWriter *KarmadaWaiter
		prep          func(client clientset.Interface) error
		wantErr       bool
	}{
		{
			name: "WaitForPods_WaitingForAllKarmadaAPIServerPods_Timeout",
			karmadaWriter: &KarmadaWaiter{
				karmadaConfig: &rest.Config{},
				client:        fakeclientset.NewSimpleClientset(),
				timeout:       time.Second,
			},
			prep: func(client clientset.Interface) error {
				_, err := CreatePods(client, namespace, name, replicas, karmadaAPIServerLabels, false)
				if err != nil {
					return fmt.Errorf("failed to create pods, got err: %v", err)
				}
				return nil
			},
			wantErr: true,
		},
		{
			name: "WaitForPods_WaitingForAllKarmadaAPIServerPods_AllAreUpAndRunning",
			karmadaWriter: &KarmadaWaiter{
				karmadaConfig: &rest.Config{},
				client:        fakeclientset.NewSimpleClientset(),
				timeout:       time.Second * 2,
			},
			prep: func(client clientset.Interface) error {
				pods, err := CreatePods(client, namespace, name, replicas, karmadaAPIServerLabels, false)
				if err != nil {
					return fmt.Errorf("failed to create pods, got err: %v", err)
				}
				time.AfterFunc(time.Second, func() {
					for _, pod := range pods {
						if err := UpdatePodStatus(client, pod); err != nil {
							fmt.Printf("failed to update pod status, got err: %v", err)
							return
						}
					}
				})
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.karmadaWriter.client); err != nil {
				t.Errorf("failed to prep before waiting for all Karmada APIServer pods , got err: %v", err)
			}
			err := test.karmadaWriter.WaitForPods(karmadaAPIServerLabels.String(), namespace)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestWaitForSomePods(t *testing.T) {
	name, namespace := "karmada-demo-apiserver", "test"
	karmadaAPIServerLabels := labels.Set{constants.AppNameLabel: constants.KarmadaAPIServer}
	var replicas int32 = 2
	tests := []struct {
		name          string
		karmadaWriter *KarmadaWaiter
		prep          func(client clientset.Interface) error
		wantErr       bool
	}{
		{
			name: "WaitForSomePods_WaitingForSomeKarmadaAPIServerPods_Timeout",
			karmadaWriter: &KarmadaWaiter{
				karmadaConfig: &rest.Config{},
				client:        fakeclientset.NewSimpleClientset(),
				timeout:       time.Second,
			},
			prep: func(client clientset.Interface) error {
				_, err := CreatePods(client, namespace, name, replicas, karmadaAPIServerLabels, false)
				if err != nil {
					return fmt.Errorf("failed to create pods, got err: %v", err)
				}
				return nil
			},
			wantErr: true,
		},
		{
			name: "WaitForSomePods_WaitingForSomeKarmadaAPIServerPods_SomeAreUpAndRunning",
			karmadaWriter: &KarmadaWaiter{
				karmadaConfig: &rest.Config{},
				client:        fakeclientset.NewSimpleClientset(),
				timeout:       time.Millisecond,
			},
			prep: func(client clientset.Interface) error {
				pods, err := CreatePods(client, namespace, name, replicas, karmadaAPIServerLabels, false)
				if err != nil {
					return fmt.Errorf("failed to create pods, got err: %v", err)
				}
				for _, pod := range pods[:1] {
					if err := UpdatePodStatus(client, pod); err != nil {
						return fmt.Errorf("failed to update pod status, got err: %v", err)
					}
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.karmadaWriter.client); err != nil {
				t.Errorf("failed to prep before waiting for some Karmada APIServer pods , got err: %v", err)
			}
			err := test.karmadaWriter.WaitForSomePods(karmadaAPIServerLabels.String(), namespace, 1)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestTryRunCommand(t *testing.T) {
	tests := []struct {
		name             string
		failureThreshold int
		targetFunc       func() error
		prep             func() error
		wantErr          bool
	}{
		{
			name:             "TryRunCommand_HitTheFailureThreshold_CommandTimedOut",
			failureThreshold: 2,
			targetFunc: func() error {
				return errors.New("unexpected error")
			},
			prep: func() error {
				initialBackoffDuration = time.Millisecond
				return nil
			},
			wantErr: true,
		},
		{
			name:             "TryRunCommand_BelowFailureThreshold_CommandRunSuccessfully",
			failureThreshold: 2,
			targetFunc:       func() error { return nil },
			prep: func() error {
				initialBackoffDuration = time.Millisecond
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(); err != nil {
				t.Errorf("failed to prep before trying to running command, got err: %v", err)
			}
			err := TryRunCommand(test.targetFunc, test.failureThreshold)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
		})
	}
}

func TestIsPodRunning(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "IsPodRunning_PodInPendingState_PodIsNotRunningYet",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			want: false,
		},
		{
			name: "IsPodRunning_WithDeletionTimestamp_PodIsNotRunningYet",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			want: false,
		},
		{
			name: "IsPodRunning_PodReadyConditionReadinessIsFalse_PodIsNotRunningYet",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: nil,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "IsPodRunning_PodSatisfyAllRunningConditions_PodIsAlreadyRunning",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: nil,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isPodRunning(*test.pod); got != test.want {
				t.Errorf("expected pod running status %t, but got %t", test.want, got)
			}
		})
	}
}
