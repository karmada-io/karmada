/*
Copyright 2026 The Karmada Authors.

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

package scheduler

import (
	"fmt"

	"k8s.io/client-go/tools/cache"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
)

func resourceBindingAffinityGroupIndexer(obj any) ([]string, error) {
	rb, ok := obj.(*workv1alpha2.ResourceBinding)
	if !ok {
		return []string{}, fmt.Errorf("object is not a ResourceBinding: %v", obj)
	}
	workloadAffinityGroups := rb.Spec.WorkloadAffinityGroups
	if workloadAffinityGroups == nil || workloadAffinityGroups.AffinityGroup == "" {
		return []string{}, nil
	}
	return []string{rb.Spec.WorkloadAffinityGroups.AffinityGroup}, nil
}

func resourceBindingAntiAffinityGroupIndexer(obj any) ([]string, error) {
	rb, ok := obj.(*workv1alpha2.ResourceBinding)
	if !ok {
		return []string{}, fmt.Errorf("object is not a ResourceBinding: %v", obj)
	}
	workloadAffinityGroups := rb.Spec.WorkloadAffinityGroups
	if workloadAffinityGroups == nil || workloadAffinityGroups.AntiAffinityGroup == "" {
		return []string{}, nil
	}
	return []string{rb.Spec.WorkloadAffinityGroups.AntiAffinityGroup}, nil
}

// addIndexers adds indexers for ResourceBindings to support efficient lookups.
func (s *Scheduler) addIndexers() {
	rbIndexers := cache.Indexers{}
	if features.FeatureGate.Enabled(features.WorkloadAffinity) {
		rbIndexers[indexregistry.ResourceBindingIndexByAffinityGroup] = resourceBindingAffinityGroupIndexer
		rbIndexers[indexregistry.ResourceBindingIndexByAntiAffinityGroup] = resourceBindingAntiAffinityGroupIndexer
	}

	if len(rbIndexers) == 0 {
		return
	}

	bindingInformer := s.informerFactory.Work().V1alpha2().ResourceBindings().Informer()
	err := bindingInformer.AddIndexers(rbIndexers)
	if err != nil {
		panic(fmt.Errorf("failed to add indexers for ResourceBindings: %v", err))
	}
}
