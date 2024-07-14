/*
Copyright 2022 The Karmada Authors.

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

package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/karmada-io/karmada/pkg/apis/search"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
)

// Install registers the API group and adds types to a scheme.
func Install(scheme *runtime.Scheme) {
	utilruntime.Must(search.AddToScheme(scheme))
	utilruntime.Must(searchv1alpha1.Install(scheme))
	utilruntime.Must(scheme.SetVersionPriority(schema.GroupVersion{Group: searchv1alpha1.GroupVersion.Group, Version: searchv1alpha1.GroupVersion.Version}))
}
