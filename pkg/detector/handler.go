/*
Copyright 2021 The Karmada Authors.

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

package detector

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

// ClusterWideKeyFunc generates a ClusterWideKey for object.
func ClusterWideKeyFunc(obj interface{}) (util.QueueKey, error) {
	return keys.ClusterWideKeyFunc(obj)
}

const (
	// ObjectChangedByKarmada the key name for a bool value which describes whether the object is changed by Karmada
	ObjectChangedByKarmada = "ObjectChangedByKarmada"
)

// ResourceItem a object key with certain extended config
type ResourceItem struct {
	Obj                     runtime.Object
	ResourceChangeByKarmada bool
}

// ResourceItemKeyFunc generates a ClusterWideKeyWithConfig for object.
func ResourceItemKeyFunc(obj interface{}) (util.QueueKey, error) {
	var err error
	key := keys.ClusterWideKeyWithConfig{}

	resourceItem, ok := obj.(ResourceItem)
	if !ok {
		return key, fmt.Errorf("failed to assert object as ResourceItem")
	}

	key.ResourceChangeByKarmada = resourceItem.ResourceChangeByKarmada
	key.ClusterWideKey, err = keys.ClusterWideKeyFunc(resourceItem.Obj)
	if err != nil {
		return key, err
	}

	return key, nil
}

// PolicyKey is the object key of propagation policy.
type PolicyKey struct {
	keys.ClusterWideKey
	// PermanentID is the permanent ID of the referencing propagation policy.
	PermanentID string
}

// PolicyKeyFunc generates a PolicyKey for object.
func PolicyKeyFunc(obj interface{}) (util.QueueKey, error) {
	key, err := keys.ClusterWideKeyFunc(obj)
	if err != nil {
		return nil, err
	}
	metaInfo, err := meta.Accessor(obj)
	if err != nil { // should not happen
		return nil, fmt.Errorf("object has no meta: %w", err)
	}

	var ppID string
	if len(metaInfo.GetNamespace()) == 0 {
		ppID = metaInfo.GetLabels()[policyv1alpha1.ClusterPropagationPolicyPermanentIDLabel]
	} else {
		ppID = metaInfo.GetLabels()[policyv1alpha1.PropagationPolicyPermanentIDLabel]
	}
	return &PolicyKey{
		ClusterWideKey: key,
		PermanentID:    ppID,
	}, nil
}
