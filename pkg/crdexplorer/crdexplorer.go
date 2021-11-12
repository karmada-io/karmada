/*
Copyright The Karmada Authors.

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

package crdexplorer

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/crdexplorer/customizedexplorer"
	"github.com/karmada-io/karmada/pkg/crdexplorer/defaultexplorer"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
)

// CustomResourceExplorer manages both default and customized webhooks to explore custom resource structure.
type CustomResourceExplorer interface {
	// Start starts running the component and will never stop running until the context is closed or an error occurs.
	Start(ctx context.Context) (err error)
	// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
	GetReplicas(object runtime.Object) (replica int32, replicaRequires *workv1alpha2.ReplicaRequirements, err error)
	// GetHealthy tells if the object in healthy state.
	GetHealthy(object runtime.Object) (healthy bool, err error)

	// other common method
}

// NewCustomResourceExplore return a new CustomResourceExplorer object.
func NewCustomResourceExplore(kubeconfig string, informer informermanager.SingleClusterInformerManager) CustomResourceExplorer {
	return &customResourceExplorerImpl{
		kubeconfig: kubeconfig,
		informer:   informer,
	}
}

type customResourceExplorerImpl struct {
	kubeconfig string
	informer   informermanager.SingleClusterInformerManager

	customizedExplorer *customizedexplorer.CustomizedExplorer
	defaultExplorer    *defaultexplorer.DefaultExplorer
}

// Start starts running the component and will never stop running until the context is closed or an error occurs.
func (i *customResourceExplorerImpl) Start(ctx context.Context) (err error) {
	klog.Infof("Starting custom resource explorer.")

	i.customizedExplorer, err = customizedexplorer.NewCustomizedExplorer(i.kubeconfig, i.informer)
	if err != nil {
		return
	}

	i.defaultExplorer = defaultexplorer.NewDefaultExplorer()

	i.informer.Start()
	<-ctx.Done()
	klog.Infof("Stopped as stopCh closed.")
	return nil
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (i *customResourceExplorerImpl) GetReplicas(object runtime.Object) (replica int32, replicaRequires *workv1alpha2.ReplicaRequirements, err error) {
	var hookMatched bool
	replica, replicaRequires, hookMatched, err = i.customizedExplorer.GetReplicas(context.TODO(), configv1alpha1.ExploreReplica, object)
	if hookMatched {
		return
	}

	replica, replicaRequires, err = i.defaultExplorer.GetReplicas(object)
	return
}

// GetHealthy tells if the object in healthy state.
func (i *customResourceExplorerImpl) GetHealthy(object runtime.Object) (healthy bool, err error) {
	return false, nil
}
