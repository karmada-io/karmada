package kubernetes

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

func Test_initializeDirectory(t *testing.T) {
	tests := []struct {
		name                string
		createPathInAdvance bool
		wantErr             bool
	}{
		{
			name:                "Test when there is no dir exists",
			createPathInAdvance: false,
			wantErr:             false,
		},
		{
			name:                "Test when there is dir exists",
			createPathInAdvance: true,
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createPathInAdvance {
				if err := os.MkdirAll("tmp", os.FileMode(0700)); err != nil {
					t.Errorf("create test directory failed in advance:%v", err)
				}
			}
			if err := initializeDirectory("tmp"); (err != nil) != tt.wantErr {
				t.Errorf("initializeDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := os.RemoveAll("tmp"); err != nil {
				t.Errorf("clean up test directory failed after ut case:%s, %v", tt.name, err)
			}
		})
	}
}

func TestCommandInitOption_Validate(t *testing.T) {
	tests := []struct {
		name     string
		opt      CommandInitOption
		wantErr  bool
		errorMsg string
	}{
		{
			name: "Invalid KarmadaAPIServerAdvertiseAddress",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "111",
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when KarmadaAPIServerAdvertiseAddress is wrong",
		},
		{
			name: "Empty EtcdHostDataPath",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModeHostPath,
				EtcdHostDataPath:                 "",
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when EtcdHostDataPath is empty",
		},
		{
			name: "Invalid EtcdNodeSelectorLabels",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModeHostPath,
				EtcdHostDataPath:                 "/data",
				EtcdNodeSelectorLabels:           "key",
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when EtcdNodeSelectorLabels is %v",
		},
		{
			name: "Invalid EtcdReplicas",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModeHostPath,
				EtcdHostDataPath:                 "/data",
				EtcdNodeSelectorLabels:           "key=value",
				EtcdReplicas:                     2,
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when EtcdReplicas is %v",
		},
		{
			name: "Empty StorageClassesName",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModePVC,
				EtcdHostDataPath:                 "/data",
				EtcdNodeSelectorLabels:           "key=value",
				EtcdReplicas:                     1,
				StorageClassesName:               "",
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when StorageClassesName is empty",
		},
		{
			name: "Invalid EtcdStorageMode",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  "unknown",
			},
			wantErr:  true,
			errorMsg: "CommandInitOption.Validate() does not return err when EtcdStorageMode is unknown",
		},
		{
			name: "Valid EtcdStorageMode",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  etcdStorageModeEmptyDir,
			},
			wantErr:  false,
			errorMsg: "CommandInitOption.Validate() returns err when EtcdStorageMode is emptyDir",
		},
		{
			name: "Valid EtcdStorageMode",
			opt: CommandInitOption{
				KarmadaAPIServerAdvertiseAddress: "192.0.2.1",
				EtcdStorageMode:                  "",
			},
			wantErr:  false,
			errorMsg: "CommandInitOption.Validate() returns err when EtcdStorageMode is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.opt.Validate("parentCommand"); (err != nil) != tt.wantErr {
				t.Errorf(tt.errorMsg)
			}
		})
	}
}

func TestCommandInitOption_genCerts(t *testing.T) {
	tmpPath := "/tmp/pki"
	defer os.RemoveAll(tmpPath)
	opt := &CommandInitOption{
		CertValidity:       365 * 24 * time.Hour,
		EtcdReplicas:       3,
		Namespace:          "default",
		ExternalDNS:        "example.com",
		ExternalIP:         "1.2.3.4",
		KarmadaAPIServerIP: []net.IP{utils.StringToNetIP("1.2.3.5")},
		KarmadaPkiPath:     tmpPath,
	}
	// Call the function to generate the certificates
	err := opt.genCerts()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestInitKarmadaAPIServer(t *testing.T) {
	// Create a fake clientset
	clientset := fake.NewSimpleClientset()

	// Create a new CommandInitOption
	initOption := &CommandInitOption{
		KubeClientSet: clientset,
		Namespace:     "test-namespace",
	}

	// Call the function
	err := initOption.initKarmadaAPIServer()

	// Check if the function returned an error
	if err != nil {
		t.Errorf("initKarmadaAPIServer() returned an error: %v", err)
	}

	// Check if the etcd StatefulSet was created
	_, err = clientset.AppsV1().StatefulSets(initOption.Namespace).Get(context.TODO(), etcdStatefulSetAndServiceName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("initKarmadaAPIServer() failed to create etcd StatefulSet: %v", err)
	}

	// Check if the karmada APIServer Deployment was created
	_, err = clientset.AppsV1().Deployments(initOption.Namespace).Get(context.TODO(), karmadaAPIServerDeploymentAndServiceName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("initKarmadaAPIServer() failed to create karmada APIServer Deployment: %v", err)
	}

	// Check if the karmada aggregated apiserver Deployment was created
	_, err = clientset.AppsV1().Deployments(initOption.Namespace).Get(context.TODO(), karmadaAggregatedAPIServerDeploymentAndServiceName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("initKarmadaAPIServer() failed to create karmada aggregated apiserver Deployment: %v", err)
	}
}

func TestCommandInitOption_initKarmadaComponent(t *testing.T) {
	// Create a fake kube clientset
	clientSet := fake.NewSimpleClientset()

	// Create a new CommandInitOption
	initOption := &CommandInitOption{
		KubeClientSet: clientSet,
		Namespace:     "test-namespace",
	}

	// Call the function
	err := initOption.initKarmadaComponent()

	// Assert that no error was returned
	if err != nil {
		t.Errorf("initKarmadaComponent returned an unexpected error: %v", err)
	}

	// Assert that the expected deployments were created
	expectedDeployments := []string{
		kubeControllerManagerClusterRoleAndDeploymentAndServiceName,
		schedulerDeploymentNameAndServiceAccountName,
		controllerManagerDeploymentAndServiceName,
		webhookDeploymentAndServiceAccountAndServiceName,
	}
	for _, deploymentName := range expectedDeployments {
		_, err = clientSet.AppsV1().Deployments("test-namespace").Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			t.Errorf("Expected deployment %s was not created: %v", deploymentName, err)
		}
	}
}

