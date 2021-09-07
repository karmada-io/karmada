package helper

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

// ClusterWeightInfo records the weight of a cluster
type ClusterWeightInfo struct {
	ClusterName string
	Weight      int64
}

// ClusterWeightInfoList is a slice of ClusterWeightInfo that implements sort.Interface to sort by Value.
type ClusterWeightInfoList []ClusterWeightInfo

func (p ClusterWeightInfoList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p ClusterWeightInfoList) Len() int      { return len(p) }
func (p ClusterWeightInfoList) Less(i, j int) bool {
	if p[i].Weight != p[j].Weight {
		return p[i].Weight > p[j].Weight
	}
	return p[i].ClusterName < p[j].ClusterName
}

// SortClusterByWeight sort clusters by the weight
func SortClusterByWeight(m map[string]int64) ClusterWeightInfoList {
	p := make(ClusterWeightInfoList, len(m))
	i := 0
	for k, v := range m {
		p[i] = ClusterWeightInfo{k, v}
		i++
	}
	sort.Sort(p)
	return p
}

// IsBindingReady will check if resourceBinding/clusterResourceBinding is ready to build Work.
func IsBindingReady(targetClusters []workv1alpha1.TargetCluster) bool {
	return len(targetClusters) != 0
}

// HasScheduledReplica checks if the scheduler has assigned replicas for each cluster.
func HasScheduledReplica(scheduleResult []workv1alpha1.TargetCluster) bool {
	for _, clusterResult := range scheduleResult {
		if clusterResult.Replicas > 0 {
			return true
		}
	}
	return false
}

// GetBindingClusterNames will get clusterName list from bind clusters field
func GetBindingClusterNames(targetClusters []workv1alpha1.TargetCluster) []string {
	var clusterNames []string
	for _, targetCluster := range targetClusters {
		clusterNames = append(clusterNames, targetCluster.Name)
	}
	return clusterNames
}

// FindOrphanWorks retrieves all works that labeled with current binding(ResourceBinding or ClusterResourceBinding) objects,
// then pick the works that not meet current binding declaration.
func FindOrphanWorks(c client.Client, bindingNamespace, bindingName string, clusterNames []string, scope apiextensionsv1.ResourceScope) ([]workv1alpha1.Work, error) {
	workList := &workv1alpha1.WorkList{}
	if scope == apiextensionsv1.NamespaceScoped {
		selector := labels.SelectorFromSet(labels.Set{
			workv1alpha1.ResourceBindingNamespaceLabel: bindingNamespace,
			workv1alpha1.ResourceBindingNameLabel:      bindingName,
		})

		if err := c.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector}); err != nil {
			return nil, err
		}
	} else {
		selector := labels.SelectorFromSet(labels.Set{
			workv1alpha1.ClusterResourceBindingLabel: bindingName,
		})

		if err := c.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector}); err != nil {
			return nil, err
		}
	}

	var orphanWorks []workv1alpha1.Work
	expectClusters := sets.NewString(clusterNames...)
	for _, work := range workList.Items {
		workTargetCluster, err := names.GetClusterName(work.GetNamespace())
		if err != nil {
			klog.Errorf("Failed to get cluster name which Work %s/%s belongs to. Error: %v.",
				work.GetNamespace(), work.GetName(), err)
			return nil, err
		}
		if !expectClusters.Has(workTargetCluster) {
			orphanWorks = append(orphanWorks, work)
		}
	}
	return orphanWorks, nil
}

// RemoveOrphanWorks will remove orphan works.
func RemoveOrphanWorks(c client.Client, works []workv1alpha1.Work) error {
	for workIndex, work := range works {
		err := c.Delete(context.TODO(), &works[workIndex])
		if err != nil {
			return err
		}
		klog.Infof("Delete orphan work %s/%s successfully.", work.GetNamespace(), work.GetName())
	}
	return nil
}

// FetchWorkload fetches the kubernetes resource to be propagated.
func FetchWorkload(dynamicClient dynamic.Interface, restMapper meta.RESTMapper, resource workv1alpha1.ObjectReference) (*unstructured.Unstructured, error) {
	dynamicResource, err := restmapper.GetGroupVersionResource(restMapper,
		schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s %s. Error: %v", resource.APIVersion,
			resource.Kind, err)
		return nil, err
	}

	workload, err := dynamicClient.Resource(dynamicResource).Namespace(resource.Namespace).Get(context.TODO(),
		resource.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get workload, kind: %s, namespace: %s, name: %s. Error: %v",
			resource.Kind, resource.Namespace, resource.Name, err)
		return nil, err
	}

	return workload, nil
}

// GetClusterResourceBindings returns a ClusterResourceBindingList by labels.
func GetClusterResourceBindings(c client.Client, ls labels.Set) (*workv1alpha1.ClusterResourceBindingList, error) {
	bindings := &workv1alpha1.ClusterResourceBindingList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return bindings, c.List(context.TODO(), bindings, listOpt)
}

// GetResourceBindings returns a ResourceBindingList by labels
func GetResourceBindings(c client.Client, ls labels.Set) (*workv1alpha1.ResourceBindingList, error) {
	bindings := &workv1alpha1.ResourceBindingList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return bindings, c.List(context.TODO(), bindings, listOpt)
}

// GetWorks returns a WorkList by labels
func GetWorks(c client.Client, ls labels.Set) (*workv1alpha1.WorkList, error) {
	works := &workv1alpha1.WorkList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return works, c.List(context.TODO(), works, listOpt)
}

// DeleteWorks will delete all Work objects by labels.
func DeleteWorks(c client.Client, selector labels.Set) (controllerruntime.Result, error) {
	workList, err := GetWorks(c, selector)
	if err != nil {
		klog.Errorf("Failed to get works by label %v: %v", selector, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	var errs []error
	for index, work := range workList.Items {
		if err := c.Delete(context.TODO(), &workList.Items[index]); err != nil {
			klog.Errorf("Failed to delete work(%s/%s): %v", work.Namespace, work.Name, err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return controllerruntime.Result{Requeue: true}, errors.NewAggregate(errs)
	}

	return controllerruntime.Result{}, nil
}

// GenerateNodeClaimByPodSpec will return a NodeClaim from PodSpec.
func GenerateNodeClaimByPodSpec(podSpec *corev1.PodSpec) *workv1alpha1.NodeClaim {
	nodeClaim := &workv1alpha1.NodeClaim{
		NodeSelector: podSpec.NodeSelector,
		Tolerations:  podSpec.Tolerations,
	}
	hasAffinity := podSpec.Affinity != nil && podSpec.Affinity.NodeAffinity != nil && podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil
	if hasAffinity {
		nodeClaim.HardNodeAffinity = podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	}
	if nodeClaim.NodeSelector == nil && nodeClaim.HardNodeAffinity == nil && len(nodeClaim.Tolerations) == 0 {
		return nil
	}
	return nodeClaim
}
