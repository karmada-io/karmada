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

package coredns

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
)

type mockConditionStore struct {
	conditions []metav1.Condition
}

func (m *mockConditionStore) ListAll() ([]metav1.Condition, error) {
	return m.conditions, nil
}

func (m *mockConditionStore) Load(_ string) (*metav1.Condition, error) {
	return nil, nil
}

func (m *mockConditionStore) Store(_ string, _ *metav1.Condition) error {
	return nil
}

// Note: During test execution, you may observe log messages including DNS lookup failures
// and "node not found" errors. These messages do not indicate test failures, but rather
// simulate real-world scenarios where network issues or resource unavailability might occur.
// The detector is designed to handle these situations gracefully, which is reflected in the
// passing tests.
//
// Specifically:
// 1. DNS lookup failures ("nslookup failed") occur because the test environment doesn't have
//    a real DNS setup. In a production environment, these lookups would typically succeed.
// 2. "Node not found" messages appear because the test is simulating scenarios where a node
//    might not be immediately available.
//
// These log messages provide insight into the detector's behavior under less-than-ideal conditions,
// demonstrating its ability to operate correctly even when facing network or resource issues.

func TestNewCorednsDetector(t *testing.T) {
	tests := []struct {
		name        string
		hostName    string
		clusterName string
		leConfig    componentbaseconfig.LeaderElectionConfiguration
		expectedErr bool
	}{
		{
			name:        "Valid detector creation",
			hostName:    "test-host",
			clusterName: "test-cluster",
			leConfig:    componentbaseconfig.LeaderElectionConfiguration{LeaderElect: false},
			expectedErr: false,
		},
		{
			name:        "Valid detector creation with empty host name",
			hostName:    "",
			clusterName: "test-cluster",
			leConfig:    componentbaseconfig.LeaderElectionConfiguration{LeaderElect: false},
			expectedErr: false,
		},
		{
			name:        "Valid detector creation with empty cluster name",
			hostName:    "test-host",
			clusterName: "",
			leConfig:    componentbaseconfig.LeaderElectionConfiguration{LeaderElect: false},
			expectedErr: false,
		},
		{
			name:        "Detector creation with LeaderElect true",
			hostName:    "test-host",
			clusterName: "test-cluster",
			leConfig: componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect:       true,
				ResourceLock:      "leases",
				ResourceName:      "test-resource",
				ResourceNamespace: "default",
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memberClusterClient := fake.NewSimpleClientset()
			karmadaClient := karmadafake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(memberClusterClient, 0)

			cfg := &Config{
				Period:           time.Second,
				SuccessThreshold: time.Minute,
				FailureThreshold: time.Minute,
				StaleThreshold:   time.Minute,
			}

			detector, err := NewCorednsDetector(memberClusterClient, karmadaClient, informerFactory, tt.leConfig, cfg, tt.hostName, tt.clusterName)

			if tt.expectedErr && err == nil {
				t.Fatal("Expected an error, but got none")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if err == nil {
				if detector == nil {
					t.Fatal("Expected detector to not be nil")
				}
				if detector.nodeName != tt.hostName {
					t.Errorf("Expected nodeName to be '%s', got '%s'", tt.hostName, detector.nodeName)
				}
				if detector.clusterName != tt.clusterName {
					t.Errorf("Expected clusterName to be '%s', got '%s'", tt.clusterName, detector.clusterName)
				}
			}
		})
	}
}

func TestLookupOnce(t *testing.T) {
	tests := []struct {
		name           string
		expectedType   string
		expectedStatus []metav1.ConditionStatus
	}{
		{
			name:           "Valid lookup",
			expectedType:   condType,
			expectedStatus: []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := klog.NewKlogr()
			condition := lookupOnce(logger)

			if condition.Type != tt.expectedType {
				t.Errorf("Expected condition type to be '%s', got '%s'", tt.expectedType, condition.Type)
			}

			validStatus := false
			for _, status := range tt.expectedStatus {
				if condition.Status == status {
					validStatus = true
					break
				}
			}
			if !validStatus {
				t.Errorf("Expected condition status to be one of %v, got '%s'", tt.expectedStatus, condition.Status)
			}

			if condition.Status == metav1.ConditionFalse {
				if condition.Reason != serviceDomainNameResolutionFailed {
					t.Errorf("Expected reason to be '%s', got '%s'", serviceDomainNameResolutionFailed, condition.Reason)
				}
			}
		})
	}
}

