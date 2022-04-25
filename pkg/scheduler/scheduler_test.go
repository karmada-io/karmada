package scheduler

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
)

func TestCreateScheduler(t *testing.T) {
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	karmadaClient := karmadafake.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()

	testcases := []struct {
		name                     string
		opts                     []Option
		enableSchedulerEstimator bool
	}{
		{
			name:                     "scheduler with default configuration",
			opts:                     nil,
			enableSchedulerEstimator: false,
		},
		{
			name: "scheduler with enableSchedulerEstimator enabled",
			opts: []Option{
				WithEnableSchedulerEstimator(true),
			},
			enableSchedulerEstimator: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			sche, err := NewScheduler(dynamicClient, karmadaClient, kubeClient, tc.opts...)
			if err != nil {
				t.Errorf("create scheduler error: %s", err)
			}

			if tc.enableSchedulerEstimator != sche.enableSchedulerEstimator {
				t.Errorf("unexpected enableSchedulerEstimator want %v, got %v", tc.enableSchedulerEstimator, sche.enableSchedulerEstimator)
			}
		})
	}
}
