package mutation

import (
	"math"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
)

func TestStandardizeClusterResourceModels(t *testing.T) {
	testCases := map[string]struct {
		models         []clusterapis.ResourceModel
		expectedModels []clusterapis.ResourceModel
	}{
		"sort models": {
			models: []clusterapis.ResourceModel{
				{
					Grade: 2,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(2, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
					},
				},
			},
			expectedModels: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
					},
				},
				{
					Grade: 2,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(2, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
		},
		"start with 0": {
			models: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(1, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
			expectedModels: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
		},
		"end with MaxInt64": {
			models: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
					},
				},
			},
			expectedModels: []clusterapis.ResourceModel{
				{
					Grade: 1,
					Ranges: []clusterapis.ResourceModelRange{
						{
							Name: clusterapis.ResourceCPU,
							Min:  *resource.NewQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	for name, testCase := range testCases {
		StandardizeClusterResourceModels(testCase.models)
		if !reflect.DeepEqual(testCase.models, testCase.expectedModels) {
			t.Errorf("expected sorted resource models for %q, but it did not work", name)
			return
		}
	}
}
