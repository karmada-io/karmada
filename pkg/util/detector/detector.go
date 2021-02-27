package detector

import (
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// ResourceDetector is a resource watcher which watches all resources and reconcile the events.
type ResourceDetector struct {
	ClientSet       kubernetes.Interface
	InformerManager informermanager.SingleClusterInformerManager
	EventHandler    cache.ResourceEventHandler
	Processor       util.AsyncWorker
	stopCh          <-chan struct{}
}

// Start runs the detector, never stop until stopCh closed.
func (d *ResourceDetector) Start(stopCh <-chan struct{}) error {
	klog.Infof("Starting resource detector.")
	d.stopCh = stopCh

	d.Processor.Run(1, stopCh)
	go d.discoverResources(30 * time.Second)

	<-stopCh
	klog.Infof("Stopped as stopCh closed.")
	return nil
}

// Check if our ResourceDetector implements necessary interfaces
var _ manager.Runnable = &ResourceDetector{}
var _ manager.LeaderElectionRunnable = &ResourceDetector{}

func (d *ResourceDetector) discoverResources(period time.Duration) {
	wait.Until(func() {
		newResources := GetDeletableResources(d.ClientSet.Discovery())
		for r := range newResources {
			if d.InformerManager.IsHandlerExist(r, d.EventHandler) {
				continue
			}
			klog.Infof("Setup informer for %s", r.String())
			d.InformerManager.ForResource(r, d.EventHandler)
		}
		d.InformerManager.Start(d.stopCh)
	}, period, d.stopCh)
}

// NeedLeaderElection implements LeaderElectionRunnable interface.
// So that the detector could run in the leader election mode.
func (d *ResourceDetector) NeedLeaderElection() bool {
	return true
}

// Reconcile performs a full reconciliation for the object referred to by the key.
// The key will be re-queued if an error is non-nil.
func (d *ResourceDetector) Reconcile(key util.QueueKey) error {
	klog.Infof("Syncing %s", key)
	// TODO(RainbowMango): implement later.
	return nil
}

// EventFilter tells if an object should be take care of.
//
// All objects under Kubernetes reserved namespace should be ignored:
// - kube-system
// - kube-public
// - kube-node-lease
// All objects under Karmada reserved namespace should be ignored:
// - karmada-system
// - karmada-cluster
// - karmada-es-*
// All objects which API group defined by Karmada should be ignored:
// - cluster.karmada.io
// - policy.karmada.io
func (d *ResourceDetector) EventFilter(obj interface{}) bool {
	key, err := ClusterWideKeyFunc(obj)
	if err != nil {
		return false
	}

	clusterWideKey, ok := key.(ClusterWideKey)
	if !ok {
		klog.Errorf("Invalid key")
		return false
	}

	if strings.HasPrefix(clusterWideKey.Namespace, names.KubernetesReservedNSPrefix) ||
		strings.HasPrefix(clusterWideKey.Namespace, names.KarmadaReservedNSPrefix) {
		return false
	}

	if clusterWideKey.GVK.Group == clusterv1alpha1.GroupName ||
		clusterWideKey.GVK.Group == policyv1alpha1.GroupName {
		return false
	}

	return true
}

// OnAdd handles object add event and push the object to queue.
func (d *ResourceDetector) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	d.Processor.EnqueueRateLimited(runtimeObj)
}

// OnUpdate handles object update event and push the object to queue.
func (d *ResourceDetector) OnUpdate(oldObj, newObj interface{}) {
	d.OnAdd(newObj)
}

// OnDelete handles object delete event and push the object to queue.
func (d *ResourceDetector) OnDelete(obj interface{}) {
	d.OnAdd(obj)
}
