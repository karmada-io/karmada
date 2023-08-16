package execution

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	interpreterfake "github.com/karmada-io/karmada/pkg/resourceinterpreter/fake"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/memberclusterinformer"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

func TestExecutionController_RunWorkQueue(t *testing.T) {
	c := Controller{
		Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)).Build(),
		PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
		RatelimiterOptions: ratelimiterflag.Options{},
	}

	injectMemberClusterInformer(&c, genericmanager.GetInstance(), util.NewClusterDynamicClientSetForAgent)

	c.RunWorkQueue()
}

func TestExecutionController_Reconcile(t *testing.T) {
	tests := []struct {
		name      string
		c         Controller
		dFunc     func(clusterName string, client client.Client) (*util.DynamicClusterClient, error)
		work      *workv1alpha1.Work
		ns        string
		expectRes controllerruntime.Result
		existErr  bool
	}{
		{
			name: "normal case",
			c: Controller{
				Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint:                 "https://127.0.0.1",
							SecretRef:                   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
							InsecureSkipTLSVerification: true,
						},
						Status: clusterv1alpha1.ClusterStatus{
							Conditions: []metav1.Condition{
								{
									Type:   clusterv1alpha1.ClusterConditionReady,
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token")},
					}).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RatelimiterOptions: ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-es-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			dFunc:     util.NewClusterDynamicClientSet,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "work not exists",
			c: Controller{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RatelimiterOptions: ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work-1",
					Namespace: "karmada-es-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "work's DeletionTimestamp isn't zero",
			c: Controller{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RatelimiterOptions: ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "work",
					Namespace:         "karmada-es-cluster",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "failed to get cluster name",
			c: Controller{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RatelimiterOptions: ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-cluster",
			expectRes: controllerruntime.Result{Requeue: true},
			existErr:  true,
		},
		{
			name: "failed to get cluster",
			c: Controller{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster1", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RatelimiterOptions: ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-es-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{Requeue: true},
			existErr:  true,
		},
		{
			name: "cluster is not ready",
			c: Controller{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RatelimiterOptions: ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-es-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{Requeue: true},
			existErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injectMemberClusterInformer(&tt.c, genericmanager.GetInstance(), tt.dFunc)

			tt.c.RunWorkQueue()

			req := controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Name:      "work",
					Namespace: tt.ns,
				},
			}

			if err := tt.c.Create(context.Background(), tt.work); err != nil {
				t.Fatalf("Failed to create work: %v", err)
			}

			res, err := tt.c.Reconcile(context.Background(), req)
			assert.Equal(t, tt.expectRes, res)
			if tt.existErr {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func Test_generateKey(t *testing.T) {
	tests := []struct {
		name      string
		obj       *unstructured.Unstructured
		expectRes controllerruntime.Request
		existRes  bool
		existErr  bool
	}{
		{
			name: "normal case",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"labels": map[string]interface{}{
							workv1alpha1.WorkNamespaceLabel: "karmada-es-cluster",
							workv1alpha1.WorkNameLabel:      "work",
						},
					},
				},
			},
			expectRes: makeRequest("karmada-es-cluster", "work"),
			existRes:  true,
			existErr:  false,
		},
		{
			name: "getWorkNameFromLabel failed",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"labels": map[string]interface{}{
							workv1alpha1.WorkNamespaceLabel: "karmada-es-cluster",
						},
					},
				},
			},
			existRes: false,
			existErr: false,
		},
		{
			name: "getWorkNamespaceFromLabel failed",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"labels": map[string]interface{}{
							workv1alpha1.WorkNameLabel: "work",
						},
					},
				},
			},
			existRes: false,
			existErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueKey, err := generateKey(tt.obj)
			if tt.existRes {
				assert.Equal(t, tt.expectRes, queueKey)
			} else {
				assert.Empty(t, queueKey)
			}

			if tt.existErr {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func newPodObj(name string, labels map[string]string) *unstructured.Unstructured {
	labelsCopy := map[string]interface{}{}

	for k, v := range labels {
		labelsCopy[k] = v
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "default",
				"labels":    labelsCopy,
			},
		},
	}
}

func newPod(name string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
		},
	}
}

