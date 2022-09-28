package descheduler

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/descheduler/app/options"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/descheduler/core"
	estimatorclient "github.com/karmada-io/karmada/pkg/estimator/client"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	worklister "github.com/karmada-io/karmada/pkg/generated/listers/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

const (
	descheduleSuccessMessage = "Binding has been descheduled"
)

// Descheduler is the descheduler schema, which is used to evict replicas from specific clusters
type Descheduler struct {
	KarmadaClient   karmadaclientset.Interface
	KubeClient      kubernetes.Interface
	informerFactory informerfactory.SharedInformerFactory
	bindingInformer cache.SharedIndexInformer
	bindingLister   worklister.ResourceBindingLister
	clusterInformer cache.SharedIndexInformer
	clusterLister   clusterlister.ClusterLister

	eventRecorder record.EventRecorder

	schedulerEstimatorCache  *estimatorclient.SchedulerEstimatorCache
	schedulerEstimatorPort   int
	schedulerEstimatorWorker util.AsyncWorker

	unschedulableThreshold time.Duration
	deschedulingInterval   time.Duration
	deschedulerWorker      util.AsyncWorker
}

// NewDescheduler instantiates a descheduler
func NewDescheduler(karmadaClient karmadaclientset.Interface, kubeClient kubernetes.Interface, opts *options.Options) *Descheduler {
	factory := informerfactory.NewSharedInformerFactory(karmadaClient, 0)
	desched := &Descheduler{
		KarmadaClient:           karmadaClient,
		KubeClient:              kubeClient,
		informerFactory:         factory,
		bindingInformer:         factory.Work().V1alpha2().ResourceBindings().Informer(),
		bindingLister:           factory.Work().V1alpha2().ResourceBindings().Lister(),
		clusterInformer:         factory.Cluster().V1alpha1().Clusters().Informer(),
		clusterLister:           factory.Cluster().V1alpha1().Clusters().Lister(),
		schedulerEstimatorCache: estimatorclient.NewSchedulerEstimatorCache(),
		schedulerEstimatorPort:  opts.SchedulerEstimatorPort,
		unschedulableThreshold:  opts.UnschedulableThreshold.Duration,
		deschedulingInterval:    opts.DeschedulingInterval.Duration,
	}
	// ignore the error here because the informers haven't been started
	_ = desched.bindingInformer.SetTransform(fedinformer.StripUnusedFields)
	_ = desched.clusterInformer.SetTransform(fedinformer.StripUnusedFields)

	schedulerEstimatorWorkerOptions := util.Options{
		Name:          "scheduler-estimator",
		KeyFunc:       nil,
		ReconcileFunc: desched.reconcileEstimatorConnection,
	}
	desched.schedulerEstimatorWorker = util.NewAsyncWorker(schedulerEstimatorWorkerOptions)
	schedulerEstimator := estimatorclient.NewSchedulerEstimator(desched.schedulerEstimatorCache, opts.SchedulerEstimatorTimeout.Duration)
	estimatorclient.RegisterSchedulerEstimator(schedulerEstimator)
	deschedulerWorkerOptions := util.Options{
		Name:          "descheduler",
		KeyFunc:       util.MetaNamespaceKeyFunc,
		ReconcileFunc: desched.worker,
	}
	desched.deschedulerWorker = util.NewAsyncWorker(deschedulerWorkerOptions)

	desched.clusterInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    desched.addCluster,
		UpdateFunc: desched.updateCluster,
		DeleteFunc: desched.deleteCluster,
	})

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	desched.eventRecorder = eventBroadcaster.NewRecorder(gclient.NewSchema(), corev1.EventSource{Component: "karmada-descheduler"})

	return desched
}

// Run runs the scheduler
func (d *Descheduler) Run(ctx context.Context) {
	stopCh := ctx.Done()
	klog.Infof("Starting karmada descheduler")
	defer klog.Infof("Shutting down karmada descheduler")

	// Establish all connections first and then begin scheduling.
	d.establishEstimatorConnections()
	d.schedulerEstimatorWorker.Run(1, stopCh)

	d.informerFactory.Start(stopCh)

	if !cache.WaitForCacheSync(stopCh, d.bindingInformer.HasSynced, d.clusterInformer.HasSynced) {
		klog.Errorf("Failed to wait for cache sync")
	}

	go wait.Until(d.descheduleOnce, d.deschedulingInterval, stopCh)
	d.deschedulerWorker.Run(1, stopCh)

	<-stopCh
}

func (d *Descheduler) descheduleOnce() {
	bindings, err := d.bindingLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("List all ResourceBindings error: %v", err)
	}
	bindings = core.FilterBindings(bindings)
	for _, binding := range bindings {
		d.deschedulerWorker.Enqueue(binding)
	}
}

func (d *Descheduler) worker(key util.QueueKey) error {
	namespacedName, ok := key.(string)
	if !ok {
		return fmt.Errorf("failed to deschedule as invalid key: %v", key)
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(namespacedName)
	if err != nil {
		return fmt.Errorf("invalid resource key: %s", namespacedName)
	}

	binding, err := d.bindingLister.ResourceBindings(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("ResourceBinding(%s) in work queue no longer exists, ignore.", namespacedName)
			return nil
		}
		return fmt.Errorf("get ResourceBinding(%s) error: %v", namespacedName, err)
	}

	h := core.NewSchedulingResultHelper(binding)
	if _, undesiredClusters := h.GetUndesiredClusters(); len(undesiredClusters) == 0 {
		return nil
	}

	h.FillUnschedulableReplicas(d.unschedulableThreshold)
	klog.V(3).Infof("Unschedulable result of resource(%s): %v", namespacedName, h.TargetClusters)

	return d.updateScheduleResult(h)
}

