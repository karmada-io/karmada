package mcs

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	mcsv1alpha1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	mcsDiscoveryAnnotationKey = "discovery.karmada.io/strategy"

	mcsDiscoveryRemoteAndLocalStrategy = "RemoteAndLocal"
)

func strategyWithNativeService(mcs *networkingv1alpha1.MultiClusterService) bool {
	// TODO: once the MCS API is refined, modify the logic of
	// this judgment strategy here.
	_, exist := mcs.Annotations[mcsDiscoveryAnnotationKey]
	return exist
}

func createServiceExportTemplate(svc *corev1.Service) *mcsv1alpha1.ServiceExport {
	return &mcsv1alpha1.ServiceExport{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svc.Namespace,
			Name:      svc.Name,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(svc, corev1.SchemeGroupVersion.WithKind("Service")),
			},
			Finalizers: []string{karmadaMCSFinalizer},
		},
	}
}

func deDuplicatesInDestination(source sets.Set[string], destination []string) []workv1alpha2.TargetCluster {
	clusterNames := sets.New[string]()
	for _, cluster := range destination {
		if !source.Has(cluster) {
			clusterNames.Insert(cluster)
		}
	}
	return util.ConvertFromStringSetToTargetClusters(clusterNames)
}

func buildNativeServiceRB(svc *corev1.Service, targetClusters []workv1alpha2.TargetCluster) *workv1alpha2.ResourceBinding {
	return &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: svc.GetNamespace(),
			Name:      generateNativeServiceBindingName(util.ServiceKind, svc.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(svc, corev1.SchemeGroupVersion.WithKind(util.ServiceKind)),
			},
			Finalizers: []string{util.BindingControllerFinalizer},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion:      corev1.SchemeGroupVersion.String(),
				Kind:            util.ServiceKind,
				Namespace:       svc.GetNamespace(),
				Name:            svc.GetName(),
				UID:             svc.GetUID(),
				ResourceVersion: svc.GetResourceVersion(),
			},
			Clusters: targetClusters,
		},
	}
}

func buildEndpointSliceRB(eps *discoveryv1.EndpointSlice, targetClusters []string) *workv1alpha2.ResourceBinding {
	fromCluster := extractClusterName(eps.Name)
	rbSpecClusters := make([]workv1alpha2.TargetCluster, 0, len(targetClusters))
	for _, cluster := range targetClusters {
		if cluster != fromCluster {
			rbSpecClusters = append(rbSpecClusters, workv1alpha2.TargetCluster{Name: cluster})
		}
	}

	return &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: eps.Namespace,
			Name:      names.GenerateBindingName(util.EndpointSliceKind, eps.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(eps, discoveryv1.SchemeGroupVersion.WithKind(util.EndpointSliceKind)),
			},
			Finalizers: []string{util.BindingControllerFinalizer},
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Resource: workv1alpha2.ObjectReference{
				APIVersion:      discoveryv1.SchemeGroupVersion.String(),
				Kind:            util.EndpointSliceKind,
				Namespace:       eps.GetNamespace(),
				Name:            eps.GetName(),
				UID:             eps.GetUID(),
				ResourceVersion: eps.GetResourceVersion(),
			},
			Clusters: rbSpecClusters,
		},
	}
}

func extractClusterName(epsName string) string {
	segments := strings.Split(epsName, "-")
	// according to the naming rules, select the
	// second part of the slice as the cluster name.
	return segments[1]
}

func generateNativeServiceBindingName(kind, name string) string {
	return fmt.Sprintf("native-%s", names.GenerateBindingName(kind, name))
}
