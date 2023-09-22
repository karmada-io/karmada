package hpareplicassyncer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	scalefake "k8s.io/client-go/scale/fake"
	coretesting "k8s.io/client-go/testing"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloadv1alpha1 "github.com/karmada-io/karmada/examples/customresourceinterpreter/apis/workload/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func TestGetGroupResourceAndScaleForWorkloadFromHPA(t *testing.T) {
	deployment := newDeployment("deployment-1", 1)
	workload := newWorkload("workload-1", 1)
	syncer := newHPAReplicasSyncer(deployment, workload)
	cases := []struct {
		name          string
		hpa           *autoscalingv2.HorizontalPodAutoscaler
		expectedError bool
		expectedScale bool
		expectedGR    schema.GroupResource
	}{
		{
			name:          "normal case",
			hpa:           newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 0),
			expectedError: false,
			expectedScale: true,
			expectedGR:    schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
		},
		{
			name:          "customized resource case",
			hpa:           newHPA(workloadv1alpha1.SchemeGroupVersion.String(), "Workload", "workload-1", 0),
			expectedError: false,
			expectedScale: true,
			expectedGR:    schema.GroupResource{Group: workloadv1alpha1.SchemeGroupVersion.Group, Resource: "workloads"},
		},
		{
			name:          "scale not found",
			hpa:           newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-2", 0),
			expectedError: false,
			expectedScale: false,
			expectedGR:    schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
		},
		{
			name:          "resource not registered",
			hpa:           newHPA("fake/v1", "FakeWorkload", "fake-workload-1", 0),
			expectedError: true,
			expectedScale: false,
			expectedGR:    schema.GroupResource{},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			gr, scale, err := syncer.getGroupResourceAndScaleForWorkloadFromHPA(context.TODO(), tt.hpa)

			if tt.expectedError {
				assert.NotEmpty(t, err)
				return
			}
			assert.Empty(t, err)

			if tt.expectedScale {
				assert.NotEmpty(t, scale)
			} else {
				assert.Empty(t, scale)
			}

			assert.Equal(t, tt.expectedGR, gr)
		})
	}
}

func TestUpdateScale(t *testing.T) {
	cases := []struct {
		name          string
		object        client.Object
		gr            schema.GroupResource
		scale         *autoscalingv1.Scale
		hpa           *autoscalingv2.HorizontalPodAutoscaler
		expectedError bool
	}{
		{
			name:          "normal case",
			object:        newDeployment("deployment-1", 0),
			gr:            schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
			scale:         newScale("deployment-1", 0),
			hpa:           newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			expectedError: false,
		},
		{
			name:          "custom resource case",
			object:        newWorkload("workload-1", 0),
			gr:            schema.GroupResource{Group: workloadv1alpha1.SchemeGroupVersion.Group, Resource: "workloads"},
			scale:         newScale("workload-1", 0),
			hpa:           newHPA(workloadv1alpha1.SchemeGroupVersion.String(), "Workload", "workload-1", 3),
			expectedError: false,
		},
		{
			name:          "scale not found",
			object:        newDeployment("deployment-1", 0),
			gr:            schema.GroupResource{Group: "fake", Resource: "fakeworkloads"},
			scale:         newScale("fake-workload-1", 0),
			hpa:           newHPA("fake/v1", "FakeWorkload", "fake-workload-1", 3),
			expectedError: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			syncer := newHPAReplicasSyncer(tt.object)
			err := syncer.updateScale(context.TODO(), tt.gr, tt.scale, tt.hpa)
			if tt.expectedError {
				assert.NotEmpty(t, err)
				return
			}
			assert.Empty(t, err)

			obj := &unstructured.Unstructured{}
			obj.SetAPIVersion(tt.hpa.Spec.ScaleTargetRef.APIVersion)
			obj.SetKind(tt.hpa.Spec.ScaleTargetRef.Kind)

			err = syncer.Client.Get(context.TODO(), types.NamespacedName{Namespace: tt.scale.Namespace, Name: tt.scale.Name}, obj)
			assert.Empty(t, err)
			if err != nil {
				return
			}

			scale, err := getScaleFromUnstructured(obj)
			assert.Empty(t, err)
			if err != nil {
				return
			}

			assert.Equal(t, tt.hpa.Status.DesiredReplicas, scale.Spec.Replicas)
		})
	}
}

