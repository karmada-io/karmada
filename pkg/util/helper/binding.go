package helper

import (
	"context"

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

	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/overridemanager"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

var resourceBindingKind = v1alpha1.SchemeGroupVersion.WithKind("ResourceBinding")
var clusterResourceBindingKind = v1alpha1.SchemeGroupVersion.WithKind("ClusterResourceBinding")

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

	var bindingGVK schema.GroupVersionKind
	var workLabel = make(map[string]string)

	for _, clusterName := range clusterNames {
		// apply override policies
		clonedWorkload := workload.DeepCopy()
		cops, ops, err := overrideManager.ApplyOverridePolicies(clonedWorkload, clusterName)
		if err != nil {
			klog.Errorf("failed to apply overrides for %s/%s/%s, err is: %v", workload.GetKind(), workload.GetNamespace(), workload.GetName(), err)
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
