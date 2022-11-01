package util

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNewResource(t *testing.T) {
	tests := []struct {
		name string
		rl   corev1.ResourceList
		want *Resource
	}{
		{
			name: "Resource",
			rl: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(5, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(5, resource.BinarySI),
				corev1.ResourcePods:             *resource.NewQuantity(5, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(5, resource.BinarySI),
				"test.karmada.io/foo":           *resource.NewQuantity(5, resource.BinarySI),
			},
			want: &Resource{
				MilliCPU:         5,
				Memory:           5,
				EphemeralStorage: 5,
				AllowedPodNumber: 5,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 5,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewResource(tt.rl); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_SubResource(t *testing.T) {
	type args struct {
		r  *Resource
		rr *Resource
	}
	tests := []struct {
		name string
		args args
		want *Resource
	}{
		{
			name: "r is nil",
			args: args{
				r: nil,
				rr: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
						"test.karmada.io/bar": 10,
					},
				},
			},
			want: nil,
		},
		{
			name: "rr is nil",
			args: args{
				r: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
						"test.karmada.io/bar": 10,
					},
				},
				rr: nil,
			},
			want: &Resource{
				MilliCPU:         10,
				Memory:           10,
				EphemeralStorage: 10,
				AllowedPodNumber: 10,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 10,
					"test.karmada.io/bar": 10,
				},
			},
		},
		{
			name: "MilliCPU is not enough",
			args: args{
				r: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
						"test.karmada.io/bar": 10,
					},
				},
				rr: &Resource{
					MilliCPU:         99999,
					Memory:           6,
					EphemeralStorage: 6,
					AllowedPodNumber: 6,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 6,
						"test.karmada.io/bar": 6,
					},
				},
			},
			want: &Resource{
				MilliCPU:         0,
				Memory:           4,
				EphemeralStorage: 4,
				AllowedPodNumber: 4,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 4,
					"test.karmada.io/bar": 4,
				},
			},
		},
		{
			name: "Memory is not enough",
			args: args{
				r: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
						"test.karmada.io/bar": 10,
					},
				},
				rr: &Resource{
					MilliCPU:         6,
					Memory:           99999,
					EphemeralStorage: 6,
					AllowedPodNumber: 6,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 6,
						"test.karmada.io/bar": 6,
					},
				},
			},
			want: &Resource{
				MilliCPU:         4,
				Memory:           0,
				EphemeralStorage: 4,
				AllowedPodNumber: 4,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 4,
					"test.karmada.io/bar": 4,
				},
			},
		},
		{
			name: "EphemeralStorage is not enough",
			args: args{
				r: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
						"test.karmada.io/bar": 10,
					},
				},
				rr: &Resource{
					MilliCPU:         6,
					Memory:           6,
					EphemeralStorage: 99999,
					AllowedPodNumber: 6,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 6,
						"test.karmada.io/bar": 6,
					},
				},
			},
			want: &Resource{
				MilliCPU:         4,
				Memory:           4,
				EphemeralStorage: 0,
				AllowedPodNumber: 4,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 4,
					"test.karmada.io/bar": 4,
				},
			},
		},
		{
			name: "AllowedPodNumber is not enough",
			args: args{
				r: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
						"test.karmada.io/bar": 10,
					},
				},
				rr: &Resource{
					MilliCPU:         6,
					Memory:           6,
					EphemeralStorage: 6,
					AllowedPodNumber: 99999,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 6,
						"test.karmada.io/bar": 6,
					},
				},
			},
			want: &Resource{
				MilliCPU:         4,
				Memory:           4,
				EphemeralStorage: 4,
				AllowedPodNumber: 0,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 4,
					"test.karmada.io/bar": 4,
				},
			},
		},
		{
			name: "ScalarResources foo is not enough",
			args: args{
				r: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
						"test.karmada.io/bar": 10,
					},
				},
				rr: &Resource{
					MilliCPU:         6,
					Memory:           6,
					EphemeralStorage: 6,
					AllowedPodNumber: 6,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 9999,
						"test.karmada.io/bar": 6,
					},
				},
			},
			want: &Resource{
				MilliCPU:         4,
				Memory:           4,
				EphemeralStorage: 4,
				AllowedPodNumber: 4,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 0,
					"test.karmada.io/bar": 4,
				},
			},
		},
		{
			name: "ScalarResources bar is not enough",
			args: args{
				r: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
						"test.karmada.io/bar": 10,
					},
				},
				rr: &Resource{
					MilliCPU:         6,
					Memory:           6,
					EphemeralStorage: 6,
					AllowedPodNumber: 6,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 6,
						"test.karmada.io/bar": 9999,
					},
				},
			},
			want: &Resource{
				MilliCPU:         4,
				Memory:           4,
				EphemeralStorage: 4,
				AllowedPodNumber: 4,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 4,
					"test.karmada.io/bar": 0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.args.r.SubResource(tt.args.rr); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want = %v", tt.args.r, tt.want)
			}
		})
	}
}

