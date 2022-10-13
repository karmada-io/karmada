package execution

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	genericmanagertesting "github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager/testing"
	objectwatchertesting "github.com/karmada-io/karmada/pkg/util/objectwatcher/testing"
)

var scheme = runtime.NewScheme()

func TestController_Reconcile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockObjectWatcher := objectwatchertesting.NewMockObjectWatcher(mockCtrl)
	mockInformerManager := genericmanagertesting.NewMockMultiClusterInformerManager(mockCtrl)
	mockSingleClusterInformerManager := genericmanagertesting.NewMockSingleClusterInformerManager(mockCtrl)

	err := workv1alpha1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("error:%v", err)
	}
	err = clusterv1alpha1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("error:%v", err)
	}

	restMapper := meta.NewDefaultRESTMapper(scheme.PreferredVersionAllGroups())
	workGVR := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	restMapper.Add(workGVR, meta.RESTScopeNamespace)

	controller := Controller{}
	controller.ObjectWatcher = mockObjectWatcher
	controller.EventRecorder = record.NewFakeRecorder(100)
	controller.RESTMapper = restMapper
	controller.InformerManager = mockInformerManager

	indexer := cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{})
	lister := cache.NewGenericLister(indexer, schema.GroupResource{})

	tests := []struct {
		name             string
		reqName          string
		reqNamespace     string
		clusterName      string
		clusterStatus    metav1.ConditionStatus
		workloadRaw      []byte
		workApplied      metav1.ConditionStatus
		workloadInDelete bool
		mockFunc         func()
		wantErr          bool
	}{
		{
			name:             "Test everything works and exec successfully",
			reqName:          "work-foo",
			reqNamespace:     "karmada-es-cluster-bar",
			clusterName:      "cluster-bar",
			clusterStatus:    metav1.ConditionTrue,
			workApplied:      metav1.ConditionFalse,
			workloadInDelete: false,
			mockFunc: func() {
				mockObjectWatcher.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:             "Test work is in deleting state and exec successfully",
			reqName:          "work-foo",
			reqNamespace:     "karmada-es-cluster-bar",
			clusterName:      "cluster-bar",
			clusterStatus:    metav1.ConditionTrue,
			workloadInDelete: true,
			workApplied:      metav1.ConditionFalse,
			mockFunc: func() {
				mockInformerManager.EXPECT().GetSingleClusterManager(gomock.Any()).Return(mockSingleClusterInformerManager)
				mockSingleClusterInformerManager.EXPECT().IsInformerSynced(gomock.Any()).Return(true)
				mockSingleClusterInformerManager.EXPECT().Lister(gomock.Any()).Return(lister)
				mockObjectWatcher.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			controller.Client = fake.NewClientBuilder().WithScheme(scheme).Build()

			deploymentRaw, err := newDeploymentRaw(tt.reqName, tt.reqNamespace)
			if err != nil {
				t.Errorf("Create deploymentRaw error:%v", err)
			}

			work := newWork(tt.reqName, tt.reqNamespace, deploymentRaw, tt.workApplied, tt.workloadInDelete)
			if err := controller.Client.Create(ctx, work); err != nil {
				t.Errorf("Create work error:%v", err)
			}

			cluster := newCluster(tt.clusterName, tt.clusterStatus)
			if err := controller.Client.Create(ctx, cluster); err != nil {
				t.Errorf("Create cluster error:%v", err)
			}

			if err := addToMemberClusterCache(indexer, deploymentRaw); err != nil {
				t.Errorf("Add deployment to member cluster cache error:%v", err)
			}

			tt.mockFunc()

			req := newReq(tt.reqName, tt.reqNamespace)
			if _, err := controller.Reconcile(ctx, req); (err != nil) != tt.wantErr {
				t.Errorf("Reconcile() get error:%v, want %v", err, tt.wantErr)
			}
		})
	}
}

func addToMemberClusterCache(indexers cache.Indexer, rawData []byte) error {
	utd := &unstructured.Unstructured{}
	if err := json.Unmarshal(rawData, &utd.Object); err != nil {
		return err
	}
	if err := indexers.Add(utd); err != nil {
		return err
	}
	return nil
}

func newDeploymentRaw(name, namespace string) ([]byte, error) {
	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return json.Marshal(deployment)
}

func newReq(name, namespace string) controllerruntime.Request {
	return controllerruntime.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func newWork(name, namespace string, workloadRaw []byte,
	applied metav1.ConditionStatus, workloadInDelete bool) *workv1alpha1.Work {
	var deletionTimestamp *metav1.Time
	if workloadInDelete {
		deletionTimestamp = &metav1.Time{Time: time.Now()}
	}
	return &workv1alpha1.Work{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "work.karmada.io/v1alpha1",
			Kind:       "Work",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			DeletionTimestamp: deletionTimestamp,
		},
		Spec: workv1alpha1.WorkSpec{
			Workload: workv1alpha1.WorkloadTemplate{
				Manifests: []workv1alpha1.Manifest{
					{RawExtension: runtime.RawExtension{Raw: workloadRaw}},
				},
			},
		},
		Status: workv1alpha1.WorkStatus{
			Conditions: []metav1.Condition{
				{Type: workv1alpha1.WorkApplied, Status: applied},
			},
		},
	}
}

func newCluster(clusterName string, clusterStatus metav1.ConditionStatus) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "cluster.karmada.io/v1alpha1",
			Kind:       "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterName,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{Type: clusterv1alpha1.ClusterConditionReady, Status: clusterStatus},
			},
		},
	}
}
