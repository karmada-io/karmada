package taint

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

func TestValidate(t *testing.T) {
	taintsToAdd := []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}
	taintsToRemove := []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}

	cases := []struct {
		taintOpt    CommandTaintOption
		description string
		expectFatal bool
	}{
		{
			taintOpt:    CommandTaintOption{resources: []string{"cluster"}},
			description: "Cluster name is not provided",
			expectFatal: true,
		},
		{
			taintOpt:    CommandTaintOption{resources: []string{"node"}},
			description: "invalid resource type",
			expectFatal: true,
		},
		{
			taintOpt:    CommandTaintOption{resources: []string{"cluster", "cluster-name"}, taintsToAdd: taintsToAdd, taintsToRemove: taintsToRemove},
			description: "both modify and remove the following taint(s) in the same command",
			expectFatal: true,
		},
		{
			taintOpt:    CommandTaintOption{resources: []string{"cluster", "cluster-name"}},
			description: "Cluster name is provided",
			expectFatal: false,
		},
	}
	for _, c := range cases {
		sawFatal := false
		err := c.taintOpt.Validate()
		if err != nil {
			sawFatal = true
		}
		if c.expectFatal {
			if !sawFatal {
				t.Fatalf("%s expected not to fail", c.description)
			}
		}
	}
}

func TestParseTaintArgs(t *testing.T) {
	cases := []struct {
		taintOpt    CommandTaintOption
		taintArgs   []string
		description string
		expectFatal bool
	}{
		{
			taintOpt:    CommandTaintOption{},
			taintArgs:   []string{"cluster", "cluster_name", "foo=bar:NoSchedule", "foo_1=bar:NoExecute"},
			description: "Correct order of arguments",
			expectFatal: false,
		},
		{
			taintOpt:    CommandTaintOption{},
			taintArgs:   []string{"cluster", "foo=bar:NoSchedule", "cluster_name", "foo_1=bar:NoExecute"},
			description: "Incorrect order of arguments",
			expectFatal: true,
		},
	}

	for _, c := range cases {
		sawFatal := false
		_, err := c.taintOpt.parseTaintArgs(c.taintArgs)
		if err != nil {
			sawFatal = true
		}
		if c.expectFatal {
			if !sawFatal {
				t.Fatalf("%s expected not to fail", c.description)
			}
		}
	}
}

