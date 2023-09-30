package helper

import (
	"math"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func TestGenMergePatch(t *testing.T) {
	testObj := &workv1alpha2.ResourceBinding{
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
			expectedPatch: "",
			expectErr:     false,
		},
		{
			name: "invalid input should arise error",
			modifyFunc: func() interface{} {
				var invalid = 0
				return invalid
			},
			expectedPatch: "",
			expectErr:     true,
		},
		{
			name: "update to empty annotations",
			modifyFunc: func() interface{} {
				modified := testObj.DeepCopy()
				modified.Annotations = make(map[string]string, 0)
				return modified
			},
			expectedPatch: "",
			expectErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := GenMergePatch(testObj, tt.modifyFunc())
			if err != nil && tt.expectErr == false {
				t.Fatalf("unexpect error, but got: %v", err)
			} else if err == nil && tt.expectErr == true {
				t.Fatalf("expect error, but got none")
			}
			if string(patch) != tt.expectedPatch {
				t.Fatalf("want patch: %s, but got :%s", tt.expectedPatch, string(patch))
			}
		})
	}
}

func TestGenJSONPatch(t *testing.T) {
	//doc := `{"field1":1,"field2":"2","field3":[3],"field4":{"a":"a"}}`
	type args struct {
		op    string
		from  string
		path  string
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "add object field",
			args: args{
				op:    "add",
				path:  "/abc",
				value: 1,
			},
			want:    `[{"op":"add","path":"/abc","value":1}]`,
			wantErr: false,
		},
		{
			name: "replace object field",
			args: args{
				op:    "replace",
				path:  "/abc",
				value: 1,
			},
			want:    `[{"op":"replace","path":"/abc","value":1}]`,
			wantErr: false,
		},
		{
			name: "remove object field, redundant args will be ignored",
			args: args{
				op:    "remove",
				from:  "123",
				path:  "/abc",
				value: 1,
			},
			want:    `[{"op":"remove","path":"/abc"}]`,
			wantErr: false,
		},
		{
			name: "move object field",
			args: args{
				op:   "move",
				from: "/abc",
				path: "/123",
			},
			want:    `[{"op":"move","from":"/abc","path":"/123"}]`,
			wantErr: false,
		},
		{
			name: "copy object field, redundant array value will be ignored",
			args: args{
				op:    "copy",
				from:  "/123",
				path:  "/abc",
				value: []interface{}{1, "a", false, 4.5},
			},
			want:    `[{"op":"copy","from":"/123","path":"/abc"}]`,
			wantErr: false,
		},
		{
			name: "replace object field, input string typed number",
			args: args{
				op:    "replace",
				path:  "/abc",
				value: "1",
			},
			want:    `[{"op":"replace","path":"/abc","value":"1"}]`,
			wantErr: false,
		},
		{
			name: "replace object field, input invalid type",
			args: args{
				op:    "replace",
				path:  "/abc",
				value: make(chan int),
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "replace object field, input invalid value",
			args: args{
				op:    "replace",
				path:  "/abc",
				value: math.Inf(1),
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "replace object field, input struct value",
			args: args{
				op:   "replace",
				path: "/abc",
				value: struct {
					A string
					B int
					C float64
					D bool
				}{"a", 1, 1.2, true},
			},
			want:    `[{"op":"replace","path":"/abc","value":{"A":"a","B":1,"C":1.2,"D":true}}]`,
			wantErr: false,
		},
		{
			name: "test object field, input array value",
			args: args{
				op:    "test",
				path:  "/abc",
				value: []interface{}{1, "a", false, 4.5},
			},
			want:    `[{"op":"test","path":"/abc","value":[1,"a",false,4.5]}]`,
			wantErr: false,
		},
		{
			name: "move object field, input invalid path, but we won't verify it",
			args: args{
				op:   "move",
				from: "123",
				path: "abc",
			},
			want:    `[{"op":"move","from":"123","path":"abc"}]`,
			wantErr: false,
		},
		{
			name: "input invalid op",
			args: args{
				op:   "whatever",
				path: "/abc",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patchBytes, err := GenJSONPatch(tt.args.op, tt.args.from, tt.args.path, tt.args.value)
			if tt.wantErr != (err != nil) {
				t.Errorf("wantErr: %v, but got err: %v", tt.wantErr, err)
			}
			if tt.want != string(patchBytes) {
				t.Errorf("want: %s, but got: %s", tt.want, patchBytes)
			}
		})
	}
}
