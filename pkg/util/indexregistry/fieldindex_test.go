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

package indexregistry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGenLabelIndexerFunc(t *testing.T) {
	type args struct {
		key string
		obj client.Object
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "cache hit",
			args: args{
				key: "a",
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"a": "a",
						},
					},
				},
			},
			want: []string{"a"},
		},
		{
			name: "cache missed",
			args: args{
				key: "b",
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"a": "a",
						},
					},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := GenLabelIndexerFunc(tt.args.key)
			if !assert.NotNil(t, fn) {
				t.FailNow()
			}
			assert.Equalf(t, tt.want, fn(tt.args.obj), "GenLabelIndexerFunc(%v)", tt.args.key)
		})
	}
}
