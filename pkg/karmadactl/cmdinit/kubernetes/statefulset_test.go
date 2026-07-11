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

package kubernetes

import (
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestCommandInitIOption_etcdVolume(t *testing.T) {
	tests := []struct {
		name       string
		opt        CommandInitOption
		claimIsNil bool
		errorMsg   string
	}{
		{
			name: "EtcdStorageMode is etcdStorageModePVC",
			opt: CommandInitOption{
				EtcdStorageMode:          etcdStorageModePVC,
				Namespace:                "karmada",
				StorageClassesName:       "StorageClassesName",
				EtcdPersistentVolumeSize: "1024",
			},
			claimIsNil: false,
			errorMsg:   "CommandInitOption.etcdVolume() returns persistentVolumeClaim nil",
		},
		{
			name: "EtcdStorageMode is etcdStorageModeHostPath",
			opt: CommandInitOption{
				EtcdStorageMode:    etcdStorageModeHostPath,
				Namespace:          "karmada",
				StorageClassesName: "StorageClassesName",
				EtcdHostDataPath:   "/data",
			},
			claimIsNil: true,
			errorMsg:   "CommandInitOption.etcdVolume() returns persistentVolumeClaim not nil",
		},
		{
			name: "EtcdStorageMode is etcdStorageModeEmptyDir",
			opt: CommandInitOption{
				EtcdStorageMode:    etcdStorageModeEmptyDir,
				Namespace:          "karmada",
				StorageClassesName: "StorageClassesName",
			},
			claimIsNil: true,
			errorMsg:   "CommandInitOption.etcdVolume() returns persistentVolumeClaim not nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := tt.opt.etcdVolume()
			if (got == nil) != tt.claimIsNil {
				t.Error(tt.errorMsg)
			}
		})
	}
}

func TestCommandInitIOption_makeETCDStatefulSet_HostPathTolerations(t *testing.T) {
	opt := &CommandInitOption{
		EtcdStorageMode: etcdStorageModeHostPath,
		Namespace:       "karmada",
	}
	etcd := opt.makeETCDStatefulSet()

	if len(etcd.Spec.Template.Spec.Tolerations) != len(defaultEtcdTolerations) {
		t.Fatalf("CommandInitOption.makeETCDStatefulSet() tolerations = %d, want %d", len(etcd.Spec.Template.Spec.Tolerations), len(defaultEtcdTolerations))
	}

	for _, expected := range defaultEtcdTolerations {
		if !slices.Contains(etcd.Spec.Template.Spec.Tolerations, expected) {
			t.Fatalf("CommandInitOption.makeETCDStatefulSet() missing toleration %+v", expected)
		}
	}

	pvcOpt := &CommandInitOption{
		EtcdStorageMode:          etcdStorageModePVC,
		Namespace:                "karmada",
		EtcdPersistentVolumeSize: "1Gi",
	}
	if pvc := pvcOpt.makeETCDStatefulSet(); len(pvc.Spec.Template.Spec.Tolerations) != 0 {
		t.Fatalf("CommandInitOption.makeETCDStatefulSet() PVC tolerations = %v, want none", pvc.Spec.Template.Spec.Tolerations)
	}
}

func TestTolerationsTolerateTaints(t *testing.T) {
	tests := []struct {
		name        string
		taints      []corev1.Taint
		tolerations []corev1.Toleration
		want        bool
	}{
		{
			name:        "control-plane taints are tolerated",
			taints:      []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}},
			tolerations: defaultEtcdTolerations,
			want:        true,
		},
		{
			name:        "mixed custom taints are not tolerated",
			taints:      []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}, {Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}},
			tolerations: defaultEtcdTolerations,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tolerationsTolerateTaints(tt.tolerations, tt.taints); got != tt.want {
				t.Fatalf("tolerationsTolerateTaints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandInitIOption_makeETCDStatefulSet(t *testing.T) {
	tests := []struct {
		name          string
		opt           CommandInitOption
		expectedNSMap map[string]string
	}{
		{
			name: "EtcdStorageMode is etcdStorageModeHostPath, single valid EtcdNodeSelectorLabel",
			opt: CommandInitOption{
				EtcdStorageMode:          etcdStorageModeHostPath,
				Namespace:                "karmada",
				StorageClassesName:       "StorageClassesName",
				EtcdPersistentVolumeSize: "1024",
				EtcdNodeSelectorLabels:   "label=value",
				EtcdNodeSelectorLabelsMap: map[string]string{
					"label": "value",
				},
			},
			expectedNSMap: map[string]string{
				"label": "value",
			},
		},
		{
			name: "EtcdStorageMode is etcdStorageModeHostPath, multiple valid EtcdNodeSelectorLabels",
			opt: CommandInitOption{
				EtcdStorageMode:          etcdStorageModeHostPath,
				Namespace:                "karmada",
				StorageClassesName:       "StorageClassesName",
				EtcdPersistentVolumeSize: "1024",
				EtcdNodeSelectorLabels:   "label1=value1,label2=value2,kubernetes.io/os=linux",
				EtcdNodeSelectorLabelsMap: map[string]string{
					"label1":           "value1",
					"label2":           "value2",
					"kubernetes.io/os": "linux",
				},
			},
			expectedNSMap: map[string]string{
				"label1":           "value1",
				"label2":           "value2",
				"kubernetes.io/os": "linux",
			},
		},
		{
			name: "EtcdStorageMode is etcdStorageModePVC",
			opt: CommandInitOption{
				EtcdStorageMode:           etcdStorageModePVC,
				Namespace:                 "karmada",
				StorageClassesName:        "StorageClassesName",
				EtcdPersistentVolumeSize:  "1024",
				EtcdNodeSelectorLabels:    "",
				EtcdNodeSelectorLabelsMap: map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			etcd := tt.opt.makeETCDStatefulSet()
			if tt.opt.EtcdStorageMode == etcdStorageModePVC {
				if len(etcd.Spec.VolumeClaimTemplates) == 0 {
					t.Errorf("CommandInitOption.makeETCDStatefulSet() returns empty VolumeClaimTemplates")
				}
			} else {
				nodeSelector := etcd.Spec.Template.Spec.NodeSelector
				for label, value := range tt.expectedNSMap {
					if val, ok := nodeSelector[label]; !ok || val != value {
						t.Errorf("CommandInitOption.makeETCDStatefulSet() returns wrong nodeSelector %v, expected %v=%v", nodeSelector, label, value)
					}
				}

				if len(etcd.Spec.VolumeClaimTemplates) != 0 {
					t.Errorf("CommandInitOption.makeETCDStatefulSet() returns non-empty VolumeClaimTemplates")
				}
			}
		})
	}
}
