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

package pb

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// SetNodeAffinity marshals the given NodeSelector into NodeAffinityBytes field.
func (n *NodeClaim) SetNodeAffinity(s *corev1.NodeSelector) error {
	if s == nil {
		n.NodeAffinityBytes = nil
		return nil
	}

	// Set the new field by marshaling
	b, err := s.Marshal()
	if err != nil {
		return err
	}
	n.NodeAffinityBytes = b
	return nil
}

// UnmarshalNodeAffinity unmarshals the NodeAffinityBytes field into a NodeSelector.
func (n *NodeClaim) UnmarshalNodeAffinity() (*corev1.NodeSelector, error) {
	if n == nil {
		return nil, nil
	}

	if len(n.NodeAffinityBytes) > 0 {
		s := &corev1.NodeSelector{}
		if err := s.Unmarshal(n.NodeAffinityBytes); err != nil {
			return nil, err
		}
		return s, nil
	}

	return nil, nil
}

// SetTolerations marshals the given Tolerations into TolerationsBytes field.
func (n *NodeClaim) SetTolerations(ts []corev1.Toleration) error {
	if len(ts) == 0 {
		n.TolerationsBytes = nil
		return nil
	}

	n.TolerationsBytes = make([][]byte, len(ts))
	for i, t := range ts {
		b, err := t.Marshal()
		if err != nil {
			return err
		}
		n.TolerationsBytes[i] = b
	}
	return nil
}

// UnmarshalTolerations unmarshals the TolerationsBytes field into a slice of Tolerations.
func (n *NodeClaim) UnmarshalTolerations() ([]corev1.Toleration, error) {
	if n == nil {
		return nil, nil
	}

	if len(n.TolerationsBytes) > 0 {
		ts := make([]corev1.Toleration, len(n.TolerationsBytes))
		for i, b := range n.TolerationsBytes {
			if err := ts[i].Unmarshal(b); err != nil {
				return nil, err
			}
		}
		return ts, nil
	}

	return nil, nil
}

// SetResourceRequest marshals the given ResourceList into ResourceRequestBytes field.
func (r *ReplicaRequirements) SetResourceRequest(res corev1.ResourceList) error {
	if len(res) == 0 {
		r.ResourceRequestBytes = nil
		return nil
	}

	// Set new field
	r.ResourceRequestBytes = make(map[string][]byte)
	for k, v := range res {
		b, err := v.Marshal()
		if err != nil {
			return err
		}
		r.ResourceRequestBytes[string(k)] = b
	}
	return nil
}

// UnmarshalResourceRequest unmarshals the ResourceRequestBytes field into a ResourceList.
func (r *ReplicaRequirements) UnmarshalResourceRequest() (corev1.ResourceList, error) {
	if r == nil {
		return nil, nil
	}

	if len(r.ResourceRequestBytes) > 0 {
		res := make(corev1.ResourceList)
		for k, v := range r.ResourceRequestBytes {
			q := resource.Quantity{}
			if err := q.Unmarshal(v); err != nil {
				return nil, err
			}
			res[corev1.ResourceName(k)] = q
		}
		return res, nil
	}

	return nil, nil
}

// SetResourceRequest marshals the given ResourceList into ResourceRequestBytes field.
func (m *ComponentReplicaRequirements) SetResourceRequest(res corev1.ResourceList) error {
	if len(res) == 0 {
		m.ResourceRequestBytes = nil
		return nil
	}

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

// UnmarshalResourceRequest unmarshals the ResourceRequestBytes field into a ResourceList.
func (m *ComponentReplicaRequirements) UnmarshalResourceRequest() (corev1.ResourceList, error) {
	if m == nil {
		return nil, nil
	}

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
	return nil, nil
}

// MustSetNodeAffinity sets node affinity and panics on error.
func (n *NodeClaim) MustSetNodeAffinity(s *corev1.NodeSelector) *NodeClaim {
	if err := n.SetNodeAffinity(s); err != nil {
		panic(err)
	}
	return n
}

// MustSetTolerations sets tolerations and panics on error.
func (n *NodeClaim) MustSetTolerations(ts []corev1.Toleration) *NodeClaim {
	if err := n.SetTolerations(ts); err != nil {
		panic(err)
	}
	return n
}

// MustSetResourceRequest sets resource request and panics on error.
func (r *ReplicaRequirements) MustSetResourceRequest(res corev1.ResourceList) *ReplicaRequirements {
	if err := r.SetResourceRequest(res); err != nil {
		panic(err)
	}
	return r
}

// MustSetResourceRequest sets resource request and panics on error.
func (m *ComponentReplicaRequirements) MustSetResourceRequest(res corev1.ResourceList) *ComponentReplicaRequirements {
	if err := m.SetResourceRequest(res); err != nil {
		panic(err)
	}
	return m
}
