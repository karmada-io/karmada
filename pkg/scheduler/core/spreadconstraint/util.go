package spreadconstraint

import (
	"context"
	"fmt"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"math"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	"github.com/karmada-io/karmada/pkg/util"
)

func calAvailableReplicas(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster {
	availableClusters := make([]workv1alpha2.TargetCluster, len(clusters))

	// Set the boundary.
	for i := range availableClusters {
		availableClusters[i].Name = clusters[i].Name
		availableClusters[i].Replicas = math.MaxInt32
	}

	// Get the minimum value of MaxAvailableReplicas in terms of all estimators.
	estimators := estimatorclient.GetReplicaEstimators()
	ctx := context.WithValue(context.TODO(), util.ContextKeyObject,
		fmt.Sprintf("kind=%s, name=%s/%s", spec.Resource.Kind, spec.Resource.Namespace, spec.Resource.Name))
	for _, estimator := range estimators {
		res, err := estimator.MaxAvailableReplicas(ctx, clusters, spec.ReplicaRequirements)
		if err != nil {
			klog.Errorf("Max cluster available replicas error: %v", err)
			continue
		}
		for i := range res {
			if res[i].Replicas == estimatorclient.UnauthenticReplica {
				continue
			}
			if availableClusters[i].Name == res[i].Name && availableClusters[i].Replicas > res[i].Replicas {
				availableClusters[i].Replicas = res[i].Replicas
			}
		}
	}

	// In most cases, the target cluster max available replicas should not be MaxInt32 unless the workload is best-effort
	// and the scheduler-estimator has not been enabled. So we set the replicas to spec.Replicas for avoiding overflow.
	for i := range availableClusters {
		if availableClusters[i].Replicas == math.MaxInt32 {
			availableClusters[i].Replicas = spec.Replicas
		}
	}

	klog.V(4).Infof("cluster replicas info: %v", availableClusters)
	return availableClusters
}

// IsSpreadConstraintExisted judge if the specific field is existed in the spread constraints
func IsSpreadConstraintExisted(spreadConstraints []policyv1alpha1.SpreadConstraint, field policyv1alpha1.SpreadFieldValue) bool {
	for _, spreadConstraint := range spreadConstraints {
		if spreadConstraint.SpreadByField == field {
			return true
		}
	}

	return false
}