func TestRemoveFinalizerIfNeed(t *testing.T) {
	cases := []struct {
		name          string
		hpa           *autoscalingv2.HorizontalPodAutoscaler
		withFinalizer bool
		expectedError bool
	}{
		{
			name:          "hpa with finalizer",
			hpa:           newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			withFinalizer: true,
			expectedError: false,
		},
		{
			name:          "hpa without finalizer",
			hpa:           newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			withFinalizer: false,
			expectedError: false,
		},
		{
			name:          "hpa not found",
			hpa:           newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			withFinalizer: true,
			expectedError: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.withFinalizer {
				tt.hpa.Finalizers = []string{util.HPAReplicasSyncerFinalizer}
			}

			syncer := newHPAReplicasSyncer()
			if !tt.expectedError {
				err := syncer.Client.Create(context.TODO(), tt.hpa)
				assert.Empty(t, err)
				if err != nil {
					return
				}
			}

			err := syncer.removeFinalizerIfNeed(context.TODO(), tt.hpa)
			if tt.expectedError {
				assert.NotEmpty(t, err)
				return
			}
			assert.Empty(t, err)

			newHPA := &autoscalingv2.HorizontalPodAutoscaler{}
			err = syncer.Client.Get(context.TODO(), types.NamespacedName{Namespace: tt.hpa.Namespace, Name: tt.hpa.Name}, newHPA)
			assert.Empty(t, err)
			if err != nil {
				return
			}

			assert.Empty(t, newHPA.Finalizers)

			if tt.withFinalizer {
				assert.Equal(t, "2", newHPA.ResourceVersion)
			} else {
				assert.Equal(t, "1", newHPA.ResourceVersion)
			}
		})
	}
}

func TestAddFinalizerIfNeed(t *testing.T) {
	cases := []struct {
		name          string
		hpa           *autoscalingv2.HorizontalPodAutoscaler
		withFinalizer bool
		expectedError bool
	}{
		{
			name:          "hpa without finalizer",
			hpa:           newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			withFinalizer: false,
			expectedError: false,
		},
		{
			name:          "hpa with finalizer",
			hpa:           newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			withFinalizer: true,
			expectedError: false,
		},
		{
			name:          "hpa not found",
			hpa:           newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			withFinalizer: false,
			expectedError: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.withFinalizer {
				tt.hpa.Finalizers = []string{util.HPAReplicasSyncerFinalizer}
			}

			syncer := newHPAReplicasSyncer()
			if !tt.expectedError {
				err := syncer.Client.Create(context.TODO(), tt.hpa)
				assert.Empty(t, err)
				if err != nil {
					return
				}
			}

			err := syncer.addFinalizerIfNeed(context.TODO(), tt.hpa)
			if tt.expectedError {
				assert.NotEmpty(t, err)
				return
			}
			assert.Empty(t, err)

			newHPA := &autoscalingv2.HorizontalPodAutoscaler{}
			err = syncer.Client.Get(context.TODO(), types.NamespacedName{Namespace: tt.hpa.Namespace, Name: tt.hpa.Name}, newHPA)
			assert.Empty(t, err)
			if err != nil {
				return
			}

			assert.NotEmpty(t, newHPA.Finalizers)

			if !tt.withFinalizer {
				assert.Equal(t, "2", newHPA.ResourceVersion)
			} else {
				assert.Equal(t, "1", newHPA.ResourceVersion)
			}
		})
	}
}

