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

package pb

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// SetNodeAffinity marshals the given NodeSelector into both NodeAffinity (deprecated) and NodeAffinityBytes fields.
func (m *NodeClaim) SetNodeAffinity(s *corev1.NodeSelector) error {
	if s == nil {
		m.NodeAffinity = nil
		m.NodeAffinityBytes = nil
		return nil
	}
	// Set the old field directly
	m.NodeAffinity = s

	// Set the new field by marshaling
	b, err := s.Marshal()
	if err != nil {
		return err
	}
	m.NodeAffinityBytes = b
	return nil
}

// UnmarshalNodeAffinity unmarshals the NodeAffinityBytes field (or NodeAffinity field as fallback) into a NodeSelector.
func (m *NodeClaim) UnmarshalNodeAffinity() (*corev1.NodeSelector, error) {
	// 1. Try new field first
	if len(m.NodeAffinityBytes) > 0 {
		s := &corev1.NodeSelector{}
		if err := s.Unmarshal(m.NodeAffinityBytes); err != nil {
			return nil, err
		}
		return s, nil
	}

	// 2. Fallback to old field
	if m.NodeAffinity != nil {
		return m.NodeAffinity, nil
	}

	return nil, nil
}

// SetTolerations marshals the given Tolerations into both Tolerations (deprecated) and TolerationsBytes fields.
func (m *NodeClaim) SetTolerations(ts []corev1.Toleration) error {
	if ts == nil {
		m.Tolerations = nil
		m.TolerationsBytes = nil
		return nil
	}

	// Set old field
	// Note: In the generated code, repeated message fields are slices of pointers ([]*corev1.Toleration)
	// We need to convert []corev1.Toleration to []*corev1.Toleration
	m.Tolerations = make([]*corev1.Toleration, len(ts))
	for i := range ts {
		// Deep copy to avoid sharing pointers if necessary, or just take address
		t := ts[i]
		m.Tolerations[i] = &t
	}

	// Set new field
	m.TolerationsBytes = make([][]byte, len(ts))
	for i, t := range ts {
		b, err := t.Marshal()
		if err != nil {
			return err
		}
		m.TolerationsBytes[i] = b
	}
	return nil
}

// UnmarshalTolerations unmarshals the TolerationsBytes field (or Tolerations field as fallback) into a slice of Tolerations.
func (m *NodeClaim) UnmarshalTolerations() ([]corev1.Toleration, error) {
	// 1. Try new field first
	if len(m.TolerationsBytes) > 0 {
		ts := make([]corev1.Toleration, len(m.TolerationsBytes))
		for i, b := range m.TolerationsBytes {
			if err := ts[i].Unmarshal(b); err != nil {
				return nil, err
			}
		}
		return ts, nil
	}

	// 2. Fallback to old field
	if len(m.Tolerations) > 0 {
		ts := make([]corev1.Toleration, len(m.Tolerations))
		for i, t := range m.Tolerations {
			if t != nil {
				ts[i] = *t
			}
		}
		return ts, nil
	}

	return nil, nil
}

// SetResourceRequest marshals the given ResourceList into both ResourceRequest (deprecated) and ResourceRequestBytes fields.
func (m *ReplicaRequirements) SetResourceRequest(res corev1.ResourceList) error {
	if res == nil {
		m.ResourceRequest = nil
		m.ResourceRequestBytes = nil
		return nil
	}

	// Set old field
	m.ResourceRequest = make(map[string]*resource.Quantity)
	for k, v := range res {
		q := v // copy
		m.ResourceRequest[string(k)] = &q
	}

	// Set new field
	m.ResourceRequestBytes = make(map[string][]byte)
	for k, v := range res {
		b, err := v.Marshal()
		if err != nil {
			return err
		}
		m.ResourceRequestBytes[string(k)] = b
	}
	return nil
}

// UnmarshalResourceRequest unmarshals the ResourceRequestBytes field (or ResourceRequest field as fallback) into a ResourceList.
func (m *ReplicaRequirements) UnmarshalResourceRequest() (corev1.ResourceList, error) {
	// 1. Try new field first
	if len(m.ResourceRequestBytes) > 0 {
		res := make(corev1.ResourceList)
		for k, v := range m.ResourceRequestBytes {
			q := resource.Quantity{}
			if err := q.Unmarshal(v); err != nil {
				return nil, err
			}
			res[corev1.ResourceName(k)] = q
		}
		return res, nil
	}

	// 2. Fallback to old field
	if len(m.ResourceRequest) > 0 {
		res := make(corev1.ResourceList)
		for k, v := range m.ResourceRequest {
			if v != nil {
				res[corev1.ResourceName(k)] = *v
			}
		}
		return res, nil
	}

	return nil, nil
}

// SetResourceRequest marshals the given ResourceList into both ResourceRequest (deprecated) and ResourceRequestBytes fields.
func (m *ComponentReplicaRequirements) SetResourceRequest(res corev1.ResourceList) error {
	if res == nil {
		m.ResourceRequest = nil
		m.ResourceRequestBytes = nil
		return nil
	}

	// Set old field
	m.ResourceRequest = make(map[string]*resource.Quantity)
	for k, v := range res {
		q := v
		m.ResourceRequest[string(k)] = &q
	}

	// Set new field
	m.ResourceRequestBytes = make(map[string][]byte)
	for k, v := range res {
		b, err := v.Marshal()
		if err != nil {
			return err
		}
		m.ResourceRequestBytes[string(k)] = b
	}
	return nil
}

// UnmarshalResourceRequest unmarshals the ResourceRequestBytes field (or ResourceRequest field as fallback) into a ResourceList.
func (m *ComponentReplicaRequirements) UnmarshalResourceRequest() (corev1.ResourceList, error) {
	// 1. Try new field first
	if len(m.ResourceRequestBytes) > 0 {
		res := make(corev1.ResourceList)
		for k, v := range m.ResourceRequestBytes {
			q := resource.Quantity{}
			if err := q.Unmarshal(v); err != nil {
				return nil, err
			}
			res[corev1.ResourceName(k)] = q
		}
		return res, nil
	}

	// 2. Fallback to old field
	if len(m.ResourceRequest) > 0 {
		res := make(corev1.ResourceList)
		for k, v := range m.ResourceRequest {
			if v != nil {
				res[corev1.ResourceName(k)] = *v
			}
		}
		return res, nil
	}

	return nil, nil
}

// MustSetNodeAffinity sets node affinity and panics on error.
func (m *NodeClaim) MustSetNodeAffinity(s *corev1.NodeSelector) *NodeClaim {
	if err := m.SetNodeAffinity(s); err != nil {
		panic(err)
	}
	return m
}

// MustSetTolerations sets tolerations and panics on error.
func (m *NodeClaim) MustSetTolerations(ts []corev1.Toleration) *NodeClaim {
	if err := m.SetTolerations(ts); err != nil {
		panic(err)
	}
	return m
}

// MustSetResourceRequest sets resource request and panics on error.
func (m *ReplicaRequirements) MustSetResourceRequest(res corev1.ResourceList) *ReplicaRequirements {
	if err := m.SetResourceRequest(res); err != nil {
		panic(err)
	}
	return m
}

// MustSetResourceRequest sets resource request and panics on error.
func (m *ComponentReplicaRequirements) MustSetResourceRequest(res corev1.ResourceList) *ComponentReplicaRequirements {
	if err := m.SetResourceRequest(res); err != nil {
		panic(err)
	}
	return m
}
