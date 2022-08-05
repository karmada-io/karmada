package helper

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestGenMergePatch(t *testing.T) {
	testObj := workv1alpha2.ResourceBinding{
		TypeMeta:   metav1.TypeMeta{Kind: "ResourceBinding", APIVersion: "work.karmada.io/v1alpha2"},
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec:       workv1alpha2.ResourceBindingSpec{Clusters: []workv1alpha2.TargetCluster{{Name: "foo", Replicas: 20}}},
		Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{{Type: "Dummy", Reason: "Dummy"}}},
	}

	tests := []struct {
		name          string
		modifyFunc    func() interface{}
		expectedPatch string
		expectErr     bool
	}{
		{
			name: "update spec",
			modifyFunc: func() interface{} {
				modified := testObj.DeepCopy()
				modified.Spec.Replicas = 10
				modified.Spec.Clusters = []workv1alpha2.TargetCluster{
					{
						Name:     "m1",
						Replicas: 5,
					},
					{
						Name:     "m2",
						Replicas: 5,
					},
				}
				return modified
			},
			expectedPatch: `{"spec":{"clusters":[{"name":"m1","replicas":5},{"name":"m2","replicas":5}],"replicas":10}}`,
			expectErr:     false,
		},
		{
			name: "update status",
			modifyFunc: func() interface{} {
				modified := testObj.DeepCopy()
				modified.Status.SchedulerObservedGeneration = 10
				modified.Status.Conditions = []metav1.Condition{
					{
						Type:   "Scheduled",
						Reason: "BindingScheduled",
					},
					{
						Type:   "Dummy",
						Reason: "Dummy",
					},
				}
				return modified
			},
			expectedPatch: `{"status":{"conditions":[{"lastTransitionTime":null,"message":"","reason":"BindingScheduled","status":"","type":"Scheduled"},{"lastTransitionTime":null,"message":"","reason":"Dummy","status":"","type":"Dummy"}],"schedulerObservedGeneration":10}}`,
			expectErr:     false,
		},
		{
			name: "no change",
			modifyFunc: func() interface{} {
				modified := testObj.DeepCopy()
				return modified
			},
			expectedPatch: `{}`,
			expectErr:     false,
		},
		{
			name: "invalid input should arise error",
			modifyFunc: func() interface{} {
				var invalid = 0
				return invalid
			},
			expectedPatch: ``, // empty(nil) patch
			expectErr:     true,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(test.name, func(t *testing.T) {
			patch, err := GenMergePatch(testObj, tc.modifyFunc())
			if err != nil && tc.expectErr == false {
				t.Fatalf("unexpect error, but got: %v", err)
			} else if err == nil && tc.expectErr == true {
				t.Fatalf("expect error, but got none")
			}
			if string(patch) != tc.expectedPatch {
				t.Fatalf("want patch: %s, but got :%s", tc.expectedPatch, string(patch))
			}
		})
	}
}
