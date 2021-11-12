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

package helper

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// GetObjectFromCache gets full object information from cache by key in worker queue.
func GetObjectFromCache(restMapper meta.RESTMapper, manager informermanager.MultiClusterInformerManager,
	fedKey keys.FederatedKey) (*unstructured.Unstructured, error) {
	gvr, err := restmapper.GetGroupVersionResource(restMapper, fedKey.GroupVersionKind())
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s. Error: %v", fedKey.GroupVersionKind(), err)
		return nil, err
	}

	singleClusterManager := manager.GetSingleClusterManager(fedKey.Cluster)
	if singleClusterManager == nil {
		// That may happen in case of multi-controllers sharing one informer. For example:
		// controller-A takes responsibility of initialize informer for clusters, but controller-B consumes
		// the informer before the initialization.
		// Usually this error will be eliminated during the controller reconciling loop.
		return nil, fmt.Errorf("the informer of cluster(%s) has not been initialized", fedKey.Cluster)
	}
	var obj runtime.Object
	lister := singleClusterManager.Lister(gvr)
	obj, err = lister.Get(fedKey.NamespaceKey())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, err
		}

		// print logs only for real error.
		klog.Errorf("Failed to get obj %s. error: %v.", fedKey.String(), err)

		return nil, err
	}
	return obj.(*unstructured.Unstructured), nil
}
