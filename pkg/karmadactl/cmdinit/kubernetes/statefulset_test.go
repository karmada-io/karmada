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
				t.Errorf(tt.errorMsg)
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
		name            string
		opt             CommandInitOption
		expectedNSValue string
		expectedNSLabel string
	}{
		{
			name: "EtcdStorageMode is etcdStorageModeHostPath, EtcdNodeSelectorLabels is set",
			opt: CommandInitOption{
				EtcdStorageMode:          etcdStorageModeHostPath,
				Namespace:                "karmada",
				StorageClassesName:       "StorageClassesName",
				EtcdPersistentVolumeSize: "1024",
				EtcdNodeSelectorLabels:   "label=value",
			},
			expectedNSValue: "value",
			expectedNSLabel: "label",
		},
		{
			name: "EtcdStorageMode is etcdStorageModeHostPath, EtcdNodeSelectorLabels is not set",
			opt: CommandInitOption{
				EtcdStorageMode:          etcdStorageModeHostPath,
				Namespace:                "karmada",
				StorageClassesName:       "StorageClassesName",
				EtcdPersistentVolumeSize: "1024",
				EtcdNodeSelectorLabels:   "",
			},
			expectedNSValue: "",
			expectedNSLabel: "karmada.io/etcd",
		},
		{
			name: "EtcdStorageMode is etcdStorageModePVC",
			opt: CommandInitOption{
				EtcdStorageMode:          etcdStorageModePVC,
				Namespace:                "karmada",
				StorageClassesName:       "StorageClassesName",
				EtcdPersistentVolumeSize: "1024",
				EtcdNodeSelectorLabels:   "",
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
				if val, ok := nodeSelector[tt.expectedNSLabel]; !ok || val != tt.expectedNSValue {
					t.Errorf("CommandInitOption.makeETCDStatefulSet() returns wrong nodeSelector %v", nodeSelector)
				}

				if len(etcd.Spec.VolumeClaimTemplates) != 0 {
					t.Errorf("CommandInitOption.makeETCDStatefulSet() returns non-empty VolumeClaimTemplates")
				}
			}
		})
	}
}
