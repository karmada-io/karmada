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

import "testing"

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

func TestCommandInitIOption_etcdInitContainerCommand(t *testing.T) {
	opt := CommandInitOption{Namespace: "karmada", EtcdReplicas: 1}
	if got := opt.etcdInitContainerCommand(); len(got) == 0 {
		t.Errorf("CommandInitOption.etcdInitContainerCommand() returns empty")
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
