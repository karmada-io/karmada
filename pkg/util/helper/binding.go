package helper

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
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

// IsBindingScheduled will check if resourceBinding/clusterResourceBinding is successfully scheduled.
func IsBindingScheduled(status *workv1alpha2.ResourceBindingStatus) bool {
	return meta.IsStatusConditionTrue(status.Conditions, workv1alpha2.Scheduled)
}

// HasScheduledReplica checks if the scheduler has assigned replicas for a cluster.
func HasScheduledReplica(scheduleResult []workv1alpha2.TargetCluster) bool {
	for _, clusterResult := range scheduleResult {
		if clusterResult.Replicas > 0 {
			return true
		}
	}
	return false
}

// GetBindingClusterNames will get clusterName list from bind clusters field and requiredBy field.
func GetBindingClusterNames(targetClusters []workv1alpha2.TargetCluster, bindingSnapshot []workv1alpha2.BindingSnapshot) []string {
	clusterNames := util.ConvertToClusterNames(targetClusters)
	for _, binding := range bindingSnapshot {
		for _, targetCluster := range binding.Clusters {
			clusterNames.Insert(targetCluster.Name)
		}
	}

	return clusterNames.List()
}

// FindOrphanWorks retrieves all works that labeled with current binding(ResourceBinding or ClusterResourceBinding) objects,
// then pick the works that not meet current binding declaration.
func FindOrphanWorks(c client.Client, bindingNamespace, bindingName string, clusterNames []string, scope apiextensionsv1.ResourceScope) ([]workv1alpha1.Work, error) {
	var needJudgeWorks []workv1alpha1.Work
	if scope == apiextensionsv1.NamespaceScoped {
		workList, err := GetWorksByBindingNamespaceName(c, bindingNamespace, bindingName)
		if err != nil {
			klog.Errorf("Failed to get works by ResourceBinding(%s/%s): %v", bindingNamespace, bindingName, err)
			return nil, err
		}
		needJudgeWorks = append(needJudgeWorks, workList.Items...)
	} else {
		workList, err := GetWorksByBindingNamespaceName(c, "", bindingName)
		if err != nil {
			klog.Errorf("Failed to get works by ClusterResourceBinding(%s): %v", bindingName, err)
			return nil, err
		}
		needJudgeWorks = append(needJudgeWorks, workList.Items...)
	}

	var orphanWorks []workv1alpha1.Work
	expectClusters := sets.NewString(clusterNames...)
	for _, work := range needJudgeWorks {
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
	var errs []error
	for workIndex, work := range works {
		err := c.Delete(context.TODO(), &works[workIndex])
		if err != nil {
			klog.Errorf("Failed to delete orphan work %s/%s, err is %v", work.GetNamespace(), work.GetName(), err)
			errs = append(errs, err)
			continue
		}
		klog.Infof("Delete orphan work %s/%s successfully.", work.GetNamespace(), work.GetName())
	}
	return errors.NewAggregate(errs)
}

// FetchWorkload fetches the kubernetes resource to be propagated.
func FetchWorkload(dynamicClient dynamic.Interface, informerManager informermanager.SingleClusterInformerManager,
	restMapper meta.RESTMapper, resource workv1alpha2.ObjectReference) (*unstructured.Unstructured, error) {
	dynamicResource, err := restmapper.GetGroupVersionResource(restMapper,
		schema.FromAPIVersionAndKind(resource.APIVersion, resource.Kind))
	if err != nil {
		klog.Errorf("Failed to get GVR from GVK %s %s. Error: %v", resource.APIVersion,
			resource.Kind, err)
		return nil, err
	}

	var workload runtime.Object

	if len(resource.Namespace) == 0 {
		// cluster-scoped resource
		workload, err = informerManager.Lister(dynamicResource).Get(resource.Name)
	} else {
		workload, err = informerManager.Lister(dynamicResource).ByNamespace(resource.Namespace).Get(resource.Name)
	}
	if err != nil {
		// fall back to call api server in case the cache has not been synchronized yet
		klog.Warningf("Failed to get workload from cache, kind: %s, namespace: %s, name: %s. Error: %v. Fall back to call api server",
			resource.Kind, resource.Namespace, resource.Name, err)
		workload, err = dynamicClient.Resource(dynamicResource).Namespace(resource.Namespace).Get(context.TODO(),
			resource.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to get workload from api server, kind: %s, namespace: %s, name: %s. Error: %v",
				resource.Kind, resource.Namespace, resource.Name, err)
			return nil, err
		}
	}

	unstructuredWorkLoad, err := ToUnstructured(workload)
	if err != nil {
		klog.Errorf("Failed to transform object(%s/%s): %v", resource.Namespace, resource.Name, err)
		return nil, err
	}

	return unstructuredWorkLoad, nil
}

// GetClusterResourceBindings returns a ClusterResourceBindingList by labels.
func GetClusterResourceBindings(c client.Client, ls labels.Set) (*workv1alpha2.ClusterResourceBindingList, error) {
	bindings := &workv1alpha2.ClusterResourceBindingList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return bindings, c.List(context.TODO(), bindings, listOpt)
}

// GetResourceBindings returns a ResourceBindingList by labels
func GetResourceBindings(c client.Client, ls labels.Set) (*workv1alpha2.ResourceBindingList, error) {
	bindings := &workv1alpha2.ResourceBindingList{}
	listOpt := &client.ListOptions{LabelSelector: labels.SelectorFromSet(ls)}

	return bindings, c.List(context.TODO(), bindings, listOpt)
}

// DeleteWorkByRBNamespaceAndName will delete all Work objects by ResourceBinding namespace and name.
func DeleteWorkByRBNamespaceAndName(c client.Client, namespace, name string) error {
	return DeleteWorks(c, namespace, name)
}

// DeleteWorkByCRBName will delete all Work objects by ClusterResourceBinding name.
func DeleteWorkByCRBName(c client.Client, name string) error {
	return DeleteWorks(c, "", name)
}

// DeleteWorks will delete all Work objects by labels.
func DeleteWorks(c client.Client, namespace, name string) error {
	workList, err := GetWorksByBindingNamespaceName(c, namespace, name)
	if err != nil {
		klog.Errorf("Failed to get works by ResourceBinding(%s/%s) : %v", namespace, name, err)
		return err
	}

	var errs []error
	for index, work := range workList.Items {
		if err := c.Delete(context.TODO(), &workList.Items[index]); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			klog.Errorf("Failed to delete work(%s/%s): %v", work.Namespace, work.Name, err)
			errs = append(errs, err)
		}
	}
	return errors.NewAggregate(errs)
}

// GenerateNodeClaimByPodSpec will return a NodeClaim from PodSpec.
func GenerateNodeClaimByPodSpec(podSpec *corev1.PodSpec) *workv1alpha2.NodeClaim {
	nodeClaim := &workv1alpha2.NodeClaim{
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

// GenerateReplicaRequirements generates replica requirements for node and resources.
func GenerateReplicaRequirements(podTemplate *corev1.PodTemplateSpec) *workv1alpha2.ReplicaRequirements {
	nodeClaim := GenerateNodeClaimByPodSpec(&podTemplate.Spec)
	resourceRequest := util.EmptyResource().AddPodTemplateRequest(&podTemplate.Spec).ResourceList()

	if nodeClaim != nil || resourceRequest != nil {
		return &workv1alpha2.ReplicaRequirements{
			NodeClaim:       nodeClaim,
			ResourceRequest: resourceRequest,
		}
	}

	return nil
}