func TestDeleteTaint(t *testing.T) {
	cases := []struct {
		name           string
		taints         []corev1.Taint
		taintToDelete  *corev1.Taint
		expectedTaints []corev1.Taint
		expectedResult bool
	}{
		{
			name: "delete taint with different name",
			taints: []corev1.Taint{
				{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			taintToDelete: &corev1.Taint{Key: "foo_1", Effect: corev1.TaintEffectNoSchedule},
			expectedTaints: []corev1.Taint{
				{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedResult: false,
		},
		{
			name: "delete taint with different effect",
			taints: []corev1.Taint{
				{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			taintToDelete: &corev1.Taint{Key: "foo", Effect: corev1.TaintEffectNoExecute},
			expectedTaints: []corev1.Taint{
				{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedResult: false,
		},
		{
			name: "delete taint successfully",
			taints: []corev1.Taint{
				{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			taintToDelete:  &corev1.Taint{Key: "foo", Effect: corev1.TaintEffectNoSchedule},
			expectedTaints: nil,
			expectedResult: true,
		},
		{
			name:           "delete taint from empty taint array",
			taints:         []corev1.Taint{},
			taintToDelete:  &corev1.Taint{Key: "foo", Effect: corev1.TaintEffectNoSchedule},
			expectedTaints: nil,
			expectedResult: false,
		},
	}

	for _, c := range cases {
		taints, result := deleteTaint(c.taints, c.taintToDelete)
		if result != c.expectedResult {
			t.Errorf("[%s] should return %t, but, got: %t", c.name, c.expectedResult, result)
		}
		if !reflect.DeepEqual(taints, c.expectedTaints) {
			t.Errorf("[%s] the result taints should be %v, but got: %v", c.name, c.expectedTaints, taints)
		}
	}
}

func TestDeleteTaintByKey(t *testing.T) {
	cases := []struct {
		name           string
		taints         []corev1.Taint
		taintKey       string
		expectedTaints []corev1.Taint
		expectedResult bool
	}{
		{
			name: "delete taint unsuccessfully",
			taints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			taintKey: "foo_1",
			expectedTaints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedResult: false,
		},
		{
			name: "delete taint successfully",
			taints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			taintKey:       "foo",
			expectedTaints: nil,
			expectedResult: true,
		},
		{
			name:           "delete taint from empty taint array",
			taints:         []corev1.Taint{},
			taintKey:       "foo",
			expectedTaints: nil,
			expectedResult: false,
		},
	}

	for _, c := range cases {
		taints, result := deleteTaintsByKey(c.taints, c.taintKey)
		if result != c.expectedResult {
			t.Errorf("[%s] should return %t, but got: %t", c.name, c.expectedResult, result)
		}
		if !reflect.DeepEqual(c.expectedTaints, taints) {
			t.Errorf("[%s] the result taints should be %v, but got: %v", c.name, c.expectedTaints, taints)
		}
	}
}

func TestCheckIfTaintsAlreadyExists(t *testing.T) {
	oldTaints := []corev1.Taint{
		{
			Key:    "foo_1",
			Value:  "bar",
			Effect: corev1.TaintEffectNoSchedule,
		},
		{
			Key:    "foo_2",
			Value:  "bar",
			Effect: corev1.TaintEffectNoSchedule,
		},
		{
			Key:    "foo_3",
			Value:  "bar",
			Effect: corev1.TaintEffectNoSchedule,
		},
	}

	cases := []struct {
		name           string
		taintsToCheck  []corev1.Taint
		expectedResult string
	}{
		{
			name:           "empty array",
			taintsToCheck:  []corev1.Taint{},
			expectedResult: "",
		},
		{
			name: "no match",
			taintsToCheck: []corev1.Taint{
				{
					Key:    "foo_1",
					Effect: corev1.TaintEffectNoExecute,
				},
			},
			expectedResult: "",
		},
		{
			name: "match one taint",
			taintsToCheck: []corev1.Taint{
				{
					Key:    "foo_2",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedResult: "foo_2",
		},
		{
			name: "match two taints",
			taintsToCheck: []corev1.Taint{
				{
					Key:    "foo_2",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "foo_3",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedResult: "foo_2,foo_3",
		},
	}

	for _, c := range cases {
		result := checkIfTaintsAlreadyExists(oldTaints, c.taintsToCheck)
		if result != c.expectedResult {
			t.Errorf("[%s] should return '%s', but got: '%s'", c.name, c.expectedResult, result)
		}
	}
}

func TestReorganizeTaints(t *testing.T) {
	cluster := &clusterv1alpha1.Cluster{
		Spec: clusterv1alpha1.ClusterSpec{
			Taints: []corev1.Taint{
				{
					Key:    "foo",
					Value:  "bar",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	cases := []struct {
		name              string
		overwrite         bool
		taintsToAdd       []corev1.Taint
		taintsToDelete    []corev1.Taint
		expectedTaints    []corev1.Taint
		expectedOperation string
		expectedErr       bool
	}{
		{
			name:              "no changes with overwrite is true",
			overwrite:         true,
			taintsToAdd:       []corev1.Taint{},
			taintsToDelete:    []corev1.Taint{},
			expectedTaints:    cluster.Spec.Taints,
			expectedOperation: MODIFIED,
			expectedErr:       false,
		},
		{
			name:              "no changes with overwrite is false",
			overwrite:         false,
			taintsToAdd:       []corev1.Taint{},
			taintsToDelete:    []corev1.Taint{},
			expectedTaints:    cluster.Spec.Taints,
			expectedOperation: UNTAINTED,
			expectedErr:       false,
		},
		{
			name:      "add new taint",
			overwrite: false,
			taintsToAdd: []corev1.Taint{
				{
					Key:    "foo_1",
					Effect: corev1.TaintEffectNoExecute,
				},
			},
			taintsToDelete:    []corev1.Taint{},
			expectedTaints:    append([]corev1.Taint{{Key: "foo_1", Effect: corev1.TaintEffectNoExecute}}, cluster.Spec.Taints...),
			expectedOperation: TAINTED,
			expectedErr:       false,
		},
		{
			name:        "delete taint with effect",
			overwrite:   false,
			taintsToAdd: []corev1.Taint{},
			taintsToDelete: []corev1.Taint{
				{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedTaints:    nil,
			expectedOperation: UNTAINTED,
			expectedErr:       false,
		},
		{
			name:        "delete taint with no effect",
			overwrite:   false,
			taintsToAdd: []corev1.Taint{},
			taintsToDelete: []corev1.Taint{
				{
					Key: "foo",
				},
			},
			expectedTaints:    nil,
			expectedOperation: UNTAINTED,
			expectedErr:       false,
		},
		{
			name:        "delete non-exist taint",
			overwrite:   false,
			taintsToAdd: []corev1.Taint{},
			taintsToDelete: []corev1.Taint{
				{
					Key:    "foo_1",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedTaints:    cluster.Spec.Taints,
			expectedOperation: UNTAINTED,
			expectedErr:       true,
		},
		{
			name:      "add new taint and delete old one",
			overwrite: false,
			taintsToAdd: []corev1.Taint{
				{
					Key:    "foo_1",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			taintsToDelete: []corev1.Taint{
				{
					Key:    "foo",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedTaints: []corev1.Taint{
				{
					Key:    "foo_1",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			expectedOperation: MODIFIED,
			expectedErr:       false,
		},
	}

	for _, c := range cases {
		operation, taints, err := reorganizeTaints(cluster, c.overwrite, c.taintsToAdd, c.taintsToDelete)
		if c.expectedErr && err == nil {
			t.Errorf("[%s] expect to see an error, but did not get one", c.name)
		} else if !c.expectedErr && err != nil {
			t.Errorf("[%s] expect not to see an error, but got one: %v", c.name, err)
		}

		if !reflect.DeepEqual(c.expectedTaints, taints) {
			t.Errorf("[%s] expect to see taint list %#v, but got: %#v", c.name, c.expectedTaints, taints)
		}

		if c.expectedOperation != operation {
			t.Errorf("[%s] expect to see operation %s, but got: %s", c.name, c.expectedOperation, operation)
		}
	}
}
