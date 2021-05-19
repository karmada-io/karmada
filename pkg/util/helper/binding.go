package helper

import (
	"context"
	"sort"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

var resourceBindingKind = v1alpha1.SchemeGroupVersion.WithKind("ResourceBinding")
var clusterResourceBindingKind = v1alpha1.SchemeGroupVersion.WithKind("ClusterResourceBinding")

const (
	// DeploymentKind indicates the target resource is a deployment
	DeploymentKind = "Deployment"
	// SpecField indicates the 'spec' field of a deployment
	SpecField = "spec"
	// ReplicasField indicates the 'replicas' field of a deployment
	ReplicasField = "replicas"
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

func sortClusterByWeight(m map[string]int64) ClusterWeightInfoList {
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
			util.ResourceBindingNamespaceLabel: bindingNamespace,
			util.ResourceBindingNameLabel:      bindingName,
		})

		if err := c.List(context.TODO(), workList, &client.ListOptions{LabelSelector: selector}); err != nil {
			return nil, err
		}
	} else {
		selector := labels.SelectorFromSet(labels.Set{
			util.ClusterResourceBindingLabel: bindingName,
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
	for _, work := range works {
		err := c.Delete(context.TODO(), &work)
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

// EnsureWork ensure Work to be created or updated.
func EnsureWork(c client.Client, workload *unstructured.Unstructured, clusterNames []string, overrideManager overridemanager.OverrideManager,
	binding metav1.Object, scope apiextensionsv1.ResourceScope) error {
	referenceRSP, desireReplicaInfos, err := calculateReplicasIfNeeded(c, workload, clusterNames)
	if err != nil {
		klog.Errorf("Failed to get ReplicaSchedulingPolicy for %s/%s/%s, err is: %v", workload.GetKind(), workload.GetNamespace(), workload.GetName(), err)
		return err
	}

	var bindingGVK schema.GroupVersionKind
	var workLabel = make(map[string]string)

	for _, clusterName := range clusterNames {
		// apply override policies
		clonedWorkload := workload.DeepCopy()
		cops, ops, err := overrideManager.ApplyOverridePolicies(clonedWorkload, clusterName)
		if err != nil {
			klog.Errorf("Failed to apply overrides for %s/%s/%s, err is: %v", workload.GetKind(), workload.GetNamespace(), workload.GetName(), err)
			return err
		}

		workName := binding.GetName()
		workNamespace, err := names.GenerateExecutionSpaceName(clusterName)
		if err != nil {
			klog.Errorf("Failed to ensure Work for cluster: %s. Error: %v.", clusterName, err)
			return err
		}

		util.MergeLabel(clonedWorkload, util.WorkNamespaceLabel, workNamespace)
		util.MergeLabel(clonedWorkload, util.WorkNameLabel, workName)

		if scope == apiextensionsv1.NamespaceScoped {
			bindingGVK = resourceBindingKind
			util.MergeLabel(clonedWorkload, util.ResourceBindingNamespaceLabel, binding.GetNamespace())
			util.MergeLabel(clonedWorkload, util.ResourceBindingNameLabel, binding.GetName())
			workLabel[util.ResourceBindingNamespaceLabel] = binding.GetNamespace()
			workLabel[util.ResourceBindingNameLabel] = binding.GetName()
		} else {
			bindingGVK = clusterResourceBindingKind
			util.MergeLabel(clonedWorkload, util.ClusterResourceBindingLabel, binding.GetName())
			workLabel[util.ClusterResourceBindingLabel] = binding.GetName()
		}

		if clonedWorkload.GetKind() == DeploymentKind && referenceRSP != nil {
			err = applyReplicaSchedulingPolicy(clonedWorkload, desireReplicaInfos[clusterName])
			if err != nil {
				klog.Errorf("failed to apply ReplicaSchedulingPolicy for %s/%s/%s in cluster %s, err is: %v",
					clonedWorkload.GetKind(), clonedWorkload.GetNamespace(), clonedWorkload.GetName(), clusterName, err)
				return err
			}
		}

		workloadJSON, err := clonedWorkload.MarshalJSON()
		if err != nil {
			klog.Errorf("Failed to marshal workload, kind: %s, namespace: %s, name: %s. Error: %v",
				clonedWorkload.GetKind(), clonedWorkload.GetName(), clonedWorkload.GetNamespace(), err)
			return err
		}

		work := &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name:       workName,
				Namespace:  workNamespace,
				Finalizers: []string{util.ExecutionControllerFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(binding, bindingGVK),
				},
				Labels: workLabel,
			},
			Spec: workv1alpha1.WorkSpec{
				Workload: workv1alpha1.WorkloadTemplate{
					Manifests: []workv1alpha1.Manifest{
						{
							RawExtension: runtime.RawExtension{
								Raw: workloadJSON,
							},
						},
					},
				},
			},
		}

		// set applied override policies if needed.
		var appliedBytes []byte
		if cops != nil {
			appliedBytes, err = cops.MarshalJSON()
			if err != nil {
				return err
			}
			if appliedBytes != nil {
				if work.Annotations == nil {
					work.Annotations = make(map[string]string, 1)
				}
				work.Annotations[util.AppliedClusterOverrides] = string(appliedBytes)
			}
		}
		if ops != nil {
			appliedBytes, err = ops.MarshalJSON()
			if err != nil {
				return err
			}
			if appliedBytes != nil {
				if work.Annotations == nil {
					work.Annotations = make(map[string]string, 1)
				}
				work.Annotations[util.AppliedOverrides] = string(appliedBytes)
			}
		}

		runtimeObject := work.DeepCopy()
		operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), c, runtimeObject, func() error {
			runtimeObject.Annotations = work.Annotations
			runtimeObject.Labels = work.Labels
			runtimeObject.Spec = work.Spec
			return nil
		})
		if err != nil {
			klog.Errorf("Failed to create/update work %s/%s. Error: %v", work.GetNamespace(), work.GetName(), err)
			return err
		}

		if operationResult == controllerutil.OperationResultCreated {
			klog.Infof("Create work %s/%s successfully.", work.GetNamespace(), work.GetName())
		} else if operationResult == controllerutil.OperationResultUpdated {
			klog.Infof("Update work %s/%s successfully.", work.GetNamespace(), work.GetName())
		} else {
			klog.V(2).Infof("Work %s/%s is up to date.", work.GetNamespace(), work.GetName())
		}
	}
	return nil
}