func newWorkLabels(workNs, workName string) map[string]string {
	labels := map[string]string{}
	if workNs != "" {
		labels[workv1alpha1.WorkNamespaceLabel] = workNs
	}

	if workName != "" {
		labels[workv1alpha1.WorkNameLabel] = workName
	}

	return labels
}

func newWorkWithStatus(status metav1.ConditionStatus, reason, message string, t time.Time) *workv1alpha1.Work {
	return &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "karmada-es-cluster",
			Name:      "work-name",
		},
		Status: workv1alpha1.WorkStatus{
			Conditions: []metav1.Condition{
				{
					Type:               workv1alpha1.WorkApplied,
					Status:             status,
					Reason:             reason,
					Message:            message,
					LastTransitionTime: metav1.NewTime(t),
				},
			},
		},
	}
}

func TestExecutionController_tryDeleteWorkload(t *testing.T) {
	raw := []byte(`
	{
		"apiVersion":"v1",
		"kind":"Pod",
		"metadata":{
			"name":"pod",
			"namespace":"default",
			"labels":{
				"work.karmada.io/name":"work-name",
				"work.karmada.io/namespace":"karmada-es-cluster"
			}
		}
	}`)
	podName := "pod"
	workName := "work-name"
	workNs := "karmada-es-cluster"
	clusterName := "cluster"
	podGVR := corev1.SchemeGroupVersion.WithResource("pods")

	tests := []struct {
		name                      string
		pod                       *corev1.Pod
		work                      *workv1alpha1.Work
		controllerWithoutInformer bool
		expectedError             bool
		objectNeedDelete          bool
	}{
		{
			name:                      "failed to GetObjectFromCache, wrong InformerManager in ExecutionController",
			pod:                       newPod(podName, newWorkLabels(workNs, workName)),
			work:                      testhelper.NewWork(workName, workNs, raw),
			controllerWithoutInformer: false,
			expectedError:             false,
			objectNeedDelete:          false,
		},
		{
			name:                      "workload is not managed by karmada, without work-related labels",
			pod:                       newPod(podName, nil),
			work:                      testhelper.NewWork(workName, workNs, raw),
			controllerWithoutInformer: true,
			expectedError:             false,
			objectNeedDelete:          false,
		},
		{
			name:                      "workload is not related to current work",
			pod:                       newPod(podName, newWorkLabels(workNs, "wrong-work")),
			work:                      testhelper.NewWork(workName, workNs, raw),
			controllerWithoutInformer: true,
			expectedError:             false,
			objectNeedDelete:          false,
		},
		{
			name:                      "normal case",
			pod:                       newPod(podName, newWorkLabels(workNs, workName)),
			work:                      testhelper.NewWork(workName, workNs, raw),
			controllerWithoutInformer: true,
			expectedError:             false,
			objectNeedDelete:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var dynamicClientSet *dynamicfake.FakeDynamicClient
			if tt.pod != nil {
				dynamicClientSet = dynamicfake.NewSimpleDynamicClient(scheme.Scheme, tt.pod)
			} else {
				dynamicClientSet = dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			}

			fedDynamicClientSet := dynamicClientSet
			if !tt.controllerWithoutInformer {
				fedDynamicClientSet = dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			}

			o := newExecutionOptions()
			o.dynamicClientSet = fedDynamicClientSet

			c := newExecutionController(ctx, o)

			err := c.tryDeleteWorkload(clusterName, tt.work)
			if tt.expectedError {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}

			_, err = dynamicClientSet.Resource(podGVR).Namespace("default").Get(context.Background(), "pod", metav1.GetOptions{})
			if tt.objectNeedDelete {
				assert.True(t, apierrors.IsNotFound(err))
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func TestExecutionController_tryCreateOrUpdateWorkload(t *testing.T) {
	podName := "pod"
	workName := "work-name"
	workNs := "karmada-es-cluster"
	clusterName := "cluster"
	podGVR := corev1.SchemeGroupVersion.WithResource("pods")
	annotations := map[string]string{
		workv1alpha2.ResourceConflictResolutionAnnotation: workv1alpha2.ResourceConflictResolutionOverwrite,
	}

	tests := []struct {
		name           string
		pod            *corev1.Pod
		obj            *unstructured.Unstructured
		withAnnotation bool
		expectedError  bool
		objectExist    bool
		labelMatch     bool
	}{
		{
			name:           "created workload",
			pod:            newPod("wrong-pod", nil),
			obj:            newPodObj(podName, newWorkLabels(workNs, workName)),
			withAnnotation: false,
			expectedError:  false,
			objectExist:    true,
			labelMatch:     true,
		},
		{
			name:           "failed to update object, overwrite conflict resolusion not set",
			pod:            newPod(podName, nil),
			obj:            newPodObj(podName, newWorkLabels(workNs, workName)),
			withAnnotation: false,
			expectedError:  true,
			objectExist:    true,
			labelMatch:     false,
		},
		{
			name:           "updated object",
			pod:            newPod(podName, nil),
			obj:            newPodObj(podName, newWorkLabels(workNs, workName)),
			withAnnotation: true,
			expectedError:  false,
			objectExist:    true,
			labelMatch:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme, tt.pod)

			o := newExecutionOptions()
			o.dynamicClientSet = dynamicClientSet

			c := newExecutionController(ctx, o)

			if tt.withAnnotation {
				tt.obj.SetAnnotations(annotations)
			}
			err := c.tryCreateOrUpdateWorkload(clusterName, tt.obj)
			if tt.expectedError {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}

			resource, err := dynamicClientSet.Resource(podGVR).Namespace("default").Get(context.Background(), "pod", metav1.GetOptions{})
			if tt.objectExist {
				assert.Empty(t, err)
			} else {
				assert.True(t, apierrors.IsNotFound(err))
				return
			}

			labels := map[string]string{workv1alpha1.WorkNamespaceLabel: workNs, workv1alpha1.WorkNameLabel: workName}
			if tt.labelMatch {
				assert.Equal(t, resource.GetLabels(), labels)
			} else {
				assert.Empty(t, resource.GetLabels())
			}
		})
	}
}

func TestExecutionController_updateAppliedConditionIfNeed(t *testing.T) {
	baseTime := time.Now().Add(-time.Minute)

	tests := []struct {
		name    string
		work    *workv1alpha1.Work
		status  metav1.ConditionStatus
		reason  string
		message string
		updated bool
	}{
		{
			name:    "update condition, from false to true",
			work:    newWorkWithStatus(metav1.ConditionFalse, "reason1", "message1", baseTime),
			status:  metav1.ConditionTrue,
			reason:  "",
			message: "",
			updated: true,
		},
		{
			name:    "update condition, from true to false",
			work:    newWorkWithStatus(metav1.ConditionTrue, "", "", baseTime),
			status:  metav1.ConditionFalse,
			reason:  "reason1",
			message: "message1",
			updated: true,
		},
		{
			name:    "update condition, for reason changed",
			work:    newWorkWithStatus(metav1.ConditionFalse, "reason1", "message1", baseTime),
			status:  metav1.ConditionFalse,
			reason:  "reason2",
			message: "message1",
			updated: true,
		},
		{
			name:    "update condition, for message changed",
			work:    newWorkWithStatus(metav1.ConditionFalse, "reason1", "message1", baseTime),
			status:  metav1.ConditionFalse,
			reason:  "reason1",
			message: "message2",
			updated: true,
		},
		{
			name:    "not update condition, for nothing changed",
			work:    newWorkWithStatus(metav1.ConditionFalse, "reason1", "message1", baseTime),
			status:  metav1.ConditionFalse,
			reason:  "reason1",
			message: "message1",
			updated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			o := newExecutionOptions()
			o.objects = append(o.objects, tt.work)
			o.objectsWithStatus = append(o.objectsWithStatus, &workv1alpha1.Work{})

			c := newExecutionController(ctx, o)

			workOld := &workv1alpha1.Work{}
			err := c.Get(ctx, types.NamespacedName{Namespace: tt.work.Namespace, Name: tt.work.Name}, workOld)
			if err != nil {
				t.Errorf("failed to get created work: %v", err)
				return
			}

			t.Logf("Got work: %+v", *workOld)

			appliedCopy := meta.FindStatusCondition(workOld.DeepCopy().Status.Conditions, workv1alpha1.WorkApplied).DeepCopy()
			err = c.updateAppliedConditionIfNeed(workOld, tt.status, tt.reason, tt.message)
			if err != nil {
				t.Errorf("failed to update work: %v", err)
				return
			}

			workNew := &workv1alpha1.Work{}
			err = c.Get(ctx, types.NamespacedName{Namespace: tt.work.Namespace, Name: tt.work.Name}, workNew)
			if err != nil {
				t.Errorf("failed to get updated work: %v", err)
				return
			}

			newApplied := meta.FindStatusCondition(workNew.Status.Conditions, workv1alpha1.WorkApplied)
			if tt.updated {
				if appliedCopy.Status != newApplied.Status {
					assert.NotEqual(t, newApplied.LastTransitionTime, appliedCopy.LastTransitionTime)
				}

				appliedCopy.LastTransitionTime = newApplied.LastTransitionTime
				assert.NotEqual(t, newApplied, appliedCopy)
			} else {
				assert.Equal(t, newApplied, appliedCopy)
			}
		})
	}
}

func TestExecutionController_syncWork(t *testing.T) {
	basePod := newPod("pod", nil)
	workName := "work"
	workNs := "karmada-es-cluster"
	podGVR := corev1.SchemeGroupVersion.WithResource("pods")
	podRaw := []byte(`
	{
		"apiVersion":"v1",
		"kind":"Pod",
		"metadata":{
			"name":"pod",
			"namespace":"default",
			"annotations":{
				"work.karmada.io/conflict-resolution": "overwrite"
			},
			"labels":{
				"work.karmada.io/name":"work",
				"work.karmada.io/namespace":"karmada-es-cluster"
			}
		}
	}`)

	tests := []struct {
		name                      string
		workNamespace             string
		controllerWithoutInformer bool
		expectedError             bool
		clusterNotReady           bool
		updated                   bool
	}{
		{
			name:                      "failed to GetObjectFromCache, wrong InformerManager in ExecutionController",
			workNamespace:             workNs,
			controllerWithoutInformer: false,
			expectedError:             true,
		},
		{
			name:                      "obj not found in informer, wrong dynamicClientSet without pod",
			workNamespace:             workNs,
			controllerWithoutInformer: true,
			expectedError:             false,
		},
		{
			name:                      "workNamespace is zero",
			workNamespace:             "",
			controllerWithoutInformer: true,
			expectedError:             true,
		},
		{
			name:                      "failed to exec Client.Get, set wrong cluster name in work",
			workNamespace:             "karmada-es-wrong-cluster",
			controllerWithoutInformer: true,
			expectedError:             true,
		},
		{
			name:                      "cluster is not ready",
			workNamespace:             workNs,
			controllerWithoutInformer: true,
			expectedError:             true,
			clusterNotReady:           true,
		},
		{
			name:                      "normal case",
			workNamespace:             workNs,
			controllerWithoutInformer: true,
			expectedError:             false,
			updated:                   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			o := newExecutionOptions()
			if tt.clusterNotReady {
				o = newExecutionOptions(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse))
			}

			pod := basePod.DeepCopy()
			dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme, pod)

			if tt.controllerWithoutInformer {
				o.dynamicClientSet = dynamicClientSet
			}

			work := testhelper.NewWork(workName, tt.workNamespace, podRaw)
			o.objects = append(o.objects, work)
			o.objectsWithStatus = append(o.objectsWithStatus, &workv1alpha1.Work{})

			key := makeRequest(work.Namespace, work.Name)

			c := newExecutionController(ctx, o)

			err := c.syncWork(key)
			if tt.expectedError {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}

			if tt.updated {
				resource, err := dynamicClientSet.Resource(podGVR).Namespace(basePod.Namespace).Get(ctx, basePod.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get pod: %v", err)
				}

				expectedLabels := newWorkLabels(workNs, workName)
				assert.Equal(t, resource.GetLabels(), expectedLabels)
			}
		})
	}
}

