/*
Copyright 2023 The Karmada Authors.

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

package cluster

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/endpoints/request"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"

	clusterapis "github.com/karmada-io/karmada/pkg/apis/cluster"
	"github.com/karmada-io/karmada/pkg/apis/cluster/mutation"
	clusterscheme "github.com/karmada-io/karmada/pkg/apis/cluster/scheme"
	"github.com/karmada-io/karmada/pkg/features"
)

func getValidCluster(name string) *clusterapis.Cluster {
	return &clusterapis.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: clusterapis.ClusterSpec{
			ID:          "abc",
			APIEndpoint: "http://0.0.0.0:80",
			SyncMode:    clusterapis.Push,
		},
	}
}

func getValidClusterWithGeneration(name string, generation int64) *clusterapis.Cluster {
	cluster := getValidCluster(name)
	cluster.Generation = generation
	return cluster
}

func setFeatureGateDuringTest(tb testing.TB, gate featuregate.FeatureGate, f featuregate.Feature, value bool) func() {
	originalValue := gate.Enabled(f)

	if err := gate.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=%v", f, value)); err != nil {
		tb.Errorf("error setting %s=%v: %v", f, value, err)
	}

	return func() {
		if err := gate.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=%v", f, originalValue)); err != nil {
			tb.Errorf("error restoring %s=%v: %v", f, originalValue, err)
		}
	}
}

func TestStrategy_NamespaceScoped(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	if clusterStrategy.NamespaceScoped() {
		t.Errorf("Cluster must be cluster scoped")
	}
}

func TestStrategy_PrepareForCreate(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	ctx := request.NewContext()

	defaultResourceModelCluster := getValidClusterWithGeneration("m1", 1)
	mutation.SetDefaultClusterResourceModels(defaultResourceModelCluster)

	standardResourceModelClusterBefore := getValidCluster("m2")
	standardResourceModelClusterBefore.Spec.ResourceModels = []clusterapis.ResourceModel{
		{
			Grade: 2,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: corev1.ResourceCPU,
					Min:  *resource.NewQuantity(2, resource.DecimalSI),
					Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: corev1.ResourceCPU,
					Min:  *resource.NewQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
			},
		}}

	standardResourceModelClusterAfter := getValidClusterWithGeneration("m2", 1)
	standardResourceModelClusterAfter.Spec.ResourceModels = []clusterapis.ResourceModel{
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: corev1.ResourceCPU,
					Min:  *resource.NewQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 2,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: corev1.ResourceCPU,
					Min:  *resource.NewQuantity(2, resource.DecimalSI),
					Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
				},
			},
		},
	}

	tests := []struct {
		name     string
		object   runtime.Object
		expect   runtime.Object
		gateFlag bool
	}{
		{
			name:     "featureGate CustomizedClusterResourceModeling is false",
			object:   getValidCluster("cluster"),
			expect:   getValidClusterWithGeneration("cluster", 1),
			gateFlag: false,
		},
		{
			name:     "featureGate CustomizedClusterResourceModeling is true and cluster.Spec.ResourceModels is nil",
			object:   getValidCluster("m1"),
			expect:   defaultResourceModelCluster,
			gateFlag: true,
		},
		{
			name:     "featureGate CustomizedClusterResourceModeling is true and cluster.Spec.ResourceModels is not nil",
			object:   standardResourceModelClusterBefore,
			expect:   standardResourceModelClusterAfter,
			gateFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeutil.Must(utilfeature.DefaultMutableFeatureGate.Add(features.DefaultFeatureGates))
			setFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, features.CustomizedClusterResourceModeling, tt.gateFlag)

			clusterStrategy.PrepareForCreate(ctx, tt.object)
			if !reflect.DeepEqual(tt.expect, tt.object) {
				t.Errorf("Object mismatch! Excepted: \n%#v \ngot: \n%#v", tt.expect, tt.object)
			}
		})
	}
}

func TestStrategy_PrepareForUpdate(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	ctx := request.NewContext()

	standardResourceModelClusterBefore := getValidClusterWithGeneration("m2", 2)
	standardResourceModelClusterBefore.Spec.ResourceModels = []clusterapis.ResourceModel{
		{
			Grade: 2,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: corev1.ResourceCPU,
					Min:  *resource.NewQuantity(2, resource.DecimalSI),
					Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: corev1.ResourceCPU,
					Min:  *resource.NewQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
			},
		}}

	standardResourceModelClusterAfter := getValidClusterWithGeneration("m2", 2)
	standardResourceModelClusterAfter.Spec.ResourceModels = []clusterapis.ResourceModel{
		{
			Grade: 1,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: corev1.ResourceCPU,
					Min:  *resource.NewQuantity(0, resource.DecimalSI),
					Max:  *resource.NewQuantity(2, resource.DecimalSI),
				},
			},
		},
		{
			Grade: 2,
			Ranges: []clusterapis.ResourceModelRange{
				{
					Name: corev1.ResourceCPU,
					Min:  *resource.NewQuantity(2, resource.DecimalSI),
					Max:  *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
				},
			},
		},
	}

	tests := []struct {
		name     string
		oldObj   runtime.Object
		newObj   runtime.Object
		expect   runtime.Object
		gateFlag bool
	}{
		{
			name:     "cluster spec has no change",
			oldObj:   getValidClusterWithGeneration("cluster", 2),
			newObj:   getValidClusterWithGeneration("cluster", 2),
			expect:   getValidClusterWithGeneration("cluster", 2),
			gateFlag: false,
		},
		{
			name:   "cluster spec has changed",
			oldObj: getValidClusterWithGeneration("cluster", 2),
			newObj: func() *clusterapis.Cluster {
				cluster := getValidClusterWithGeneration("cluster", 2)
				cluster.Spec.Provider = "a"
				return cluster
			}(),
			expect: func() *clusterapis.Cluster {
				cluster := getValidClusterWithGeneration("cluster", 3)
				cluster.Spec.Provider = "a"
				return cluster
			}(),
			gateFlag: false,
		},
		{
			name:     "featureGate CustomizedClusterResourceModeling is false",
			oldObj:   getValidClusterWithGeneration("cluster", 2),
			newObj:   getValidClusterWithGeneration("cluster", 2),
			expect:   getValidClusterWithGeneration("cluster", 2),
			gateFlag: false,
		},
		{
			name:     "featureGate CustomizedClusterResourceModeling is true and cluster.Spec.ResourceModels is nil",
			oldObj:   getValidClusterWithGeneration("m1", 2),
			newObj:   getValidClusterWithGeneration("m1", 2),
			expect:   getValidClusterWithGeneration("m1", 2),
			gateFlag: true,
		},
		{
			name:     "featureGate CustomizedClusterResourceModeling is true and cluster.Spec.ResourceModels is not nil",
			oldObj:   standardResourceModelClusterBefore,
			newObj:   standardResourceModelClusterBefore,
			expect:   standardResourceModelClusterAfter,
			gateFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtimeutil.Must(utilfeature.DefaultMutableFeatureGate.Add(features.DefaultFeatureGates))
			setFeatureGateDuringTest(t, utilfeature.DefaultMutableFeatureGate, features.CustomizedClusterResourceModeling, tt.gateFlag)

			clusterStrategy.PrepareForUpdate(ctx, tt.newObj, tt.oldObj)
			if !reflect.DeepEqual(tt.expect, tt.newObj) {
				t.Errorf("Object mismatch! Excepted: \n%#v \ngot: \n%#b", tt.expect, tt.newObj)
			}
		})
	}
}

func TestStrategy_Validate(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	ctx := request.NewContext()
	cluster := getValidCluster("cluster")

	errs := clusterStrategy.Validate(ctx, cluster)
	if len(errs) > 0 {
		t.Errorf("Cluster is validate but got errs: %v", errs)
	}
}

func TestStrategy_WarningsOnCreate(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	ctx := request.NewContext()
	cluster := getValidCluster("cluster")

	wrs := clusterStrategy.WarningsOnCreate(ctx, cluster)
	if len(wrs) > 0 {
		t.Errorf("Cluster is validate but go warnings: %v", wrs)
	}
}

func TestStrategy_AllowCreateOnUpdate(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	if clusterStrategy.AllowCreateOnUpdate() {
		t.Errorf("Cluster do not allow create on update")
	}
}

func TestStrategy_AllowUnconditionalUpdate(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	if !clusterStrategy.AllowUnconditionalUpdate() {
		t.Errorf("Cluster can be updated unconditionally on update")
	}
}

func TestStrategy_Canonicalize(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	cluster := getValidCluster("cluster")
	cluster.Spec.Taints = []corev1.Taint{
		{
			Key:    "foo",
			Value:  "bar",
			Effect: corev1.TaintEffectNoSchedule,
		},
		{
			Key:    "foo1",
			Value:  "bar1",
			Effect: corev1.TaintEffectNoExecute,
		},
	}

	clusterStrategy.Canonicalize(cluster)
	success := true
	for _, taint := range cluster.Spec.Taints {
		if taint.Effect == corev1.TaintEffectNoExecute && taint.TimeAdded == nil {
			success = false
			break
		}
	}
	if !success {
		t.Errorf("cluster canonicalize error, got: \n%#v", cluster)
	}
}

func TestStrategy_ValidateUpdate(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	ctx := request.NewContext()
	clusterOld := getValidCluster("cluster")
	clusterOld.ResourceVersion = "abc"
	clusterNew := getValidCluster("cluster")
	clusterNew.ResourceVersion = "def"

	errs := clusterStrategy.ValidateUpdate(ctx, clusterOld, clusterNew)
	if len(errs) > 0 {
		t.Errorf("Clusters old and new are both validate but got errs: %v", errs)
	}
}

func TestStrategy_WarningsOnUpdate(t *testing.T) {
	clusterStrategy := NewStrategy(clusterscheme.Scheme)
	ctx := request.NewContext()
	cluster := getValidCluster("cluster")

	wrs := clusterStrategy.WarningsOnUpdate(ctx, cluster, nil)
	if len(wrs) > 0 {
		t.Errorf("Cluster is validate but go warnings: %v", wrs)
	}
}
