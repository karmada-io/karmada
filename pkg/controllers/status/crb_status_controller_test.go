package status

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func generateCRBStatusController() *CRBStatusController {
	stopCh := make(chan struct{})
	defer close(stopCh)
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "default"}})
	m := genericmanager.NewSingleClusterInformerManager(dynamicClient, 0, stopCh)
	m.Lister(corev1.SchemeGroupVersion.WithResource("pods"))
	m.Start()
	m.WaitForCacheSync()

	c := &CRBStatusController{
		Client:          fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
		DynamicClient:   dynamicClient,
		InformerManager: m,
		RESTMapper: func() meta.RESTMapper {
			m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
			m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
			return m
		}(),
		EventRecorder: &record.FakeRecorder{},
	}
	return c
}

func TestCRBStatusController_Reconcile(t *testing.T) {
	preTime := metav1.Date(2023, 0, 0, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		binding     *workv1alpha2.ClusterResourceBinding
		expectRes   controllerruntime.Result
		expectError bool
	}{
		{
			name: "failed in syncBindingStatus",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
				},
			},
			expectRes:   controllerruntime.Result{},
			expectError: false,
		},
		{
			name:        "binding not found in client",
			expectRes:   controllerruntime.Result{},
			expectError: false,
		},
		{
			name: "failed in syncBindingStatus",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "binding",
					Namespace:         "default",
					DeletionTimestamp: &preTime,
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
				},
			},
			expectRes:   controllerruntime.Result{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := generateCRBStatusController()

			// Prepare req
			req := controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Name:      "binding",
					Namespace: "default",
				},
			}

			// Prepare binding and create it in client
			if tt.binding != nil {
				if err := c.Client.Create(context.Background(), tt.binding); err != nil {
					t.Fatalf("Failed to create binding: %v", err)
				}
			}

			res, err := c.Reconcile(context.Background(), req)
			assert.Equal(t, tt.expectRes, res)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCRBStatusController_syncBindingStatus(t *testing.T) {
	tests := []struct {
		name                   string
		resource               workv1alpha2.ObjectReference
		podNameInDynamicClient string
		resourceExistInClient  bool
		expectedError          bool
	}{
		{
			name: "failed in FetchResourceTemplate, err is NotFound",
			resource: workv1alpha2.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "pod",
			},
			podNameInDynamicClient: "pod1",
			resourceExistInClient:  true,
			expectedError:          false,
		},
		{
			name:                   "failed in FetchResourceTemplate, err is not NotFound",
			resource:               workv1alpha2.ObjectReference{},
			podNameInDynamicClient: "pod",
			resourceExistInClient:  true,
			expectedError:          true,
		},
		{
			name: "failed in AggregateClusterResourceBindingWorkStatus",
			resource: workv1alpha2.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "pod",
			},
			podNameInDynamicClient: "pod",
			resourceExistInClient:  false,
			expectedError:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := generateCRBStatusController()
			c.DynamicClient = dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: tt.podNameInDynamicClient, Namespace: "default"}})

			binding := &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: tt.resource,
				},
			}

			if tt.resourceExistInClient {
				if err := c.Client.Create(context.Background(), binding); err != nil {
					t.Fatalf("Failed to create binding: %v", err)
				}
			}

			err := c.syncBindingStatus(binding)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