func injectMemberClusterInformer(c *Controller,
	informerManager genericmanager.MultiClusterInformerManager,
	clusterDynamicClientSetFunc func(clusterName string, client client.Client) (*util.DynamicClusterClient, error),
) {
	r := memberclusterinformer.NewMemberClusterInformer(c.Client, newRESTMapper(), informerManager, metav1.Duration{}, clusterDynamicClientSetFunc)
	c.MemberClusterInformer = r
}

func newRESTMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
	return m
}

type executionOptions struct {
	dynamicClientSet  dynamic.Interface
	cluster           *clusterv1alpha1.Cluster
	objects           []client.Object
	objectsWithStatus []client.Object
}

func newExecutionOptions(cluster ...*clusterv1alpha1.Cluster) *executionOptions {
	o := &executionOptions{}

	if len(cluster) > 0 {
		o.cluster = cluster[0]
	} else {
		o.cluster = testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
	}

	o.objects = append(o.objects, o.cluster)
	o.objectsWithStatus = append(o.objectsWithStatus, &clusterv1alpha1.Cluster{})

	return o
}

func newExecutionController(ctx context.Context, opt *executionOptions) Controller {
	c := Controller{
		Ctx: ctx,
		Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).
			WithObjects(opt.objects...).WithStatusSubresource(opt.objectsWithStatus...).Build(),
		PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
		RatelimiterOptions: ratelimiterflag.Options{},
		EventRecorder:      record.NewFakeRecorder(1024),
	}

	informerManager := genericmanager.GetInstance()
	clusterDynamicClientSetFunc := util.NewClusterDynamicClientSetForAgent

	if opt.dynamicClientSet != nil {
		clusterName := opt.cluster.Name

		// Generate ResourceInterpreter and ObjectWatcher
		resourceInterpreter := interpreterfake.NewFakeInterpreter()
		clusterDynamicClientSetFunc = newClusterDynamicClientSetForAgent(clusterName, opt.dynamicClientSet)

		c.ObjectWatcher = objectwatcher.NewObjectWatcher(c.Client, newRESTMapper(), clusterDynamicClientSetFunc, resourceInterpreter)

		// Generate InformerManager
		m := genericmanager.NewMultiClusterInformerManager(ctx.Done())
		m.ForCluster(clusterName, opt.dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		informerManager = m
	}

	injectMemberClusterInformer(&c, informerManager, clusterDynamicClientSetFunc)

	return c
}

func newClusterDynamicClientSetForAgent(clusterName string, dynamicClientSet dynamic.Interface) func(string, client.Client) (*util.DynamicClusterClient, error) {
	return func(string, client.Client) (*util.DynamicClusterClient, error) {
		return &util.DynamicClusterClient{
			ClusterName:      clusterName,
			DynamicClientSet: dynamicClientSet,
		}, nil
	}
}

func makeRequest(workNamespace, workName string) controllerruntime.Request {
	return controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Namespace: workNamespace,
			Name:      workName,
		},
	}
}
