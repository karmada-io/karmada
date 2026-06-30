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

func TestEmptyResource(t *testing.T) {
	got := EmptyResource()
	want := &Resource{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EmptyResource() = %v, want %v", got, want)
	}
}

func TestResource_Add(t *testing.T) {
	tests := []struct {
		name string
		r    *Resource
		rl   corev1.ResourceList
		want *Resource
	}{
		{
			name: "r is nil",
			r:    nil,
			rl: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(10, resource.BinarySI),
			},
			want: nil,
		},
		{
			name: "add standard resources",
			r:    EmptyResource(),
			rl: corev1.ResourceList{
				corev1.ResourceCPU:              *resource.NewMilliQuantity(10, resource.DecimalSI),
				corev1.ResourceMemory:           *resource.NewQuantity(20, resource.BinarySI),
				corev1.ResourcePods:             *resource.NewQuantity(5, resource.DecimalSI),
				corev1.ResourceEphemeralStorage: *resource.NewQuantity(30, resource.BinarySI),
			},
			want: &Resource{
				MilliCPU:         10,
				Memory:           20,
				AllowedPodNumber: 5,
				EphemeralStorage: 30,
			},
		},
		{
			name: "add scalar resources",
			r:    EmptyResource(),
			rl: corev1.ResourceList{
				"test.karmada.io/gpu": *resource.NewQuantity(4, resource.DecimalSI),
			},
			want: &Resource{
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 4,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.r.Add(tt.rl)
			if !reflect.DeepEqual(tt.r, tt.want) {
				t.Errorf("Add() result = %v, want %v", tt.r, tt.want)
			}
		})
	}
}

func TestResource_Multiply(t *testing.T) {
	tests := []struct {
		name   string
		r      *Resource
		factor int64
		want   *Resource
	}{
		{
			name:   "r is nil",
			r:      nil,
			factor: 3,
			want:   nil,
		},
		{
			name: "multiply by factor",
			r: &Resource{
				MilliCPU:         10,
				Memory:           20,
				EphemeralStorage: 30,
				AllowedPodNumber: 5,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 2,
				},
			},
			factor: 3,
			want: &Resource{
				MilliCPU:         30,
				Memory:           60,
				EphemeralStorage: 90,
				AllowedPodNumber: 15,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 6,
				},
			},
		},
		{
			name: "multiply by zero",
			r: &Resource{
				MilliCPU:         10,
				Memory:           20,
				EphemeralStorage: 30,
				AllowedPodNumber: 5,
			},
			factor: 0,
			want: &Resource{
				MilliCPU:         0,
				Memory:           0,
				EphemeralStorage: 0,
				AllowedPodNumber: 0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.Multiply(tt.factor)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Multiply() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_Allocatable(t *testing.T) {
	tests := []struct {
		name string
		r    *Resource
		rr   *Resource
		want bool
	}{
		{
			name: "rr is nil",
			r: &Resource{
				MilliCPU: 10,
				Memory:   10,
			},
			rr:   nil,
			want: true,
		},
		{
			name: "r is nil",
			r:    nil,
			rr: &Resource{
				MilliCPU: 10,
				Memory:   10,
			},
			want: false,
		},
		{
			name: "r has sufficient resources",
			r: &Resource{
				MilliCPU:         100,
				Memory:           100,
				EphemeralStorage: 100,
				AllowedPodNumber: 10,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 8,
				},
			},
			rr: &Resource{
				MilliCPU:         50,
				Memory:           50,
				EphemeralStorage: 50,
				AllowedPodNumber: 5,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 4,
				},
			},
			want: true,
		},
		{
			name: "insufficient MilliCPU",
			r: &Resource{
				MilliCPU: 10,
				Memory:   100,
			},
			rr: &Resource{
				MilliCPU: 50,
				Memory:   50,
			},
			want: false,
		},
		{
			name: "insufficient Memory",
			r: &Resource{
				MilliCPU: 100,
				Memory:   10,
			},
			rr: &Resource{
				MilliCPU: 50,
				Memory:   50,
			},
			want: false,
		},
		{
			name: "insufficient EphemeralStorage",
			r: &Resource{
				MilliCPU:         100,
				Memory:           100,
				EphemeralStorage: 10,
			},
			rr: &Resource{
				MilliCPU:         50,
				Memory:           50,
				EphemeralStorage: 50,
			},
			want: false,
		},
		{
			name: "insufficient AllowedPodNumber",
			r: &Resource{
				MilliCPU:         100,
				Memory:           100,
				AllowedPodNumber: 2,
			},
			rr: &Resource{
				MilliCPU:         50,
				Memory:           50,
				AllowedPodNumber: 5,
			},
			want: false,
		},
		{
			name: "insufficient scalar resource",
			r: &Resource{
				MilliCPU: 100,
				Memory:   100,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 2,
				},
			},
			rr: &Resource{
				MilliCPU: 50,
				Memory:   50,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 4,
				},
			},
			want: false,
		},
		{
			name: "missing scalar resource in r",
			r: &Resource{
				MilliCPU: 100,
				Memory:   100,
			},
			rr: &Resource{
				MilliCPU: 50,
				Memory:   50,
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 4,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Allocatable(tt.rr); got != tt.want {
				t.Errorf("Allocatable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_SetScalar(t *testing.T) {
	tests := []struct {
		name     string
		r        *Resource
		rName    corev1.ResourceName
		quantity int64
		want     *Resource
	}{
		{
			name:     "lazy init scalar resources map",
			r:        EmptyResource(),
			rName:    "test.karmada.io/gpu",
			quantity: 4,
			want: &Resource{
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 4,
				},
			},
		},
		{
			name: "overwrite existing scalar",
			r: &Resource{
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 2,
				},
			},
			rName:    "test.karmada.io/gpu",
			quantity: 8,
			want: &Resource{
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 8,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.r.SetScalar(tt.rName, tt.quantity)
			if !reflect.DeepEqual(tt.r, tt.want) {
				t.Errorf("SetScalar() result = %v, want %v", tt.r, tt.want)
			}
		})
	}
}

func TestResource_AddScalar(t *testing.T) {
	tests := []struct {
		name     string
		r        *Resource
		rName    corev1.ResourceName
		quantity int64
		want     *Resource
	}{
		{
			name:     "add scalar to empty resource",
			r:        EmptyResource(),
			rName:    "test.karmada.io/gpu",
			quantity: 4,
			want: &Resource{
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 4,
				},
			},
		},
		{
			name: "accumulate scalar resource",
			r: &Resource{
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 2,
				},
			},
			rName:    "test.karmada.io/gpu",
			quantity: 3,
			want: &Resource{
				ScalarResources: map[corev1.ResourceName]int64{
					"test.karmada.io/gpu": 5,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.r.AddScalar(tt.rName, tt.quantity)
			if !reflect.DeepEqual(tt.r, tt.want) {
				t.Errorf("AddScalar() result = %v, want %v", tt.r, tt.want)
			}
		})
	}
}