func TestShouldAlarm(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []metav1.Condition
		expectedSkip  bool
		expectedAlarm bool
		expectedError bool
	}{
		{
			name:          "No conditions",
			conditions:    []metav1.Condition{},
			expectedSkip:  true,
			expectedAlarm: false,
		},
		{
			name: "All conditions false",
			conditions: []metav1.Condition{
				{Status: metav1.ConditionFalse},
				{Status: metav1.ConditionFalse},
			},
			expectedSkip:  false,
			expectedAlarm: true,
		},
		{
			name: "Mixed conditions",
			conditions: []metav1.Condition{
				{Status: metav1.ConditionFalse},
				{Status: metav1.ConditionTrue},
			},
			expectedSkip:  false,
			expectedAlarm: false,
		},
		{
			name: "With unknown condition",
			conditions: []metav1.Condition{
				{Status: metav1.ConditionUnknown},
				{Status: metav1.ConditionFalse},
			},
			expectedSkip:  true,
			expectedAlarm: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Detector{
				conditionStore: &mockConditionStore{conditions: tt.conditions},
			}

			skip, alarm, err := d.shouldAlarm()

			if tt.expectedError && err == nil {
				t.Error("Expected an error, but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if skip != tt.expectedSkip {
				t.Errorf("Expected skip to be %v, but got %v", tt.expectedSkip, skip)
			}
			if alarm != tt.expectedAlarm {
				t.Errorf("Expected alarm to be %v, but got %v", tt.expectedAlarm, alarm)
			}
		})
	}
}
func TestNewClusterCondition(t *testing.T) {
	tests := []struct {
		name            string
		alarm           bool
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "Alarm is true",
			alarm:           true,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  serviceDomainNameResolutionFailed,
			expectedMessage: "service domain name resolution is unready",
		},
		{
			name:            "Alarm is false",
			alarm:           false,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  serviceDomainNameResolutionReady,
			expectedMessage: "service domain name resolution is ready",
		},
	}

	detector := &Detector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond, err := detector.newClusterCondition(tt.alarm)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if cond.Status != tt.expectedStatus {
				t.Errorf("Expected condition status to be %s, got %s", tt.expectedStatus, cond.Status)
			}
			if cond.Reason != tt.expectedReason {
				t.Errorf("Expected condition reason to be %s, got %s", tt.expectedReason, cond.Reason)
			}
			if cond.Message != tt.expectedMessage {
				t.Errorf("Expected condition message to be %s, got %s", tt.expectedMessage, cond.Message)
			}
		})
	}
}

func TestDetectorRun(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*Detector)
		expectPanic bool
	}{
		{
			name:        "Normal run",
			setupFunc:   func(*Detector) {},
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memberClusterClient := fake.NewSimpleClientset()
			karmadaClient := karmadafake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(memberClusterClient, 0)

			cfg := &Config{
				Period:           time.Second,
				SuccessThreshold: time.Minute,
				FailureThreshold: time.Minute,
				StaleThreshold:   time.Minute,
			}

			leConfig := componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: false,
			}

			detector, err := NewCorednsDetector(
				memberClusterClient,
				karmadaClient,
				informerFactory,
				leConfig,
				cfg,
				"test-host",
				"test-cluster",
			)
			if err != nil {
				t.Fatalf("Failed to create CoreDNS detector: %v", err)
			}

			tt.setupFunc(detector)

			informerFactory.Start(nil)

			ctx, cancel := context.WithCancel(context.Background())

			panicked := make(chan bool)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						panicked <- true
					}
					close(panicked)
				}()
				detector.Run(ctx)
			}()

			time.Sleep(2 * time.Second)
			cancel()
			detector.queue.ShutDown()

			if tt.expectPanic {
				if !<-panicked {
					t.Error("Expected Run to panic, but it didn't")
				}
			} else {
				if <-panicked {
					t.Error("Run panicked unexpectedly")
				}
			}
		})
	}
}

