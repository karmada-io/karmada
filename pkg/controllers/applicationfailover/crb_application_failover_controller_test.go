package applicationfailover

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func generateCRBApplicationFailoverController() *CRBApplicationFailoverController {
	m := newWorkloadUnhealthyMap()
	c := &CRBApplicationFailoverController{
		Client:               fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
		EventRecorder:        &record.FakeRecorder{},
		workloadUnhealthyMap: m,
	}
	return c
}

func TestCRBApplicationFailoverController_Reconcile(t *testing.T) {
	t.Run("failed in clusterResourceBindingFilter", func(t *testing.T) {
		binding := &workv1alpha2.ClusterResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "binding",
				Namespace: "default",
			},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "pod",
				},
			},
		}
		c := generateCRBApplicationFailoverController()

		// Prepare req
		req := controllerruntime.Request{
			NamespacedName: types.NamespacedName{
				Name:      "binding",
				Namespace: "default",
			},
		}

		if err := c.Client.Create(context.Background(), binding); err != nil {
			t.Fatalf("Failed to create binding: %v", err)
		}

		res, err := c.Reconcile(context.Background(), req)
		assert.Equal(t, controllerruntime.Result{}, res)
		assert.Equal(t, nil, err)
	})

	t.Run("failed in c.Client.Get", func(t *testing.T) {
		c := generateCRBApplicationFailoverController()

		// Prepare req
		req := controllerruntime.Request{
			NamespacedName: types.NamespacedName{
				Name:      "binding",
				Namespace: "default",
			},
		}

		res, err := c.Reconcile(context.Background(), req)
		assert.Equal(t, controllerruntime.Result{}, res)
		assert.Equal(t, nil, err)
	})
}

func TestCRBApplicationFailoverController_detectFailure(t *testing.T) {
	cluster1 := "cluster1"
	cluster2 := "cluster2"
	key := types.NamespacedName{
		Namespace: "default",
		Name:      "test",
	}

	t.Run("hasWorkloadBeenUnhealthy return false", func(t *testing.T) {
		clusters := []string{cluster1, cluster2}
		tolerationSeconds := int32(1)

		c := generateCRBApplicationFailoverController()
		duration, needEvictClusters := c.detectFailure(clusters, &tolerationSeconds, key)
		assert.Equal(t, tolerationSeconds, duration)
		assert.Equal(t, []string(nil), needEvictClusters)
	})

	t.Run("more than the tolerance time", func(t *testing.T) {
		clusters := []string{cluster1, cluster2}
		tolerationSeconds := int32(1)

		c := generateCRBApplicationFailoverController()
		c.workloadUnhealthyMap.setTimeStamp(key, cluster1)
		time.Sleep(2 * time.Second)
		duration, needEvictClusters := c.detectFailure(clusters, &tolerationSeconds, key)
		assert.Equal(t, tolerationSeconds, duration)
		assert.Equal(t, []string{"cluster1"}, needEvictClusters)
	})

	t.Run("less than the tolerance time", func(t *testing.T) {
		clusters := []string{cluster1, cluster2}
		tolerationSeconds := int32(100)

		c := generateCRBApplicationFailoverController()
		c.workloadUnhealthyMap.setTimeStamp(key, cluster1)
		duration, needEvictClusters := c.detectFailure(clusters, &tolerationSeconds, key)
		assert.Equal(t, tolerationSeconds, duration)
		assert.Equal(t, []string(nil), needEvictClusters)
	})

	t.Run("final duration is 0", func(t *testing.T) {
		clusters := []string{}
		tolerationSeconds := int32(100)

		c := generateCRBApplicationFailoverController()
		duration, needEvictClusters := c.detectFailure(clusters, &tolerationSeconds, key)
		assert.Equal(t, int32(0), duration)
		assert.Equal(t, []string(nil), needEvictClusters)
	})
}

