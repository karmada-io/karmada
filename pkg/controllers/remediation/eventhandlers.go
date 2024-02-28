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

package remediation

import (
	"context"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
)

func newClusterEventHandler() handler.EventHandler {
	return &clusterEventHandler{}
}

var _ handler.EventHandler = (*clusterEventHandler)(nil)

type clusterEventHandler struct {
}

func (h *clusterEventHandler) Create(_ context.Context, _ event.CreateEvent, _ workqueue.RateLimitingInterface) {
	// Don't care about cluster creation events
}

func (h *clusterEventHandler) Update(_ context.Context, e event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	oldCluster := e.ObjectOld.(*clusterv1alpha1.Cluster)
	newCluster := e.ObjectNew.(*clusterv1alpha1.Cluster)

	if reflect.DeepEqual(oldCluster.Status.Conditions, newCluster.Status.Conditions) {
		return
	}

	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Name: newCluster.Name,
	}})
}

func (h *clusterEventHandler) Delete(_ context.Context, _ event.DeleteEvent, _ workqueue.RateLimitingInterface) {
	// Don't care about cluster deletion events
}

func (h *clusterEventHandler) Generic(_ context.Context, e event.GenericEvent, queue workqueue.RateLimitingInterface) {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{
		Name: e.Object.GetName(),
	}})
}

func newRemedyEventHandler(clusterChan chan<- event.GenericEvent, client client.Client) handler.EventHandler {
	return &remedyEventHandler{
		client:      client,
		clusterChan: clusterChan,
	}
}

var _ handler.EventHandler = (*remedyEventHandler)(nil)

type remedyEventHandler struct {
	client      client.Client
	clusterChan chan<- event.GenericEvent
}

func (h *remedyEventHandler) Create(ctx context.Context, e event.CreateEvent, _ workqueue.RateLimitingInterface) {
	remedy := e.Object.(*remedyv1alpha1.Remedy)
	if remedy.Spec.ClusterAffinity != nil {
		for _, clusterName := range remedy.Spec.ClusterAffinity.ClusterNames {
			h.clusterChan <- event.GenericEvent{Object: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				}}}
		}
		return
	}

	clusterList := &clusterv1alpha1.ClusterList{}
	err := h.client.List(ctx, clusterList)
	if err != nil {
		klog.Errorf("Failed to list cluster: %v", err)
		return
	}

	for _, cluster := range clusterList.Items {
		h.clusterChan <- event.GenericEvent{Object: &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: cluster.Name,
			}}}
	}
}

func (h *remedyEventHandler) Update(ctx context.Context, e event.UpdateEvent, _ workqueue.RateLimitingInterface) {
	oldRemedy := e.ObjectOld.(*remedyv1alpha1.Remedy)
	newRemedy := e.ObjectNew.(*remedyv1alpha1.Remedy)

	if oldRemedy.Spec.ClusterAffinity == nil || newRemedy.Spec.ClusterAffinity == nil {
		clusterList := &clusterv1alpha1.ClusterList{}
		err := h.client.List(ctx, clusterList)
		if err != nil {
			klog.Errorf("Failed to list cluster: %v", err)
			return
		}

		for _, cluster := range clusterList.Items {
			h.clusterChan <- event.GenericEvent{Object: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: cluster.Name,
				}}}
		}
		return
	}

	clusters := sets.Set[string]{}
	for _, clusterName := range oldRemedy.Spec.ClusterAffinity.ClusterNames {
		clusters.Insert(clusterName)
	}
	for _, clusterName := range newRemedy.Spec.ClusterAffinity.ClusterNames {
		clusters.Insert(clusterName)
	}
	for clusterName := range clusters {
		h.clusterChan <- event.GenericEvent{Object: &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			}}}
	}
}

func (h *remedyEventHandler) Delete(ctx context.Context, e event.DeleteEvent, _ workqueue.RateLimitingInterface) {
	remedy := e.Object.(*remedyv1alpha1.Remedy)
	if remedy.Spec.ClusterAffinity != nil {
		for _, clusterName := range remedy.Spec.ClusterAffinity.ClusterNames {
			h.clusterChan <- event.GenericEvent{Object: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				}}}
		}
		return
	}

	clusterList := &clusterv1alpha1.ClusterList{}
	err := h.client.List(ctx, clusterList)
	if err != nil {
		klog.Errorf("Failed to list cluster: %v", err)
		return
	}

	for _, cluster := range clusterList.Items {
		h.clusterChan <- event.GenericEvent{Object: &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: cluster.Name,
			}}}
	}
}

func (h *remedyEventHandler) Generic(_ context.Context, _ event.GenericEvent, _ workqueue.RateLimitingInterface) {
}
