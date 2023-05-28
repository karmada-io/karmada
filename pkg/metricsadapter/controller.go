package metricsadapter

import (
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	clusterV1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/metricsadapter/provider"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

// MetricsController is a controller for metrics, control the lifecycle of multi-clusters informer
type MetricsController struct {
	InformerFactory informerfactory.SharedInformerFactory
	ClusterLister   clusterlister.ClusterLister
	InformerManager genericmanager.MultiClusterInformerManager

	queue      workqueue.RateLimitingInterface
	restConfig *rest.Config
}

// NewMetricsController creates a new metrics controller
func NewMetricsController(restConfig *rest.Config, factory informerfactory.SharedInformerFactory) *MetricsController {
	clusterLister := factory.Cluster().V1alpha1().Clusters().Lister()
	controller := &MetricsController{
		InformerFactory: factory,
		ClusterLister:   clusterLister,
		InformerManager: genericmanager.GetInstance(),
		restConfig:      restConfig,
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "metrics-adapter"),
	}
	controller.addEventHandler()

	return controller
}

// addEventHandler adds event handler for cluster
func (m *MetricsController) addEventHandler() {
	clusterInformer := m.InformerFactory.Cluster().V1alpha1().Clusters().Informer()
	_, err := clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: m.addCluster,
		// Update event and delete event will be handled by the same handler
		UpdateFunc: m.updateCluster,
	})
	if err != nil {
		klog.Errorf("Failed to add cluster event handler for cluster: %v", err)
	}
}

// addCluster adds cluster to queue
func (m *MetricsController) addCluster(obj interface{}) {
	cluster := obj.(*clusterV1alpha1.Cluster)
	m.queue.Add(cluster.GetName())
}

// updateCluster updates cluster in queue
func (m *MetricsController) updateCluster(oldObj, curObj interface{}) {
	curCluster := curObj.(*clusterV1alpha1.Cluster)
	oldCluster := oldObj.(*clusterV1alpha1.Cluster)
	if curCluster.ResourceVersion == oldCluster.ResourceVersion {
		// no change, do nothing.
		return
	}

	if oldCluster.DeletionTimestamp.IsZero() != curCluster.DeletionTimestamp.IsZero() {
		// cluster is being deleted.
		m.queue.Add(curCluster.GetName())
	}

	if util.ClusterAccessCredentialChanged(curCluster.Spec, oldCluster.Spec) ||
		util.IsClusterReady(&curCluster.Status) != util.IsClusterReady(&oldCluster.Status) {
		// Cluster.Spec or Cluster health state is changed, rebuild informer.
		m.InformerManager.Stop(curCluster.GetName())
		m.queue.Add(curCluster.GetName())
	}
}

// startController starts controller
func (m *MetricsController) startController(stopCh <-chan struct{}) {
	m.InformerFactory.WaitForCacheSync(stopCh)

	go wait.Until(m.worker, time.Second, stopCh)

	go func() {
		<-stopCh
		genericmanager.StopInstance()
		klog.Infof("Shutting down karmada-metrics-adapter")
	}()
}

// worker is a worker for handle the data in queue
func (m *MetricsController) worker() {
	for m.handleClusters() {
	}
}

// handleClusters handles clusters changes
func (m *MetricsController) handleClusters() bool {
	key, shutdown := m.queue.Get()
	if shutdown {
		klog.Errorf("Fail to pop item from queue")
		return false
	}
	defer m.queue.Done(key)

	clusterName := key.(string)
	cls, err := m.ClusterLister.Get(clusterName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("try to stop cluster informer %s", clusterName)
			m.InformerManager.Stop(clusterName)
			return true
		}
		return false
	}

	if !cls.DeletionTimestamp.IsZero() {
		klog.Infof("try to stop cluster informer %s", clusterName)
		m.InformerManager.Stop(clusterName)
		return true
	}

	if !util.IsClusterReady(&cls.Status) {
		klog.Warningf("cluster %s is notReady try to stop this cluster informer", clusterName)
		m.InformerManager.Stop(clusterName)
		return false
	}

	if !m.InformerManager.IsManagerExist(clusterName) {
		klog.Info("Try to build informer manager for cluster ", clusterName)
		controlPlaneClient := gclient.NewForConfigOrDie(m.restConfig)
		clusterDynamicClient, err := util.NewClusterDynamicClientSet(clusterName, controlPlaneClient)
		if err != nil {
			return false
		}
		_ = m.InformerManager.ForCluster(clusterName, clusterDynamicClient.DynamicClientSet, 0)
	}
	sci := m.InformerManager.GetSingleClusterManager(clusterName)

	// Just trigger the informer to work
	_ = sci.Lister(provider.PodsGVR)
	_ = sci.Lister(provider.NodesGVR)

	sci.Start()
	_ = sci.WaitForCacheSync()

	return true
}