func calculateReplicasIfNeeded(c client.Client, workload *unstructured.Unstructured, clusterNames []string) (*v1alpha1.ReplicaSchedulingPolicy, map[string]int64, error) {
	var err error
	var referenceRSP *v1alpha1.ReplicaSchedulingPolicy
	var desireReplicaInfos = make(map[string]int64)

	if workload.GetKind() == DeploymentKind {
		referenceRSP, err = matchReplicaSchedulingPolicy(c, workload)
		if err != nil {
			return nil, nil, err
		}
		if referenceRSP != nil {
			desireReplicaInfos, err = calculateReplicas(c, referenceRSP, clusterNames)
			if err != nil {
				klog.Errorf("Failed to get desire replicas for %s/%s/%s, err is: %v", workload.GetKind(), workload.GetNamespace(), workload.GetName(), err)
				return nil, nil, err
			}
			klog.V(4).Infof("DesireReplicaInfos with replica scheduling policies(%s/%s) is %v", referenceRSP.Namespace, referenceRSP.Name, desireReplicaInfos)
		}
	}
	return referenceRSP, desireReplicaInfos, nil
}

func matchReplicaSchedulingPolicy(c client.Client, workload *unstructured.Unstructured) (*v1alpha1.ReplicaSchedulingPolicy, error) {
	// get all namespace-scoped replica scheduling policies
	policyList := &v1alpha1.ReplicaSchedulingPolicyList{}
	if err := c.List(context.TODO(), policyList, &client.ListOptions{Namespace: workload.GetNamespace()}); err != nil {
		klog.Errorf("Failed to list replica scheduling policies from namespace: %s, error: %v", workload.GetNamespace(), err)
		return nil, err
	}

	if len(policyList.Items) == 0 {
		return nil, nil
	}

	matchedPolicies := getMatchedReplicaSchedulingPolicy(policyList.Items, workload)
	if len(matchedPolicies) == 0 {
		klog.V(2).Infof("No replica scheduling policy for resource: %s/%s", workload.GetNamespace(), workload.GetName())
		return nil, nil
	}

	return &matchedPolicies[0], nil
}

