package clusterapi

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	clusterapiv1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/karmadactl"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	clusterGVRs = []schema.GroupVersionResource{
		{Group: clusterapiv1alpha4.GroupVersion.Group, Version: clusterapiv1alpha4.GroupVersion.Version, Resource: "clusters"},
	}
)

// ClusterDetector is a cluster watcher which watched cluster object in cluster-api management cluster and reconcile the events.
type ClusterDetector struct {
	KarmadaConfig         karmadactl.KarmadaConfig
	ControllerPlaneConfig *rest.Config
	ClusterAPIConfig      *rest.Config
	ClusterAPIClient      client.Client
	InformerManager       informermanager.SingleClusterInformerManager
	EventHandler          cache.ResourceEventHandler
	Processor             util.AsyncWorker

	stopCh <-chan struct{}
}

// Start runs the detector, never stop until stopCh closed.
func (d *ClusterDetector) Start(ctx context.Context) error {
	klog.Infof("Starting cluster-api cluster detector.")
	d.stopCh = ctx.Done()

	d.EventHandler = informermanager.NewHandlerOnEvents(d.OnAdd, d.OnUpdate, d.OnDelete)
	d.Processor = util.NewAsyncWorker("cluster-api cluster detector", time.Second, ClusterWideKeyFunc, d.Reconcile)
	d.Processor.Run(1, d.stopCh)
	d.discoveryCluster()

	<-d.stopCh
	klog.Infof("Stopped as stopCh closed.")
	return nil
}

func (d *ClusterDetector) discoveryCluster() {
	for _, gvr := range clusterGVRs {
		if !d.InformerManager.IsHandlerExist(gvr, d.EventHandler) {
			klog.Infof("Setup informer fo %s", gvr.String())
			d.InformerManager.ForResource(gvr, d.EventHandler)
		}
	}

	d.InformerManager.Start()
}

// OnAdd handles object add event and push the object to queue.
func (d *ClusterDetector) OnAdd(obj interface{}) {
	runtimeObj, ok := obj.(runtime.Object)
	if !ok {
		return
	}
	d.Processor.EnqueueRateLimited(runtimeObj)
}

// OnUpdate handles object update event and push the object to queue.
func (d *ClusterDetector) OnUpdate(oldObj, newObj interface{}) {
	d.OnAdd(newObj)
}

// OnDelete handles object delete event and push the object to queue.
func (d *ClusterDetector) OnDelete(obj interface{}) {
	d.OnAdd(obj)
}

// Reconcile performs a full reconciliation for the object referred to by the key.
// The key will be re-queued if an error is non-nil.
func (d *ClusterDetector) Reconcile(key util.QueueKey) error {
	clusterWideKey, ok := key.(keys.ClusterWideKey)
	if !ok {
		klog.Errorf("invalid key")
		return fmt.Errorf("invalid key")
	}
	klog.Infof("Reconciling cluster-api object: %s", clusterWideKey)

	object, err := d.GetUnstructuredObject(clusterWideKey)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return d.unJoinClusterAPICluster(clusterWideKey.Name)
		}
		klog.Errorf("Failed to get unstructured object(%s), error: %v", clusterWideKey, err)
		return err
	}

	clusterPhase, ok, err := unstructured.NestedString(object.Object, "status", "phase")
	if err != nil {
		klog.Errorf("Failed to retrieving status phase from cluster: %v", err)
		return err
	}

	if ok && clusterPhase == string(clusterapiv1alpha4.ClusterPhaseProvisioned) {
		return d.joinClusterAPICluster(clusterWideKey.Name)
	}

	return nil
}

// GetUnstructuredObject retrieves object by key and returned its unstructured.
func (d *ClusterDetector) GetUnstructuredObject(objectKey keys.ClusterWideKey) (*unstructured.Unstructured, error) {
	objectGVR := schema.GroupVersionResource{
		Group:    objectKey.Group,
		Version:  objectKey.Version,
		Resource: "clusters",
	}

	object, err := d.InformerManager.Lister(objectGVR).Get(objectKey.NamespaceKey())
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Errorf("Failed to get object(%s), error: %v", objectKey, err)
		}
		return nil, err
	}

	uncastObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		klog.Errorf("Failed to transform object(%s), error: %v", objectKey, err)
		return nil, err
	}

	return &unstructured.Unstructured{Object: uncastObj}, nil
}

func (d *ClusterDetector) joinClusterAPICluster(clusterName string) error {
	klog.Infof("Begin to join cluster-api's Cluster(%s) to karmada", clusterName)

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: metav1.NamespaceDefault,
		Name:      names.GenerateClusterAPISecretName(clusterName),
	}
	err := d.ClusterAPIClient.Get(context.TODO(), secretKey, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Errorf("Can not found secret(%s): %v", secretKey.String(), err)
		} else {
			klog.Errorf("Failed to get secret(%s): %v", secretKey.String(), err)
		}
		return err
	}

	kubeconfigPath, err := generateKubeconfigFile(clusterName, secret.Data["value"])
	if err != nil {
		return err
	}

	clusterRestConfig, err := d.KarmadaConfig.GetRestConfig("", kubeconfigPath)
	if err != nil {
		klog.Fatalf("Failed to get cluster-api management cluster rest config. kubeconfig: %s, err: %v", kubeconfigPath, err)
	}

	err = karmadactl.JoinCluster(d.ControllerPlaneConfig, clusterRestConfig, options.DefaultKarmadaClusterNamespace, clusterName, false)
	if err != nil {
		klog.Errorf("Failed to join cluster-api's cluster(%s): %v", clusterName, err)
		return err
	}

	klog.Infof("End to join cluster-api's Cluster(%s) to karmada", clusterName)
	return nil
}

func (d *ClusterDetector) unJoinClusterAPICluster(clusterName string) error {
	klog.Infof("Begin to unJoin cluster-api's Cluster(%s) to karmada", clusterName)
	err := karmadactl.UnJoinCluster(d.ControllerPlaneConfig, nil, options.DefaultKarmadaClusterNamespace, clusterName, false, false)
	if err != nil {
		klog.Errorf("Failed to unJoin cluster-api's cluster(%s): %v", clusterName, err)
		return err
	}

	klog.Infof("End to unJoin cluster-api's Cluster(%s) to karmada", clusterName)
	return nil
}

func generateKubeconfigFile(clusterName string, kubeconfigData []byte) (string, error) {
	kubeconfigPath := fmt.Sprintf("/etc/%s.kubeconfig", clusterName)
	err := ioutil.WriteFile(kubeconfigPath, kubeconfigData, 0600)
	if err != nil {
		klog.Errorf("Failed to write File %s: %v", kubeconfigPath, err)
		return "", err
	}

	return kubeconfigPath, nil
}