func (d *Descheduler) updateScheduleResult(h *core.SchedulingResultHelper) error {
	unschedulableSum := int32(0)
	message := descheduleSuccessMessage
	binding := h.ResourceBinding.DeepCopy()
	for i, cluster := range h.TargetClusters {
		if cluster.Unschedulable > 0 && cluster.Spec >= cluster.Unschedulable {
			target := cluster.Spec - cluster.Unschedulable
			// The target cluster replicas must not be less than ready replicas.
			if target < cluster.Ready && cluster.Ready <= cluster.Spec {
				target = cluster.Ready
			}
			binding.Spec.Clusters[i].Replicas = target
			unschedulable := cluster.Spec - target
			unschedulableSum += unschedulable
			message += fmt.Sprintf(", %d replica(s) in cluster(%s)", unschedulable, cluster.ClusterName)
		}
	}
	if unschedulableSum == 0 {
		return nil
	}
	message = fmt.Sprintf(", %d total descheduled replica(s)", unschedulableSum) + message

	var err error
	defer func() {
		d.recordDescheduleResultEventForResourceBinding(binding, message, err)
	}()

	binding, err = d.KarmadaClient.WorkV1alpha2().ResourceBindings(binding.Namespace).Update(context.TODO(), binding, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (d *Descheduler) addCluster(obj interface{}) {
	cluster, ok := obj.(*clusterv1alpha1.Cluster)
	if !ok {
		klog.Errorf("Cannot convert to Cluster: %v", obj)
		return
	}
	klog.V(4).Infof("Receiving add event for cluster %s", cluster.Name)
	d.schedulerEstimatorWorker.Add(cluster.Name)
}

func (d *Descheduler) updateCluster(_, newObj interface{}) {
	cluster, ok := newObj.(*clusterv1alpha1.Cluster)
	if !ok {
		klog.Errorf("Cannot convert to Cluster: %v", newObj)
		return
	}
	klog.V(4).Infof("Receiving update event for cluster %s", cluster.Name)
	d.schedulerEstimatorWorker.Add(cluster.Name)
}

func (d *Descheduler) deleteCluster(obj interface{}) {
	var cluster *clusterv1alpha1.Cluster
	switch t := obj.(type) {
	case *clusterv1alpha1.Cluster:
		cluster = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		cluster, ok = t.Obj.(*clusterv1alpha1.Cluster)
		if !ok {
			klog.Errorf("Cannot convert to clusterv1alpha1.Cluster: %v", t.Obj)
			return
		}
	default:
		klog.Errorf("Cannot convert to clusterv1alpha1.Cluster: %v", t)
		return
	}
	klog.V(4).Infof("Receiving delete event for cluster %s", cluster.Name)
	d.schedulerEstimatorWorker.Add(cluster.Name)
}

func (d *Descheduler) establishEstimatorConnections() {
	clusterList, err := d.KarmadaClient.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Cannot list all clusters when establish all cluster estimator connections: %v", err)
		return
	}
	for i := range clusterList.Items {
		if err = estimatorclient.EstablishConnection(d.KubeClient, clusterList.Items[i].Name, d.schedulerEstimatorCache, d.schedulerEstimatorPort); err != nil {
			klog.Error(err)
		}
	}
}

func (d *Descheduler) reconcileEstimatorConnection(key util.QueueKey) error {
	name, ok := key.(string)
	if !ok {
		return fmt.Errorf("failed to reconcile estimator connection as invalid key: %v", key)
	}

	_, err := d.clusterLister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			d.schedulerEstimatorCache.DeleteCluster(name)
			return nil
		}
		return err
	}
	return estimatorclient.EstablishConnection(d.KubeClient, name, d.schedulerEstimatorCache, d.schedulerEstimatorPort)
}

func (d *Descheduler) recordDescheduleResultEventForResourceBinding(rb *workv1alpha2.ResourceBinding, message string, err error) {
	if rb == nil {
		return
	}

	ref := &corev1.ObjectReference{
		Kind:       rb.Spec.Resource.Kind,
		APIVersion: rb.Spec.Resource.APIVersion,
		Namespace:  rb.Spec.Resource.Namespace,
		Name:       rb.Spec.Resource.Name,
		UID:        rb.Spec.Resource.UID,
	}
	if err == nil {
		d.eventRecorder.Event(rb, corev1.EventTypeNormal, workv1alpha2.EventReasonDescheduleBindingSucceed, message)
		d.eventRecorder.Event(ref, corev1.EventTypeNormal, workv1alpha2.EventReasonDescheduleBindingSucceed, message)
	} else {
		d.eventRecorder.Event(rb, corev1.EventTypeWarning, workv1alpha2.EventReasonDescheduleBindingFailed, err.Error())
		d.eventRecorder.Event(ref, corev1.EventTypeWarning, workv1alpha2.EventReasonDescheduleBindingFailed, err.Error())
	}
}
