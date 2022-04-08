package autopropagation

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

const (
	// ServiceControllerName is the controller name that will be used when reporting events.
	ServiceControllerName     = "service-sync-controller"
	annotationsGlobalKEYName  = "karmada.io/global"
	annotationsMembersKEYName = "karmada.io/members"
)

// ServiceController is to sync service.
type ServiceController struct {
	client.Client
	EventRecorder record.EventRecorder
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *ServiceController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	service := &corev1.Service{}
	if err := c.Client.Get(ctx, req.NamespacedName, service); err != nil {
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !service.DeletionTimestamp.IsZero() {
		// Do nothing, just return as we have added owner reference to PropagationPolicy.
		// Work will be removed automatically by garbage collector.
		return reconcile.Result{}, nil
	}

	annotations := service.GetAnnotations()
	if annotations == nil {
		klog.Warningf("service %q of namespace %q has no annotations. skip", service.Name, service.Namespace)
		return reconcile.Result{}, nil
	}

	clusterList := &clusterv1alpha1.ClusterList{}
	if err := c.Client.List(context.TODO(), clusterList); err != nil {
		klog.Errorf("Failed to list clusters, error: %v", err)
		return reconcile.Result{Requeue: true}, err
	}

	deployClusters := c.syncClusters(service.Namespace, service.Name, annotations, clusterList.Items)
	if deployClusters == nil {
		return reconcile.Result{}, nil
	}

	if err := c.buildPropagationPolicy(service, deployClusters); err != nil {
		return controllerruntime.Result{Requeue: true}, err
	}

	return reconcile.Result{}, nil
}

func (c *ServiceController) syncClusters(namespace, service string, annotations map[string]string, clusters []clusterv1alpha1.Cluster) []string {
	var deployClusters []string

	if v, ok := annotations[annotationsGlobalKEYName]; ok && v == "true" {
		klog.Infof("service %q propagation policy for namespace %q is global.", service, namespace)
		for _, cluster := range clusters {
			deployClusters = append(deployClusters, cluster.Name)
		}
		return deployClusters
	}

	v, ok := annotations[annotationsMembersKEYName]
	if !ok {
		klog.Infof("The propagation policy of service %q of namespace %q is not global, and no cluster is specified. Skip", service, namespace)
		return nil
	}

	members := strings.Split(v, ",")
	for _, cluster := range clusters {
		for _, member := range members {
			if cluster.Name == member {
				deployClusters = append(deployClusters, cluster.Name)
				continue
			}
		}
	}

	klog.V(4).Infof("member clusters: %v,valid number of deploy clusters: %v", members, deployClusters)
	return deployClusters
}

// buildPropagationPolicy create service PropagationPolicy
func (c *ServiceController) buildPropagationPolicy(service *corev1.Service, clusters []string) error {
	pp := &policyv1alpha1.PropagationPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyv1alpha1.GroupVersion.String(),
			Kind:       policyv1alpha1.ResourceKindPropagationPolicy,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(service, service.GroupVersionKind()),
			},
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: service.APIVersion,
					Kind:       service.Kind,
					Name:       service.Name,
					Namespace:  service.Namespace,
				},
			},
			Placement: policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: clusters,
				},
			},
		},
	}

	result, err := controllerutil.CreateOrUpdate(context.TODO(), c.Client, pp, func() error { return nil })
	if err != nil {
		return err
	}
	if result == controllerutil.OperationResultCreated {
		klog.Infof("Namespace %q Create PropagationPolicy %q successfully.", pp.GetNamespace(), pp.GetName())
	} else if result == controllerutil.OperationResultUpdated {
		klog.Infof("Namespace %q Update PropagationPolicy %q successfully.", pp.GetNamespace(), pp.GetName())
	} else {
		klog.V(4).Infof("Namespace %q Update PropagationPolicy %q is up to date.", pp.GetNamespace(), pp.GetName())
	}

	return nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *ServiceController) SetupWithManager(mgr controllerruntime.Manager) error {
	// Setup Scheme for k8s core/v1 resources
	if err := corev1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	// Setup Scheme for karmada cluster/v1alpha1 resources
	if err := clusterv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	// Setup Scheme for karmada policy/v1alpha1 resources
	if err := policyv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	predicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).WithEventFilter(predicate).Complete(c)
}
