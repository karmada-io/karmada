/*
Copyright 2025 The Karmada Authors.

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
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// DedupeAndSortServiceLoadBalancerIngress dedupes and sorts the ServiceLoadBalancerIngress slice.
func DedupeAndSortServiceLoadBalancerIngress(ingresses []corev1.LoadBalancerIngress) []corev1.LoadBalancerIngress {
	ipModeMap := make(map[corev1.LoadBalancerIPMode]*corev1.LoadBalancerIPMode)
	errMap := make(map[string]*string)
	for i, ingress := range ingresses {
		if ingress.IPMode != nil {
			// let same IPMode use same ptr, to let *IPMode be comparable
			if ipModeMap[*ingress.IPMode] == nil {
				ipModeMap[*ingress.IPMode] = ingress.IPMode
			} else {
				ingresses[i].IPMode = ipModeMap[*ingress.IPMode]
			}
		}
		for j, port := range ingress.Ports {
			if port.Error != nil {
				// let same error use same ptr, to let *Error be comparable
				if errMap[*port.Error] == nil {
					errMap[*port.Error] = port.Error
				} else {
					ingress.Ports[j].Error = errMap[*port.Error]
				}
			}
		}
	}
	ingressMap := make(map[string]*corev1.LoadBalancerIngress)
	portStatusMap := make(map[string]sets.Set[corev1.PortStatus])
	for _, ingress := range ingresses {
		key := fmt.Sprintf("%s/%s/%p", ingress.Hostname, ingress.IP, ingress.IPMode)
		if portStatusMap[key] == nil {
			portStatusMap[key] = sets.New[corev1.PortStatus]()
		}
		portStatusMap[key].Insert(ingress.Ports...)
		ingress.Ports = nil
		ingressMap[key] = &ingress
	}
	for key, portStatus := range portStatusMap {
		for port := range portStatus {
			ingressMap[key].Ports = append(ingressMap[key].Ports, port)
		}

		sortServicePortStatus(ingressMap[key].Ports)
	}
	out := make([]corev1.LoadBalancerIngress, 0, len(ingressMap))
	for _, v := range ingressMap {
		out = append(out, *v)
	}

	sortServiceLoadBalancerIngress(out)

	return out
}

func sortServicePortStatus(ports []corev1.PortStatus) {
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Port != ports[j].Port {
			return ports[i].Port < ports[j].Port
		}

		if ports[i].Protocol != ports[j].Protocol {
			return ports[i].Protocol < ports[j].Protocol
		}

		if ports[i].Error == nil {
			return true
		}
		if ports[j].Error == nil {
			return false
		}
		return *ports[i].Error < *ports[j].Error
	})
}

func sortServiceLoadBalancerIngress(ingresses []corev1.LoadBalancerIngress) {
	sort.Slice(ingresses, func(i, j int) bool {
		if ingresses[i].Hostname != ingresses[j].Hostname {
			if ingresses[j].Hostname == "" {
				return true
			}
			if ingresses[i].Hostname == "" {
				return false
			}
			return ingresses[i].Hostname < ingresses[j].Hostname
		}

		if ingresses[i].IP != ingresses[j].IP {
			return ingresses[i].IP < ingresses[j].IP
		}

		if ingresses[j].IPMode == nil {
			return true
		}
		if ingresses[i].IPMode == nil {
			return false
		}
		return *ingresses[i].IPMode < *ingresses[j].IPMode
	})
}