func TestIsReplicasSynced(t *testing.T) {
	cases := []struct {
		name     string
		hpa      *autoscalingv2.HorizontalPodAutoscaler
		scale    *autoscalingv1.Scale
		expected bool
	}{
		{
			name:     "nil hpa",
			hpa:      nil,
			scale:    newScale("deployment-1", 0),
			expected: true,
		},
		{
			name:     "nil scale",
			hpa:      newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			scale:    nil,
			expected: true,
		},
		{
			name:     "nil scale and hpa",
			hpa:      nil,
			scale:    nil,
			expected: true,
		},
		{
			name:     "replicas has not been synced",
			hpa:      newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			scale:    newScale("deployment-1", 0),
			expected: false,
		},
		{
			name:     "replicas has been synced",
			hpa:      newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			scale:    newScale("deployment-1", 3),
			expected: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			res := isReplicasSynced(tt.scale, tt.hpa)

			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestReconcile(t *testing.T) {
	cases := []struct {
		name              string
		object            client.Object
		gr                schema.GroupResource
		hpa               *autoscalingv2.HorizontalPodAutoscaler
		request           controllerruntime.Request
		withFinalizer     bool
		isDeleting        bool
		expectedError     bool
		expectedFinalizer bool
		expectedResult    controllerruntime.Result
		expectedScale     *autoscalingv1.Scale
	}{
		{
			name:              "normal case",
			object:            newDeployment("deployment-1", 0),
			gr:                schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
			hpa:               newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			request:           controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "deployment-1"}},
			withFinalizer:     true,
			isDeleting:        false,
			expectedError:     false,
			expectedFinalizer: true,
			expectedResult:    controllerruntime.Result{Requeue: true},
			expectedScale:     newScale("deployment-1", 3),
		},
		{
			name:              "customized resource normal case",
			object:            newWorkload("workload-1", 0),
			gr:                schema.GroupResource{Group: workloadv1alpha1.SchemeGroupVersion.Group, Resource: "workloads"},
			hpa:               newHPA(workloadv1alpha1.SchemeGroupVersion.String(), "Workload", "workload-1", 3),
			request:           controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "workload-1"}},
			withFinalizer:     true,
			isDeleting:        false,
			expectedError:     false,
			expectedFinalizer: true,
			expectedResult:    controllerruntime.Result{Requeue: true},
			expectedScale:     newScale("workload-1", 3),
		},
		{
			name:              "replicas has been synced",
			object:            newDeployment("deployment-1", 3),
			gr:                schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
			hpa:               newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			request:           controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "deployment-1"}},
			withFinalizer:     true,
			isDeleting:        false,
			expectedError:     false,
			expectedFinalizer: true,
			expectedResult:    controllerruntime.Result{},
			expectedScale:     newScale("deployment-1", 3),
		},
		{
			name:              "hpa not found",
			object:            newDeployment("deployment-1", 0),
			gr:                schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
			hpa:               nil,
			request:           controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "deployment-1"}},
			withFinalizer:     false,
			isDeleting:        false,
			expectedError:     false,
			expectedFinalizer: false,
			expectedResult:    controllerruntime.Result{},
			expectedScale:     nil,
		},
		{
			name:              "scale not found",
			object:            nil,
			gr:                schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
			hpa:               newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			request:           controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "deployment-1"}},
			withFinalizer:     false,
			isDeleting:        false,
			expectedError:     false,
			expectedFinalizer: true,
			expectedResult:    controllerruntime.Result{},
			expectedScale:     nil,
		},
		{
			name:              "without finalizer",
			object:            newDeployment("deployment-1", 0),
			gr:                schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
			hpa:               newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			request:           controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "deployment-1"}},
			withFinalizer:     false,
			isDeleting:        false,
			expectedError:     false,
			expectedFinalizer: true,
			expectedResult:    controllerruntime.Result{Requeue: true},
			expectedScale:     newScale("deployment-1", 3),
		},
		{
			name:              "hpa is deleting but replicas has not been synced",
			object:            newDeployment("deployment-1", 0),
			gr:                schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
			hpa:               newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			request:           controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "deployment-1"}},
			withFinalizer:     true,
			isDeleting:        true,
			expectedError:     false,
			expectedFinalizer: true,
			expectedResult:    controllerruntime.Result{Requeue: true},
			expectedScale:     newScale("deployment-1", 3),
		},
		{
			name:              "hpa is deleting with replicas has been synced",
			object:            newDeployment("deployment-1", 3),
			gr:                schema.GroupResource{Group: appsv1.SchemeGroupVersion.Group, Resource: "deployments"},
			hpa:               newHPA(appsv1.SchemeGroupVersion.String(), "Deployment", "deployment-1", 3),
			request:           controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "deployment-1"}},
			withFinalizer:     true,
			isDeleting:        true,
			expectedError:     false,
			expectedFinalizer: false,
			expectedResult:    controllerruntime.Result{},
			expectedScale:     newScale("deployment-1", 3),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			syncer := newHPAReplicasSyncer()

			if tt.object != nil {
				err := syncer.Client.Create(context.TODO(), tt.object)
				assert.Empty(t, err)
				if err != nil {
					return
				}
			}

			if tt.hpa != nil {
				if tt.withFinalizer {
					tt.hpa.Finalizers = []string{util.HPAReplicasSyncerFinalizer}
				}

				err := syncer.Client.Create(context.TODO(), tt.hpa)
				assert.Empty(t, err)
				if err != nil {
					return
				}
			}

			if tt.isDeleting {
				err := syncer.Client.Delete(context.TODO(), tt.hpa)
				assert.Empty(t, err)
				if err != nil {
					return
				}
			}

			res, err := syncer.Reconcile(context.TODO(), tt.request)
			if tt.expectedError {
				assert.NotEmpty(t, err)
				return
			}
			assert.Empty(t, err)

			assert.Equal(t, tt.expectedResult, res)

			if tt.expectedScale != nil {
				scale, _ := syncer.ScaleClient.Scales(tt.hpa.Namespace).Get(context.TODO(), tt.gr, tt.hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})

				assert.Equal(t, tt.expectedScale, scale)
			}

			if tt.hpa != nil {
				hpa := &autoscalingv2.HorizontalPodAutoscaler{}
				_ = syncer.Client.Get(context.TODO(), types.NamespacedName{Namespace: tt.hpa.Namespace, Name: tt.hpa.Name}, hpa)

				if tt.expectedFinalizer {
					assert.NotEmpty(t, hpa.Finalizers)
				} else {
					assert.Empty(t, hpa.Finalizers)
				}
			}
		})
	}
}

