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

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// DedupeAndSortIngressLoadBalancerIngress dedupes and sorts the IngressLoadBalancerIngress slice.
func DedupeAndSortIngressLoadBalancerIngress(ingresses []networkingv1.IngressLoadBalancerIngress) []networkingv1.IngressLoadBalancerIngress {
	errMap := make(map[string]*string)
	for _, ingress := range ingresses {
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
	ingressMap := make(map[string]*networkingv1.IngressLoadBalancerIngress)
	portStatusMap := make(map[string]sets.Set[networkingv1.IngressPortStatus])
	for _, ingress := range ingresses {
		key := fmt.Sprintf("%s/%s", ingress.Hostname, ingress.IP)
		if portStatusMap[key] == nil {
			portStatusMap[key] = sets.New[networkingv1.IngressPortStatus]()
		}
		portStatusMap[key].Insert(ingress.Ports...)
		ingress.Ports = nil
		ingressMap[key] = &ingress
	}
	for key, portStatus := range portStatusMap {
		for port := range portStatus {
			ingressMap[key].Ports = append(ingressMap[key].Ports, port)
		}

		sortIngressPortStatus(ingressMap[key].Ports)
	}
	out := make([]networkingv1.IngressLoadBalancerIngress, 0, len(ingressMap))
	for _, v := range ingressMap {
		out = append(out, *v)
	}

	sortIngressLoadBalancerIngress(out)

	return out
}

func sortIngressPortStatus(ports []networkingv1.IngressPortStatus) {
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

func sortIngressLoadBalancerIngress(ingresses []networkingv1.IngressLoadBalancerIngress) {
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

		return ingresses[i].IP < ingresses[j].IP
	})
}