func TestResource_SetMaxResource(t *testing.T) {
	type args struct {
		r  *Resource
		rl corev1.ResourceList
	}
	tests := []struct {
		name string
		args args
		want *Resource
	}{
		{
			name: "r is nil",
			args: args{
				r: nil,
				rl: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:              *resource.NewMilliQuantity(5, resource.DecimalSI),
					corev1.ResourceMemory:           *resource.NewQuantity(5, resource.BinarySI),
					corev1.ResourcePods:             *resource.NewQuantity(5, resource.DecimalSI),
					corev1.ResourceEphemeralStorage: *resource.NewQuantity(5, resource.BinarySI),
					"test.karmada.io/foo":           *resource.NewQuantity(5, resource.BinarySI),
				},
			},
			want: nil,
		},
		{
			name: "r is max",
			args: args{
				r: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
					},
				},
				rl: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:              *resource.NewMilliQuantity(5, resource.DecimalSI),
					corev1.ResourceMemory:           *resource.NewQuantity(5, resource.BinarySI),
					corev1.ResourcePods:             *resource.NewQuantity(5, resource.DecimalSI),
					corev1.ResourceEphemeralStorage: *resource.NewQuantity(5, resource.BinarySI),
					"test.karmada.io/foo":           *resource.NewQuantity(5, resource.BinarySI),
				},
			},
			want: &Resource{
				MilliCPU:         10,
				Memory:           10,
				EphemeralStorage: 10,
				AllowedPodNumber: 10,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 10,
				},
			},
		},
		{
			name: "rl is max",
			args: args{
				r: &Resource{
					MilliCPU:         10,
					Memory:           10,
					EphemeralStorage: 10,
					AllowedPodNumber: 10,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 10,
					},
				},
				rl: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:              *resource.NewMilliQuantity(20, resource.DecimalSI),
					corev1.ResourceMemory:           *resource.NewQuantity(20, resource.BinarySI),
					corev1.ResourcePods:             *resource.NewQuantity(20, resource.DecimalSI),
					corev1.ResourceEphemeralStorage: *resource.NewQuantity(20, resource.BinarySI),
					"test.karmada.io/foo":           *resource.NewQuantity(20, resource.BinarySI),
				},
			},
			want: &Resource{
				MilliCPU:         20,
				Memory:           20,
				EphemeralStorage: 20,
				AllowedPodNumber: 20,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 20,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.r.SetMaxResource(tt.args.rl)
			if !reflect.DeepEqual(tt.args.r, tt.want) {
				t.Errorf("got %v, want %v", tt.args.r, tt.want)
			}
		})
	}
}

