package objectwatcher

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	interpreterfake "github.com/karmada-io/karmada/pkg/resourceinterpreter/fake"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestObjectWatcher_NeedsUpdate(t *testing.T) {
	deployGVR := appsv1.SchemeGroupVersion.WithResource("deployments")
	tests := []struct {
		name           string
		createInternal bool
		updateInternal bool
		expected       bool
	}{
		{
			name:           "true, empty record",
			createInternal: false,
			updateInternal: false,
			expected:       true,
		},
		{
			name:           "false, update from internal",
			createInternal: true,
			updateInternal: true,
			expected:       false,
		},
		{
			name:           "true, update from client",
			createInternal: true,
			updateInternal: false,
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			resChan := make(chan bool)
			dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			o := newObjectWatcher(dynamicClientSet)

			informer := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClientSet, 0)
			_, err := informer.ForResource(deployGVR).Informer().AddEventHandler(newEventHandlerWithResultChan(o, resChan))
			if err != nil {
				t.Fatalf("Failed to add event handler: %v", err)
			}
			informer.Start(ctx.Done())
			informer.WaitForCacheSync(ctx.Done())

			clusterDeploy := newDeploymentObj(1, 1)
			if tt.createInternal {
				err = o.Create("cluster", clusterDeploy)
				if err != nil {
					t.Fatalf("Failed to create deploy from internal: %v", err)
				}
			} else {
				_, err = dynamicClientSet.Resource(deployGVR).Namespace(clusterDeploy.GetNamespace()).Create(ctx, clusterDeploy, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create deploy from client: %v", err)
				}
			}

			newDeploy := newDeploymentObj(2, 2)
			if tt.updateInternal {
				err = o.Update("cluster", newDeploy, clusterDeploy)
				if err != nil {
					t.Fatalf("Failed to update deploy from internal: %v", err)
				}
			} else {
				_, err = dynamicClientSet.Resource(deployGVR).Namespace(newDeploy.GetNamespace()).Update(ctx, newDeploy, metav1.UpdateOptions{})
				if err != nil {
					t.Fatalf("Failed to update deploy from client: %v", err)
				}
			}

			res := <-resChan
			assert.Equal(t, tt.expected, res)
		})
	}
}

func newDeploymentObj(replicas int32, generation int64) *unstructured.Unstructured {
	deployObj := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "deployment",
			"namespace": "default",
			"labels": map[string]interface{}{
				workv1alpha1.WorkNamespaceLabel: "karmada-es-cluster",
				workv1alpha1.WorkNameLabel:      "work",
			},
			"generation": generation,
		},
		"spec": map[string]interface{}{
			"replicas": &replicas,
		},
	}

	deployBytes, _ := json.Marshal(deployObj)
	deployUnstructured := &unstructured.Unstructured{}
	_ = deployUnstructured.UnmarshalJSON(deployBytes)

	return deployUnstructured
}

func newRESTMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{appsv1.SchemeGroupVersion})
	m.Add(appsv1.SchemeGroupVersion.WithKind("Deployment"), meta.RESTScopeNamespace)
	return m
}

func newObjectWatcher(dynamicClientSets ...dynamic.Interface) ObjectWatcher {
	c := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build()

	clusterDynamicClientSetFunc := util.NewClusterDynamicClientSetForAgent

	if len(dynamicClientSets) > 0 {
		clusterDynamicClientSetFunc = newClusterDynamicClientSetForAgent("cluster", dynamicClientSets[0])
	}

	return NewObjectWatcher(c, newRESTMapper(), clusterDynamicClientSetFunc, interpreterfake.NewFakeInterpreter())
}

func newClusterDynamicClientSetForAgent(clusterName string, dynamicClientSet dynamic.Interface) func(string, client.Client) (*util.DynamicClusterClient, error) {
	return func(string, client.Client) (*util.DynamicClusterClient, error) {
		return &util.DynamicClusterClient{
			ClusterName:      clusterName,
			DynamicClientSet: dynamicClientSet,
		}, nil
	}
}

func newEventHandlerWithResultChan(o ObjectWatcher, resChan chan<- bool) cache.ResourceEventHandler {
	return &cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			objOld := oldObj.(*unstructured.Unstructured)
			objNew := newObj.(*unstructured.Unstructured)

			res := false
			clusterName, _ := names.GetClusterName(util.GetLabelValue(objNew.GetLabels(), workv1alpha1.WorkNamespaceLabel))
			if clusterName != "" {
				res = o.NeedsUpdate(clusterName, objOld, objNew)
			}

			resChan <- res
		},
	}
}
