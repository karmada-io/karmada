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

package defaultexplorer

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
)

// DefaultExplorer contains all default operation explorer factory
// for exploring common resource.
type DefaultExplorer struct {
	replicaHandlers map[schema.GroupVersionKind]replicaFactory
	packingHandlers map[schema.GroupVersionKind]packingFactory
	healthyHandlers map[schema.GroupVersionKind]healthyFactory
}

// NewDefaultExplorer return a new DefaultExplorer.
func NewDefaultExplorer() *DefaultExplorer {
	return &DefaultExplorer{
		replicaHandlers: getAllDefaultReplicaExplorer(),
		packingHandlers: getAllDefaultPackingExplorer(),
		healthyHandlers: getAllDefaultHealthyExplorer(),
	}
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
func (e *DefaultExplorer) GetReplicas(object runtime.Object) (int32, *workv1alpha2.ReplicaRequirements, error) {
	// judge object type, and then get correct kind.
	_, exist := e.replicaHandlers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)]
	if !exist {
		return 0, &workv1alpha2.ReplicaRequirements{}, fmt.Errorf("defalut explorer for operation %s not found", configv1alpha1.ExploreReplica)
	}
	return e.replicaHandlers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)](object)
}

// GetHealthy tells if the object in healthy state.
func (e *DefaultExplorer) GetHealthy(object runtime.Object) (bool, error) {
	// judge object type, and then get correct kind.
	_, exist := e.healthyHandlers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)]
	if !exist {
		return false, fmt.Errorf("defalut explorer for operation %s not found", configv1alpha1.ExploreHealthy)
	}
	return e.healthyHandlers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)](object)
}