func TestResource_ResourceList(t *testing.T) {
	tests := []struct {
		name     string
		resource *Resource
		want     corev1.ResourceList
	}{
		{
			name:     "resource is empty",
			resource: EmptyResource(),
			want:     nil,
		},
		{
			name: "resource list",
			resource: &Resource{
				MilliCPU:         10,
				Memory:           10,
				EphemeralStorage: 10,
				AllowedPodNumber: 10,
				ScalarResources: map[corev1.ResourceName]int64{
					corev1.ResourceHugePagesPrefix + "foo": 10,
					"test.karmada.io/bar":                  10,
				},
			},
			want: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceCPU:                     *resource.NewMilliQuantity(10, resource.DecimalSI),
				corev1.ResourceMemory:                  *resource.NewQuantity(10, resource.BinarySI),
				corev1.ResourcePods:                    *resource.NewQuantity(10, resource.DecimalSI),
				corev1.ResourceEphemeralStorage:        *resource.NewQuantity(10, resource.BinarySI),
				corev1.ResourceHugePagesPrefix + "foo": *resource.NewQuantity(10, resource.BinarySI),
				"test.karmada.io/bar":                  *resource.NewQuantity(10, resource.DecimalSI),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resource.ResourceList(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResourceList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_MaxDivided(t *testing.T) {
	type args struct {
		r  *Resource
		rl corev1.ResourceList
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "MaxDivided",
			args: args{
				r: &Resource{
					MilliCPU:         100,
					Memory:           100,
					EphemeralStorage: 100,
					AllowedPodNumber: 100,
					ScalarResources: map[corev1.ResourceName]int64{
						"test.karmada.io/foo": 100,
					},
				},
				rl: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceCPU:              *resource.NewMilliQuantity(10, resource.DecimalSI), // 10
					corev1.ResourceMemory:           *resource.NewQuantity(5, resource.BinarySI),        // 20
					corev1.ResourcePods:             *resource.NewQuantity(20, resource.DecimalSI),      //  4
					corev1.ResourceEphemeralStorage: *resource.NewQuantity(15, resource.BinarySI),       // 6
					"test.karmada.io/foo":           *resource.NewQuantity(18, resource.DecimalSI),      // 5
				},
			},
			want: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.args.r.MaxDivided(tt.args.rl); got != tt.want {
				t.Errorf("MaxDivided() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_AddPodTemplateRequest(t *testing.T) {
	type args struct {
		podSpec *corev1.PodSpec
	}
	tests := []struct {
		name string
		args args
		want *Resource
	}{
		{
			name: "",
			args: args{
				podSpec: &corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(10, resource.BinarySI),
									"test.karmada.io/foo": *resource.NewQuantity(10, resource.DecimalSI),
									"test.karmada.io/bar": *resource.NewQuantity(10, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: *resource.NewQuantity(99999, resource.BinarySI),
									corev1.ResourcePods:   *resource.NewQuantity(10, resource.DecimalSI),
									"test.karmada.io/bar": *resource.NewQuantity(99999, resource.DecimalSI),
									"test.karmada.io/baz": *resource.NewQuantity(10, resource.DecimalSI),
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    *resource.NewMilliQuantity(5, resource.DecimalSI),
									corev1.ResourceMemory: *resource.NewQuantity(10, resource.BinarySI),
									"test.karmada.io/foo": *resource.NewQuantity(5, resource.DecimalSI),
									"test.karmada.io/bar": *resource.NewQuantity(10, resource.DecimalSI),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: *resource.NewQuantity(99999, resource.BinarySI),
									corev1.ResourcePods:   *resource.NewQuantity(20, resource.DecimalSI),
									"test.karmada.io/bar": *resource.NewQuantity(99999, resource.DecimalSI),
									"test.karmada.io/baz": *resource.NewQuantity(20, resource.DecimalSI),
								},
							},
						},
					},
					Overhead: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:              *resource.NewMilliQuantity(5, resource.DecimalSI),
						corev1.ResourceMemory:           *resource.NewQuantity(5, resource.BinarySI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(5, resource.BinarySI),
						corev1.ResourcePods:             *resource.NewQuantity(5, resource.DecimalSI),
						"test.karmada.io/foo":           *resource.NewQuantity(5, resource.DecimalSI),
						"test.karmada.io/bar":           *resource.NewQuantity(5, resource.DecimalSI),
						"test.karmada.io/baz":           *resource.NewQuantity(5, resource.DecimalSI),
					},
				},
			},
			want: &Resource{
				MilliCPU:         15,
				Memory:           15,
				EphemeralStorage: 5,
				AllowedPodNumber: 25,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 15,
					"test.karmada.io/bar": 15,
					"test.karmada.io/baz": 25,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := EmptyResource()
			if got := r.AddPodTemplateRequest(tt.args.podSpec); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddPodTemplateRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_Clone(t *testing.T) {
	tests := []struct {
		name   string
		caller *Resource
		want   *Resource
	}{
		{
			name:   "caller is nil",
			caller: nil,
			want:   nil,
		},
		{
			name: "clone",
			caller: &Resource{
				MilliCPU:         1,
				Memory:           2,
				EphemeralStorage: 3,
				AllowedPodNumber: 4,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 5,
					"test.karmada.io/bar": 6,
					"test.karmada.io/baz": 7,
				},
			},
			want: &Resource{
				MilliCPU:         1,
				Memory:           2,
				EphemeralStorage: 3,
				AllowedPodNumber: 4,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/foo": 5,
					"test.karmada.io/bar": 6,
					"test.karmada.io/baz": 7,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.caller.Clone(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Clone() = %v, want %v", got, tt.want)
			}
		})
	}
}