func TestDetectorWorker(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*Detector)
		expectedItems int
	}{
		{
			name: "Process one item",
			setupFunc: func(d *Detector) {
				d.queue.Add(0)
			},
			expectedItems: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memberClusterClient := fake.NewSimpleClientset()
			karmadaClient := karmadafake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(memberClusterClient, 0)

			cfg := &Config{
				Period:           time.Second,
				SuccessThreshold: time.Minute,
				FailureThreshold: time.Minute,
				StaleThreshold:   time.Minute,
			}

			leConfig := componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: false,
			}

			detector, err := NewCorednsDetector(
				memberClusterClient,
				karmadaClient,
				informerFactory,
				leConfig,
				cfg,
				"test-host",
				"test-cluster",
			)
			if err != nil {
				t.Fatalf("Failed to create CoreDNS detector: %v", err)
			}

			informerFactory.Start(nil)
			informerFactory.WaitForCacheSync(nil)

			_, err = memberClusterClient.CoreV1().Nodes().Create(context.Background(), &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-host",
				},
			}, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create test node: %v", err)
			}

			tt.setupFunc(detector)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			go detector.worker(ctx)

			time.Sleep(1 * time.Second)

			if detector.queue.Len() != tt.expectedItems {
				t.Errorf("Expected queue to have %d items, but it has %d items", tt.expectedItems, detector.queue.Len())
			}
		})
	}
}

func TestDetectorProcessNextWorkItem(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(*Detector)
		expectedResult bool
		expectedItems  int
	}{
		{
			name: "Process one item",
			setupFunc: func(d *Detector) {
				d.queue.Add(0)
			},
			expectedResult: true,
			expectedItems:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memberClusterClient := fake.NewSimpleClientset()
			karmadaClient := karmadafake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(memberClusterClient, 0)

			cfg := &Config{
				Period:           time.Second,
				SuccessThreshold: time.Minute,
				FailureThreshold: time.Minute,
				StaleThreshold:   time.Minute,
			}

			leConfig := componentbaseconfig.LeaderElectionConfiguration{
				LeaderElect: false,
			}

			detector, err := NewCorednsDetector(
				memberClusterClient,
				karmadaClient,
				informerFactory,
				leConfig,
				cfg,
				"test-host",
				"test-cluster",
			)
			if err != nil {
				t.Fatalf("Failed to create CoreDNS detector: %v", err)
			}

			informerFactory.Start(nil)
			informerFactory.WaitForCacheSync(nil)

			_, err = memberClusterClient.CoreV1().Nodes().Create(context.Background(), &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-host",
				},
			}, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create test node: %v", err)
			}

			tt.setupFunc(detector)

			ctx := context.Background()

			result := detector.processNextWorkItem(ctx)

			if result != tt.expectedResult {
				t.Errorf("Expected processNextWorkItem to return %v, but got %v", tt.expectedResult, result)
			}

			if detector.queue.Len() != tt.expectedItems {
				t.Errorf("Expected queue to have %d items, but it has %d items", tt.expectedItems, detector.queue.Len())
			}
		})
	}
}

func TestDetectorSync(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*Detector, *karmadafake.Clientset)
		expectedError bool
		expectedConds int
	}{
		{
			name: "Successful sync",
			setupFunc: func(d *Detector, kc *karmadafake.Clientset) {
				d.conditionStore = &mockConditionStore{
					conditions: []metav1.Condition{
						{Status: metav1.ConditionTrue},
					},
				}
				cluster := &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-cluster",
					},
				}
				_, err := kc.ClusterV1alpha1().Clusters().Create(context.Background(), cluster, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create test cluster: %v", err)
				}
			},
			expectedError: false,
			expectedConds: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			karmadaClient := karmadafake.NewSimpleClientset()

			d := &Detector{
				karmadaClient: karmadaClient,
				clusterName:   "test-cluster",
			}

			tt.setupFunc(d, karmadaClient)

			err := d.sync(context.Background())

			if tt.expectedError && err == nil {
				t.Fatal("Expected an error, but got none")
			}
			if !tt.expectedError && err != nil {
				t.Fatalf("Unexpected error from sync: %v", err)
			}

			if err == nil {
				updatedCluster, err := karmadaClient.ClusterV1alpha1().Clusters().Get(context.Background(), "test-cluster", metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get updated cluster: %v", err)
				}

				if len(updatedCluster.Status.Conditions) != tt.expectedConds {
					t.Errorf("Expected %d conditions, but got %d", tt.expectedConds, len(updatedCluster.Status.Conditions))
				}
			}
		})
	}
}
