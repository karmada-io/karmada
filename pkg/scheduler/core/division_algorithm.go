package core

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/util/sets"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// TargetClustersList is a slice of TargetCluster that implements sort.Interface to sort by Value.
type TargetClustersList []workv1alpha2.TargetCluster

func (a TargetClustersList) Len() int           { return len(a) }
func (a TargetClustersList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a TargetClustersList) Less(i, j int) bool { return a[i].Replicas > a[j].Replicas }

type dispenser struct {
	numReplicas int32
	result      []workv1alpha2.TargetCluster
}

func newDispenser(numReplicas int32, init []workv1alpha2.TargetCluster) *dispenser {
	cp := make([]workv1alpha2.TargetCluster, len(init))
	copy(cp, init)
	return &dispenser{numReplicas: numReplicas, result: cp}
}

func (a *dispenser) done() bool {
	return a.numReplicas == 0 && len(a.result) != 0
}

func (a *dispenser) takeByWeight(w helper.ClusterWeightInfoList) {
	if a.done() {
		return
	}
	sum := w.GetWeightSum()
	if sum == 0 {
		return
	}

	sort.Sort(w)

	result := make([]workv1alpha2.TargetCluster, 0, w.Len())
	remain := a.numReplicas
	for _, info := range w {
		replicas := int32(info.Weight * int64(a.numReplicas) / sum)
		result = append(result, workv1alpha2.TargetCluster{
			Name:     info.ClusterName,
			Replicas: replicas,
		})
		remain -= replicas
	}
	// TODO(Garrybest): take rest replicas by fraction part
	for i := range result {
		if remain == 0 {
			break
		}
		result[i].Replicas++
		remain--
	}

	a.numReplicas = remain
	a.result = util.MergeTargetClusters(a.result, result)
}

func getStaticWeightInfoList(clusters []*clusterv1alpha1.Cluster, weightList []policyv1alpha1.StaticClusterWeight) helper.ClusterWeightInfoList {
	list := make(helper.ClusterWeightInfoList, 0)
	for _, cluster := range clusters {
		var weight int64
		for _, staticWeightRule := range weightList {
			if util.ClusterMatches(cluster, staticWeightRule.TargetCluster) {
				weight = util.MaxInt64(weight, staticWeightRule.Weight)
			}
		}
		if weight > 0 {
			list = append(list, helper.ClusterWeightInfo{
				ClusterName: cluster.Name,
				Weight:      weight,
			})
		}
	}
	if list.GetWeightSum() == 0 {
		for _, cluster := range clusters {
			list = append(list, helper.ClusterWeightInfo{
				ClusterName: cluster.Name,
				Weight:      1,
			})
		}
	}
	return list
}

// divideReplicasByDynamicWeight assigns a total number of replicas to the selected clusters by the dynamic weight list.
func divideReplicasByDynamicWeight(clusters []*clusterv1alpha1.Cluster, dynamicWeight policyv1alpha1.DynamicWeightFactor, spec *workv1alpha2.ResourceBindingSpec) ([]workv1alpha2.TargetCluster, error) {
	switch dynamicWeight {
	case policyv1alpha1.DynamicWeightByAvailableReplicas:
		return divideReplicasByResource(clusters, spec, policyv1alpha1.ReplicaDivisionPreferenceWeighted)
	default:
		return nil, fmt.Errorf("undefined replica dynamic weight factor: %s", dynamicWeight)
	}
}

func divideReplicasByResource(
	clusters []*clusterv1alpha1.Cluster,
	spec *workv1alpha2.ResourceBindingSpec,
	preference policyv1alpha1.ReplicaDivisionPreference,
) ([]workv1alpha2.TargetCluster, error) {
	// Step 1: Find the ready clusters that have old replicas
	scheduledClusters := findOutScheduledCluster(spec.Clusters, clusters)

	// Step 2: calculate the assigned Replicas in scheduledClusters
	assignedReplicas := util.GetSumOfReplicas(scheduledClusters)

	// Step 3: Check the scale type (up or down).
	if assignedReplicas > spec.Replicas {
		// We need to reduce the replicas in terms of the previous result.
		newTargetClusters, err := scaleDownScheduleByReplicaDivisionPreference(spec, preference)
		if err != nil {
			return nil, fmt.Errorf("failed to scale down: %v", err)
		}

		return newTargetClusters, nil
	} else if assignedReplicas < spec.Replicas {
		// We need to enlarge the replicas in terms of the previous result (if exists).
		// First scheduling is considered as a special kind of scaling up.
		newTargetClusters, err := scaleUpScheduleByReplicaDivisionPreference(clusters, spec, preference, scheduledClusters, assignedReplicas)
		if err != nil {
			return nil, fmt.Errorf("failed to scaleUp: %v", err)
		}
		return newTargetClusters, nil
	} else {
		return scheduledClusters, nil
	}
}

