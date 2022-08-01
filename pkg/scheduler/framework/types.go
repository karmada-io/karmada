package framework

import (
	"fmt"
	"sort"
	"strings"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
)

const (
	// NoClusterAvailableMsg is used to format message when no clusters available.
	NoClusterAvailableMsg = "0/%v clusters are available"
)

// ClusterToResultMap declares map from cluster name to its Result.
type ClusterToResultMap map[string]*Result

// ClusterInfo is cluster level aggregated information.
type ClusterInfo struct {
	// Overall cluster information.
	cluster *clusterv1alpha1.Cluster
}

// NewClusterInfo creates a ClusterInfo object.
func NewClusterInfo(cluster *clusterv1alpha1.Cluster) *ClusterInfo {
	return &ClusterInfo{
		cluster: cluster,
	}
}

// Cluster returns overall information about this cluster.
func (n *ClusterInfo) Cluster() *clusterv1alpha1.Cluster {
	if n == nil {
		return nil
	}
	return n.cluster
}

// Diagnosis records the details to diagnose a scheduling failure.
type Diagnosis struct {
	ClusterToResultMap ClusterToResultMap
}

// FitError describes a fit error of a object.
type FitError struct {
	NumAllClusters int
	Diagnosis      Diagnosis
}

// Error returns detailed information of why the object failed to fit on each cluster
func (f *FitError) Error() string {
	reasons := make(map[string]int)
	for _, result := range f.Diagnosis.ClusterToResultMap {
		for _, reason := range result.Reasons() {
			reasons[reason]++
		}
	}

	sortReasonsHistogram := func() []string {
		var reasonStrings []string
		for k, v := range reasons {
			reasonStrings = append(reasonStrings, fmt.Sprintf("%v %v", v, k))
		}
		sort.Strings(reasonStrings)
		return reasonStrings
	}
	reasonMsg := fmt.Sprintf(NoClusterAvailableMsg+": %v.", f.NumAllClusters, strings.Join(sortReasonsHistogram(), ", "))
	return reasonMsg
}
