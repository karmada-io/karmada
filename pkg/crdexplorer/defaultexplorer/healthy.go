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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/karmada-io/karmada/pkg/util"
)

// healthyFactory return default healthy factory that tells if the object in healthy state.
type healthyFactory func(object runtime.Object) (bool, error)

func getAllDefaultHealthyExplorer() map[schema.GroupVersionKind]healthyFactory {
	explorers := make(map[schema.GroupVersionKind]healthyFactory)
	explorers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = deployHealthyExplorer
	explorers[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = jobHealthyExplorer
	return explorers
}

func deployHealthyExplorer(object runtime.Object) (bool, error) {
	return false, nil
}

func jobHealthyExplorer(object runtime.Object) (bool, error) {
	return false, nil
}