// divideReplicasByPreference assigns a total number of replicas to the selected clusters by preference according to the resource.
func divideReplicasByPreference(
	clusterAvailableReplicas []workv1alpha2.TargetCluster,
	replicas int32,
	preference policyv1alpha1.ReplicaDivisionPreference,
	scheduledClusterNames sets.String,
) ([]workv1alpha2.TargetCluster, error) {
	clustersMaxReplicas := util.GetSumOfReplicas(clusterAvailableReplicas)
	if clustersMaxReplicas < replicas {
		return nil, fmt.Errorf("clusters resources are not enough to schedule, max %d replicas are support", clustersMaxReplicas)
	}

	switch preference {
	case policyv1alpha1.ReplicaDivisionPreferenceAggregated:
		return divideReplicasByAggregation(clusterAvailableReplicas, replicas, scheduledClusterNames), nil
	case policyv1alpha1.ReplicaDivisionPreferenceWeighted:
		return divideReplicasByAvailableReplica(clusterAvailableReplicas, replicas, clustersMaxReplicas), nil
	default:
		return nil, fmt.Errorf("undefined replicaSchedulingType： %v", preference)
	}
}

func divideReplicasByAggregation(clusterAvailableReplicas []workv1alpha2.TargetCluster,
	replicas int32, scheduledClusterNames sets.String) []workv1alpha2.TargetCluster {
	clusterAvailableReplicas = resortClusterList(clusterAvailableReplicas, scheduledClusterNames)
	clustersNum, clustersMaxReplicas := 0, int32(0)
	for _, clusterInfo := range clusterAvailableReplicas {
		clustersNum++
		clustersMaxReplicas += clusterInfo.Replicas
		if clustersMaxReplicas >= replicas {
			break
		}
	}
	return divideReplicasByAvailableReplica(clusterAvailableReplicas[0:clustersNum], replicas, clustersMaxReplicas)
}

func divideReplicasByAvailableReplica(clusterAvailableReplicas []workv1alpha2.TargetCluster, replicas int32,
	clustersMaxReplicas int32) []workv1alpha2.TargetCluster {
	desireReplicaInfos := make(map[string]int64)
	allocatedReplicas := int32(0)
	for _, clusterInfo := range clusterAvailableReplicas {
		desireReplicaInfos[clusterInfo.Name] = int64(clusterInfo.Replicas * replicas / clustersMaxReplicas)
		allocatedReplicas += int32(desireReplicaInfos[clusterInfo.Name])
	}

	var clusterNames []string
	for _, targetCluster := range clusterAvailableReplicas {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	divideRemainingReplicas(int(replicas-allocatedReplicas), desireReplicaInfos, clusterNames)

	targetClusters := make([]workv1alpha2.TargetCluster, len(desireReplicaInfos))
	i := 0
	for key, value := range desireReplicaInfos {
		targetClusters[i] = workv1alpha2.TargetCluster{Name: key, Replicas: int32(value)}
		i++
	}
	return targetClusters
}

// divideRemainingReplicas divide remaining Replicas to clusters and calculate desiredReplicaInfos
func divideRemainingReplicas(remainingReplicas int, desiredReplicaInfos map[string]int64, clusterNames []string) {
	if remainingReplicas <= 0 {
		return
	}

	clusterSize := len(clusterNames)
	if remainingReplicas < clusterSize {
		for i := 0; i < remainingReplicas; i++ {
			desiredReplicaInfos[clusterNames[i]]++
		}
	} else {
		avg, residue := remainingReplicas/clusterSize, remainingReplicas%clusterSize
		for i := 0; i < clusterSize; i++ {
			if i < residue {
				desiredReplicaInfos[clusterNames[i]] += int64(avg) + 1
			} else {
				desiredReplicaInfos[clusterNames[i]] += int64(avg)
			}
		}
	}
}

func scaleDownScheduleByReplicaDivisionPreference(
	spec *workv1alpha2.ResourceBindingSpec,
	preference policyv1alpha1.ReplicaDivisionPreference,
) ([]workv1alpha2.TargetCluster, error) {
	// The previous scheduling result will be the weight reference of scaling down.
	// In other words, we scale down the replicas proportionally by their scheduled replicas.
	return divideReplicasByPreference(spec.Clusters, spec.Replicas, preference, sets.NewString())
}

func scaleUpScheduleByReplicaDivisionPreference(
	clusters []*clusterv1alpha1.Cluster,
	spec *workv1alpha2.ResourceBindingSpec,
	preference policyv1alpha1.ReplicaDivisionPreference,
	scheduledClusters []workv1alpha2.TargetCluster,
	assignedReplicas int32,
) ([]workv1alpha2.TargetCluster, error) {
	// Step 1: Get how many replicas should be scheduled in this cycle and construct a new object if necessary
	newSpec := spec
	if assignedReplicas > 0 {
		newSpec = spec.DeepCopy()
		newSpec.Replicas = spec.Replicas - assignedReplicas
	}

	// Step 2: Calculate available replicas of all candidates
	clusterAvailableReplicas := calAvailableReplicas(clusters, newSpec)
	sort.Sort(TargetClustersList(clusterAvailableReplicas))

	// Step 3: Begin dividing.
	// Only the new replicas are considered during this scheduler, the old replicas will not be moved.
	// If not, the old replicas may be recreated which is not expected during scaling up.
	// The parameter `scheduledClusterNames` is used to make sure that we assign new replicas to them preferentially
	// so that all the replicas are aggregated.
	result, err := divideReplicasByPreference(clusterAvailableReplicas, newSpec.Replicas,
		preference, util.ConvertToClusterNames(scheduledClusters))
	if err != nil {
		return result, err
	}

	// Step 4: Merge the result of previous and new results.
	return util.MergeTargetClusters(scheduledClusters, result), nil
}