func TestResource_AddPodRequest(t *testing.T) {
	tests := []struct {
		name    string
		podSpec *corev1.PodSpec
		want    *Resource
	}{
		{
			name: "containers only",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(10, resource.BinarySI),
							},
						},
					},
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewMilliQuantity(5, resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(5, resource.BinarySI),
							},
						},
					},
				},
			},
			want: &Resource{
				MilliCPU: 15,
				Memory:   15,
			},
		},
		{
			name: "init container request exceeds container request",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(10, resource.BinarySI),
							},
						},
					},
				},
				InitContainers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewMilliQuantity(20, resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(5, resource.BinarySI),
							},
						},
					},
				},
			},
			want: &Resource{
				MilliCPU: 20,
				Memory:   10,
			},
		},
		{
			name: "with overhead",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
								corev1.ResourceMemory: *resource.NewQuantity(10, resource.BinarySI),
							},
						},
					},
				},
				Overhead: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewMilliQuantity(5, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(5, resource.BinarySI),
				},
			},
			want: &Resource{
				MilliCPU: 15,
				Memory:   15,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := EmptyResource()
			got := r.AddPodRequest(tt.podSpec)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddPodRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_AddResourcePods(t *testing.T) {
	tests := []struct {
		name string
		pods int64
		want *Resource
	}{
		{
			name: "add pods",
			pods: 5,
			want: &Resource{
				AllowedPodNumber: 5,
			},
		},
		{
			name: "add zero pods",
			pods: 0,
			want: &Resource{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := EmptyResource()
			r.AddResourcePods(tt.pods)
			if !reflect.DeepEqual(r, tt.want) {
				t.Errorf("AddResourcePods() result = %v, want %v", r, tt.want)
			}
		})
	}
}

func TestMinInt64(t *testing.T) {
	tests := []struct {
		name string
		a, b int64
		want int64
	}{
		{
			name: "a is smaller",
			a:    1,
			b:    2,
			want: 1,
		},
		{
			name: "b is smaller",
			a:    5,
			b:    3,
			want: 3,
		},
		{
			name: "equal",
			a:    4,
			b:    4,
			want: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MinInt64(tt.a, tt.b); got != tt.want {
				t.Errorf("MinInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaxInt64(t *testing.T) {
	tests := []struct {
		name string
		a, b int64
		want int64
	}{
		{
			name: "a is larger",
			a:    5,
			b:    2,
			want: 5,
		},
		{
			name: "b is larger",
			a:    1,
			b:    3,
			want: 3,
		},
		{
			name: "equal",
			a:    4,
			b:    4,
			want: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaxInt64(tt.a, tt.b); got != tt.want {
				t.Errorf("MaxInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}