func newHPAReplicasSyncer(objs ...client.Object) *HPAReplicasSyncer {
	scheme := gclient.NewSchema()
	_ = workloadv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	fakeMapper := newMapper()
	fakeScaleClient := &scalefake.FakeScaleClient{}

	fakeScaleClient.AddReactor("get", "*", reactionFuncForGetting(fakeClient, fakeMapper))
	fakeScaleClient.AddReactor("update", "*", reactionFuncForUpdating(fakeClient, fakeMapper))

	return &HPAReplicasSyncer{
		Client:      fakeClient,
		RESTMapper:  fakeMapper,
		ScaleClient: fakeScaleClient,
	}
}

func reactionFuncForGetting(c client.Client, mapper meta.RESTMapper) coretesting.ReactionFunc {
	return func(action coretesting.Action) (bool, runtime.Object, error) {
		getAction, ok := action.(coretesting.GetAction)
		if !ok {
			return false, nil, fmt.Errorf("Not GET Action!")
		}

		obj, err := newUnstructured(getAction.GetResource(), mapper)
		if err != nil {
			return true, nil, err
		}

		nn := types.NamespacedName{Namespace: getAction.GetNamespace(), Name: getAction.GetName()}
		err = c.Get(context.TODO(), nn, obj)
		if err != nil {
			return true, nil, err
		}

		scale, err := getScaleFromUnstructured(obj)

		return true, scale, err
	}
}

func newUnstructured(gvr schema.GroupVersionResource, mapper meta.RESTMapper) (*unstructured.Unstructured, error) {
	gvk, err := mapper.KindFor(gvr)
	if err != nil {
		return nil, err
	}

	un := &unstructured.Unstructured{}
	un.SetGroupVersionKind(gvk)

	return un, nil
}

func getScaleFromUnstructured(obj *unstructured.Unstructured) (*autoscalingv1.Scale, error) {
	replicas := int32(0)
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if ok {
		replicas = int32(spec["replicas"].(int64))
	}

	return &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
		Spec: autoscalingv1.ScaleSpec{
			Replicas: replicas,
		},
		Status: autoscalingv1.ScaleStatus{
			Replicas: replicas,
		},
	}, nil
}

func reactionFuncForUpdating(c client.Client, mapper meta.RESTMapper) coretesting.ReactionFunc {
	return func(action coretesting.Action) (bool, runtime.Object, error) {
		updateAction, ok := action.(coretesting.UpdateAction)
		if !ok {
			return false, nil, fmt.Errorf("Not UPDATE Action!")
		}

		scale, ok := updateAction.GetObject().(*autoscalingv1.Scale)
		if !ok {
			return false, nil, fmt.Errorf("Not autoscalingv1.Scale Object!")
		}

		obj, err := newUnstructured(updateAction.GetResource(), mapper)
		if err != nil {
			return true, nil, err
		}

		nn := types.NamespacedName{Namespace: scale.Namespace, Name: scale.Name}
		err = c.Get(context.TODO(), nn, obj)
		if err != nil {
			return true, nil, err
		}

		updateScaleForUnstructured(obj, scale)

		return true, scale, c.Update(context.TODO(), obj)
	}
}

func updateScaleForUnstructured(obj *unstructured.Unstructured, scale *autoscalingv1.Scale) {
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		spec = map[string]interface{}{}
		obj.Object["spec"] = spec
	}

	spec["replicas"] = scale.Spec.Replicas
}

func newMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	m.Add(appsv1.SchemeGroupVersion.WithKind("Deployment"), meta.RESTScopeNamespace)
	m.Add(workloadv1alpha1.SchemeGroupVersion.WithKind("Workload"), meta.RESTScopeNamespace)
	return m
}

func newDeployment(name string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	}
}

func newWorkload(name string, replicas int32) *workloadv1alpha1.Workload {
	return &workloadv1alpha1.Workload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: workloadv1alpha1.WorkloadSpec{
			Replicas: &replicas,
		},
	}
}

func newHPA(apiVersion, kind, name string, replicas int32) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: apiVersion,
				Kind:       kind,
				Name:       name,
			},
		},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			DesiredReplicas: replicas,
		},
	}
}

func newScale(name string, replicas int32) *autoscalingv1.Scale {
	return &autoscalingv1.Scale{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: autoscalingv1.ScaleSpec{
			Replicas: replicas,
		},
		Status: autoscalingv1.ScaleStatus{
			Replicas: replicas,
		},
	}
}