func TestCRBApplicationFailoverController_syncBinding(t *testing.T) {
	tolerationSeconds := int32(5)
	c := generateCRBApplicationFailoverController()
	binding := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding",
			Namespace: "default",
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "pod",
			},
			Failover: &policyv1alpha1.FailoverBehavior{
				Application: &policyv1alpha1.ApplicationFailoverBehavior{
					DecisionConditions: policyv1alpha1.DecisionConditions{
						TolerationSeconds: &tolerationSeconds,
					},
				},
			},
			Clusters: []workv1alpha2.TargetCluster{
				{
					Name:     "member1",
					Replicas: 1,
				},
				{
					Name:     "member1",
					Replicas: 2,
				},
			},
		},
		Status: workv1alpha2.ResourceBindingStatus{
			AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Health:      workv1alpha2.ResourceHealthy,
				},
				{
					ClusterName: "member2",
					Health:      workv1alpha2.ResourceUnhealthy,
				},
			},
		},
	}

	dur, err := c.syncBinding(binding)
	assert.Equal(t, 5*time.Second, dur)
	assert.NoError(t, err)
}

func TestCRBApplicationFailoverController_evictBinding(t *testing.T) {
	tests := []struct {
		name        string
		purgeMode   policyv1alpha1.PurgeMode
		expectError bool
	}{
		{
			name:        "PurgeMode is Graciously",
			purgeMode:   policyv1alpha1.Graciously,
			expectError: false,
		},
		{
			name:        "PurgeMode is Never",
			purgeMode:   policyv1alpha1.Never,
			expectError: false,
		},
		{
			name:        "PurgeMode is Immediately",
			purgeMode:   policyv1alpha1.Immediately,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := generateCRBApplicationFailoverController()
			binding := &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
					Failover: &policyv1alpha1.FailoverBehavior{
						Application: &policyv1alpha1.ApplicationFailoverBehavior{
							PurgeMode: tt.purgeMode,
						},
					},
				},
			}
			clusters := []string{"member1", "member2"}
			err := c.evictBinding(binding, clusters)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCRBApplicationFailoverController_updateBinding(t *testing.T) {
	binding := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "binding",
			Namespace: "default",
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "pod",
			},
		},
	}
	allClusters := sets.New("member1", "member2", "member3")
	needEvictClusters := []string{"member1", "member2"}

	c := generateCRBApplicationFailoverController()

	t.Run("failed when c.Update", func(t *testing.T) {
		err := c.updateBinding(binding, allClusters, needEvictClusters)
		assert.Error(t, err)
	})

	t.Run("normal case", func(t *testing.T) {
		if err := c.Client.Create(context.Background(), binding); err != nil {
			t.Fatalf("Failed to create binding: %v", err)
		}
		err := c.updateBinding(binding, allClusters, needEvictClusters)
		assert.NoError(t, err)
	})
}

func generateRaw() *runtime.RawExtension {
	testTime := time.Now()
	testV1time := metav1.NewTime(testTime)
	statusMap := map[string]interface{}{
		"active":         0,
		"succeeded":      1,
		"startTime":      testV1time,
		"completionTime": testV1time,
		"failed":         0,
		"conditions":     []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}},
	}
	raw, _ := helper.BuildStatusRawExtension(statusMap)
	return raw
}

func TestCRBApplicationFailoverController_clusterResourceBindingFilter(t *testing.T) {
	tests := []struct {
		name      string
		binding   *workv1alpha2.ClusterResourceBinding
		expectRes bool
	}{
		{
			name: "crb.Spec.Failover and crb.Spec.Failover.Application is nil",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
				},
			},
			expectRes: false,
		},
		{
			name: "crb.Status.AggregatedStatus is 0",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
					Failover: &policyv1alpha1.FailoverBehavior{
						Application: &policyv1alpha1.ApplicationFailoverBehavior{},
					},
				},
			},
			expectRes: false,
		},
		{
			name: "error occurs in ConstructClusterWideKey",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion: "a/b/c",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
					Failover: &policyv1alpha1.FailoverBehavior{
						Application: &policyv1alpha1.ApplicationFailoverBehavior{},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
						{ClusterName: "memberA", Status: generateRaw()},
						{ClusterName: "memberB", Status: generateRaw()},
					},
				},
			},
			expectRes: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := generateCRBApplicationFailoverController()
			res := c.clusterResourceBindingFilter(tt.binding)
			assert.Equal(t, tt.expectRes, res)
		})
	}
}
