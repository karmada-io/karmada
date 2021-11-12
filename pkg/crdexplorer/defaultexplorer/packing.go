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

// packingFactory return default packing factory that can be used to
// retain necessary field and packing from the input object.
type packingFactory func(object runtime.Object) ([]byte, error)

func getAllDefaultPackingExplorer() map[schema.GroupVersionKind]packingFactory {
	explorers := make(map[schema.GroupVersionKind]packingFactory)
	explorers[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = deployPackingExplorer
	explorers[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = jobPackingExplorer
	return explorers
}

func deployPackingExplorer(object runtime.Object) ([]byte, error) {
	return nil, nil
}

func jobPackingExplorer(object runtime.Object) ([]byte, error) {
	return nil, nil
}
