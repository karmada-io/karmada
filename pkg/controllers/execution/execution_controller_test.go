package execution

import (
	"context"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakecontrollerruntimeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"testing"
	"time"
)

func TestController_cleanUpLabelAnnotation(t *testing.T) {
	type fields struct {
		Client             client.Client
		EventRecorder      record.EventRecorder
		RESTMapper         meta.RESTMapper
		ObjectWatcher      objectwatcher.ObjectWatcher
		PredicateFunc      predicate.Predicate
		InformerManager    genericmanager.MultiClusterInformerManager
		RatelimiterOptions ratelimiterflag.Options
	}
	type args struct {
		clusterName string
		work        *workv1alpha1.Work
		client      dynamic.Interface
	}
	restMapper := func() meta.RESTMapper {
		m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
		m.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
		m.Add(appsv1.SchemeGroupVersion.WithKind("Deployment"), meta.RESTScopeNamespace)
		return m
	}()
	tests := []struct {
		name                string
		fields              fields
		args                args
		informerManagerFunc func(clusterName string, client dynamic.Interface) genericmanager.MultiClusterInformerManager
		aop                 func(client dynamic.Interface) func()
		wantErr             bool
	}{
		{
			name: "workload has karamda's labels/annotations",
			fields: fields{
				Client:        fakecontrollerruntimeclient.NewFakeClient(),
				EventRecorder: record.NewFakeRecorder(2000),
				RESTMapper:    restMapper,
			},
			args: args{
				clusterName: "cluster1",
				work: &workv1alpha1.Work{
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment","namespace":"test-namespace","annotations":{"propagationpolicy.karmada.io/name":"demo-pp","resourcetemplate.karmada.io/managed-annotations":"appkey,kubectl.kubernetes.io/last-applied-configuration,propagationpolicy.karmada.io/name,propagationpolicy.karmada.io/namespace,resourcebinding.karmada.io/name,resourcebinding.karmada.io/namespace,resourcetemplate.karmada.io/managed-annotations,resourcetemplate.karmada.io/managed-labels,resourcetemplate.karmada.io/uid,work.karmada.io/conflict-resolution,work.karmada.io/name,work.karmada.io/namespace"},"labels":{"karmada.io/managed":"true"}}}`),
									},
								},
							},
						},
					},
				},
				client: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
			},
			informerManagerFunc: func(clusterName string, client dynamic.Interface) genericmanager.MultiClusterInformerManager {
				mgr := genericmanager.GetInstance()
				mgr.ForCluster(clusterName, client, time.Second*10)
				mgr.Start(clusterName)
				mgr.WaitForCacheSync(clusterName)
				return mgr
			},
			aop: func(client dynamic.Interface) func() {
				utObj := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-namespace",
							"labels": map[string]interface{}{
								"karmada.io/managed": "true",
							},
							"annotations": map[string]interface{}{
								"propagationpolicy.karmada.io/name":               "demo-pp",
								"resourcetemplate.karmada.io/managed-annotations": "appkey,kubectl.kubernetes.io/last-applied-configuration,propagationpolicy.karmada.io/name,propagationpolicy.karmada.io/namespace,resourcebinding.karmada.io/name,resourcebinding.karmada.io/namespace,resourcetemplate.karmada.io/managed-annotations,resourcetemplate.karmada.io/managed-labels,resourcetemplate.karmada.io/uid,work.karmada.io/conflict-resolution,work.karmada.io/name,work.karmada.io/namespace",
							},
						}},
				}
				_, err := client.Resource(schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}).Namespace("test-namespace").Create(context.TODO(), utObj, metav1.CreateOptions{})
				if err != nil {
					panic(err)
				}
				return func() {}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Controller{
				Client:             tt.fields.Client,
				EventRecorder:      tt.fields.EventRecorder,
				RESTMapper:         tt.fields.RESTMapper,
				ObjectWatcher:      tt.fields.ObjectWatcher,
				PredicateFunc:      tt.fields.PredicateFunc,
				InformerManager:    tt.fields.InformerManager,
				RatelimiterOptions: tt.fields.RatelimiterOptions,
			}
			c.ObjectWatcher = objectwatcher.NewObjectWatcher(tt.fields.Client, restMapper, func(c string, client client.Client) (*util.DynamicClusterClient, error) {
				return &util.DynamicClusterClient{DynamicClientSet: tt.args.client, ClusterName: tt.args.clusterName}, nil
			}, nil)
			c.InformerManager = tt.informerManagerFunc(tt.args.clusterName, tt.args.client)
			if tt.aop != nil {
				cancel := tt.aop(tt.args.client)
				defer cancel()
			}
			if err := c.cleanUpLabelAnnotation(tt.args.clusterName, tt.args.work); (err != nil) != tt.wantErr {
				t.Errorf("cleanUpLabelAnnotation() error = %v, wantErr %v", err, tt.wantErr)
			}

		})
	}
}