func getMatchedReplicaSchedulingPolicy(policies []v1alpha1.ReplicaSchedulingPolicy, resource *unstructured.Unstructured) []v1alpha1.ReplicaSchedulingPolicy {
	// select policy in which at least one resource selector matches target resource.
	resourceMatches := make([]v1alpha1.ReplicaSchedulingPolicy, 0)
	for _, policy := range policies {
		if util.ResourceMatchSelectors(resource, policy.Spec.ResourceSelectors...) {
			resourceMatches = append(resourceMatches, policy)
		}
	}

	// Sort by policy names.
	sort.Slice(resourceMatches, func(i, j int) bool {
		return resourceMatches[i].Name < resourceMatches[j].Name
	})

	return resourceMatches
}

func calculateReplicas(c client.Client, policy *v1alpha1.ReplicaSchedulingPolicy, clusterNames []string) (map[string]int64, error) {
	weightSum := int64(0)
	matchClusters := make(map[string]int64)
	desireReplicaInfos := make(map[string]int64)

	// found out clusters matched the given ReplicaSchedulingPolicy
	for _, clusterName := range clusterNames {
		clusterObj := &clusterv1alpha1.Cluster{}
		if err := c.Get(context.TODO(), client.ObjectKey{Name: clusterName}, clusterObj); err != nil {
			klog.Errorf("Failed to get member cluster: %s, error: %v", clusterName, err)
			return nil, err
		}
		for _, staticWeightRule := range policy.Spec.Preferences.StaticWeightList {
			if util.ClusterMatches(clusterObj, staticWeightRule.TargetCluster) {
				weightSum += staticWeightRule.Weight
				matchClusters[clusterName] = staticWeightRule.Weight
				break
			}
		}
	}

	allocatedReplicas := int32(0)
	for clusterName, weight := range matchClusters {
		desireReplicaInfos[clusterName] = weight * int64(policy.Spec.TotalReplicas) / weightSum
		allocatedReplicas += int32(desireReplicaInfos[clusterName])
	}

	if remainReplicas := policy.Spec.TotalReplicas - allocatedReplicas; remainReplicas > 0 {
		sortedClusters := sortClusterByWeight(matchClusters)
		for i := 0; remainReplicas > 0; i++ {
			desireReplicaInfos[sortedClusters[i].ClusterName]++
			remainReplicas--
			if i == len(desireReplicaInfos) {
				i = 0
			}
		}
	}

	for _, clusterName := range clusterNames {
		if _, exist := matchClusters[clusterName]; !exist {
			desireReplicaInfos[clusterName] = 0
		}
	}

	return desireReplicaInfos, nil
}

func applyReplicaSchedulingPolicy(workload *unstructured.Unstructured, desireReplica int64) error {
	_, ok, err := unstructured.NestedInt64(workload.Object, SpecField, ReplicasField)
	if err != nil {
		return err
	}
	if ok {
		err := unstructured.SetNestedField(workload.Object, desireReplica, SpecField, ReplicasField)
		if err != nil {
			return err
		}
	}
	return nil
}