func TestKubeRegistry(t *testing.T) {
	tests := []struct {
		name                 string
		opt                  *CommandInitOption
		expectedKubeRegistry string
	}{
		{
			name: "KubeImageRegistry is set",
			opt: &CommandInitOption{
				KubeImageRegistry: "my-registry",
			},
			expectedKubeRegistry: "my-registry",
		},
		{
			name: "KubeImageMirrorCountry is set to a supported value",
			opt: &CommandInitOption{
				KubeImageMirrorCountry: "CN",
			},
			expectedKubeRegistry: imageRepositories["cn"],
		},
		{
			name: "KubeImageMirrorCountry is set to an unsupported value",
			opt: &CommandInitOption{
				KubeImageMirrorCountry: "unsupported",
			},
			expectedKubeRegistry: imageRepositories["global"],
		},
		{
			name: "KubeImageRegistry and KubeImageMirrorCountry are not set, but ImageRegistry is set",
			opt: &CommandInitOption{
				ImageRegistry: "my-registry",
			},
			expectedKubeRegistry: "my-registry",
		},
		{
			name:                 "Neither KubeImageRegistry nor ImageRegistry are set, and KubeImageMirrorCountry is not set",
			opt:                  &CommandInitOption{},
			expectedKubeRegistry: imageRepositories["global"],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.kubeRegistry()
			if result != tt.expectedKubeRegistry {
				t.Errorf("Unexpected result: %s", result)
			}
		})
	}
}

func TestKubeAPIServerImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "KarmadaAPIServerImage is set",
			opt: &CommandInitOption{
				KarmadaAPIServerImage: "my-karmada-image",
			},
			expected: "my-karmada-image",
		},
		{
			name: "KarmadaAPIServerImage is not set, should return the expected value based on kubeRegistry() and KubeImageTag",
			opt: &CommandInitOption{
				KubeImageTag: "1.20.1",
			},
			expected: imageRepositories["global"] + "/kube-apiserver:1.20.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.kubeAPIServerImage()
			if result != tt.expected {
				t.Errorf("Unexpected result: %s, expected: %s", result, tt.expected)
			}
		})
	}
}

func TestKubeControllerManagerImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "KubeControllerManagerImage is set",
			opt: &CommandInitOption{
				KubeControllerManagerImage: "my-controller-manager-image",
			},
			expected: "my-controller-manager-image",
		},
		{
			name: "KubeControllerManagerImage is not set",
			opt: &CommandInitOption{
				KubeImageTag: "1.20.1",
			},
			expected: imageRepositories["global"] + "/kube-controller-manager:1.20.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.opt.kubeControllerManagerImage()
			if got != tt.expected {
				t.Errorf("CommandInitOption.kubeControllerManagerImage() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEtcdInitImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "ImageRegistry is set and EtcdInitImage is set to default value",
			opt: &CommandInitOption{
				ImageRegistry: "my-registry",
				EtcdInitImage: DefaultInitImage,
			},
			expected: "my-registry/alpine:3.15.1",
		},
		{
			name: "EtcdInitImage is set to a non-default value",
			opt: &CommandInitOption{
				EtcdInitImage: "my-etcd-init-image",
			},
			expected: "my-etcd-init-image",
		},
		{
			name: "ImageRegistry is not set and EtcdInitImage is set to default value",
			opt: &CommandInitOption{
				EtcdInitImage: DefaultInitImage,
			},
			expected: DefaultInitImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.etcdInitImage()
			if result != tt.expected {
				t.Errorf("Unexpected result: %s, expected: %s", result, tt.expected)
			}
		})
	}
}

func TestEtcdImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "EtcdImage is set",
			opt: &CommandInitOption{
				EtcdImage: "my-etcd-image",
			},
			expected: "my-etcd-image",
		},
		{
			name:     "EtcdImage is not set",
			opt:      &CommandInitOption{},
			expected: imageRepositories["global"] + "/" + defaultEtcdImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.etcdImage()
			if result != tt.expected {
				t.Errorf("Unexpected result: %s, expected: %s", result, tt.expected)
			}
		})
	}
}

func TestKarmadaSchedulerImage(t *testing.T) {
	tests := []struct {
		name     string
		opt      *CommandInitOption
		expected string
	}{
		{
			name: "ImageRegistry is set and KarmadaSchedulerImage is set to default value",
			opt: &CommandInitOption{
				ImageRegistry:         "my-registry",
				KarmadaSchedulerImage: DefaultKarmadaSchedulerImage,
			},
			expected: "my-registry/karmada-scheduler:" + karmadaRelease,
		},
		{
			name: "KarmadaSchedulerImage is set to a non-default value",
			opt: &CommandInitOption{
				KarmadaSchedulerImage: "my-scheduler-image",
			},
			expected: "my-scheduler-image",
		},
		{
			name: "ImageRegistry is not set and KarmadaSchedulerImage is set to default value",
			opt: &CommandInitOption{
				KarmadaSchedulerImage: DefaultKarmadaSchedulerImage,
			},
			expected: DefaultKarmadaSchedulerImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opt.karmadaSchedulerImage()

			if result != tt.expected {
				t.Errorf("Unexpected result: %s, expected: %s", result, tt.expected)
			}
		})
	}
}
