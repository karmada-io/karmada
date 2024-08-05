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
package federatedresourcequotalimit

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

var (
	testNamespace         = "testNamespace"
	normalResourceRequest = corev1.ResourceList{
		"cpu":    *resource.NewQuantity(2, resource.DecimalSI),
		"memory": *resource.NewQuantity(1024*1024, resource.DecimalSI),
	}
	oversizedResourceRequest = corev1.ResourceList{
		"cpu":    *resource.NewQuantity(10, resource.DecimalSI),
		"memory": *resource.NewQuantity(3*(1024*1024), resource.DecimalSI),
	}
	overallLimitRequestResourceList = corev1.ResourceList{
		"cpu":    *resource.NewQuantity(5, resource.DecimalSI),
		"memory": *resource.NewQuantity(4*(1024*1024), resource.DecimalSI),
	}
	usedLimitRequestResourceList = corev1.ResourceList{
		"cpu":    *resource.NewQuantity(1, resource.DecimalSI),
		"memory": *resource.NewQuantity(1024*1024, resource.DecimalSI),
	}
	noCPULimitsDefined = corev1.ResourceList{
		"memory": *resource.NewQuantity(10*(1024*1024), resource.DecimalSI),
	}
	testFederatedResourceQuota = &policyv1alpha1.FederatedResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testFederatedResourceQuota",
			Namespace: testNamespace,
		},
		Spec: policyv1alpha1.FederatedResourceQuotaSpec{
			Overall: overallLimitRequestResourceList,
		},
		Status: policyv1alpha1.FederatedResourceQuotaStatus{
			Overall:     overallLimitRequestResourceList,
			OverallUsed: usedLimitRequestResourceList,
		},
	}
	unusedFederatedResourceQuota = &policyv1alpha1.FederatedResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unusedFederatedResourceQuota",
			Namespace: testNamespace,
		},
		Spec: policyv1alpha1.FederatedResourceQuotaSpec{
			Overall: overallLimitRequestResourceList,
		},
		Status: policyv1alpha1.FederatedResourceQuotaStatus{
			Overall: overallLimitRequestResourceList,
		},
	}
	noLimitsFederatedResourceQuota = &policyv1alpha1.FederatedResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "noLimitsFederatedResourceQuota",
			Namespace: testNamespace,
		},
		Spec: policyv1alpha1.FederatedResourceQuotaSpec{
			Overall: noCPULimitsDefined,
		},
		Status: policyv1alpha1.FederatedResourceQuotaStatus{
			Overall: noCPULimitsDefined,
		},
	}
)

func Test_checkQuotaLimitsExceeded(t *testing.T) {
	tests := []struct {
		name               string
		quota              *policyv1alpha1.FederatedResourceQuota
		deltaResourceUsage corev1.ResourceList
		objResourceUsage   corev1.ResourceList
		expect             admission.Response
	}{
		{
			"accepted case, resource does not exceed quota limits",
			testFederatedResourceQuota,
			normalResourceRequest,
			normalResourceRequest,
			admission.Allowed(""),
		},
		{
			"accepted case, no current usage on quota",
			unusedFederatedResourceQuota,
			normalResourceRequest,
			normalResourceRequest,
			admission.Allowed(""),
		},
		{
			"accepted case, no limit set for cpu",
			noLimitsFederatedResourceQuota,
			oversizedResourceRequest,
			normalResourceRequest,
			admission.Allowed(""),
		},
		{
			"failure case, oversized resource requirement is denied by quota limits",
			testFederatedResourceQuota,
			oversizedResourceRequest,
			oversizedResourceRequest,
			admission.Denied("Exceeded quota: cpu, requested: 10, used: 1, limited: 5"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if actual := checkQuotaLimitsExceeded(tt.quota, tt.deltaResourceUsage, tt.objResourceUsage); !reflect.DeepEqual(actual, tt.expect) {
				t.Errorf("validateCheckQuotaLimitsExceeded() = %v, want %v", actual, tt.expect)
			}
		})
	}
}
