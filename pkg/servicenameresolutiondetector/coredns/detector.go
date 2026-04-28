/*
Copyright 2024 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package coredns

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog/v2"

	karmada "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/servicenameresolutiondetector/store"
)

const (
	name                              = "coredns-detector"
	condType                          = "ServiceDomainNameResolutionReady"
	serviceDomainNameResolutionReady  = "ServiceDomainNameResolutionReady"
	serviceDomainNameResolutionFailed = "ServiceDomainNameResolutionFailed"
)

var localReference = &corev1.ObjectReference{
	APIVersion: "v1",
	Kind:       "Pod",
	Name:       os.Getenv("POD_NAME"),
	Namespace:  os.Getenv("POD_NAMESPACE"),
}

// Config contains config of coredns detector.
type Config struct {
	Period           time.Duration
	SuccessThreshold time.Duration
	FailureThreshold time.Duration
	StaleThreshold   time.Duration
}

// Detector detects DNS failure and syncs conditions to control plane periodically.
type Detector struct {
	memberClusterClient kubernetes.Interface
	karmadaClient       karmada.Interface

	lec leaderelection.LeaderElectionConfig

	period time.Duration

	conditionCache store.ConditionCache
	conditionStore store.ConditionStore
	cacheSynced    []cache.InformerSynced

	nodeName    string
	clusterName string

	queue            workqueue.TypedRateLimitingInterface[any]
	eventBroadcaster record.EventBroadcaster
	eventRecorder    record.EventRecorder
}

// NewCorednsDetector returns an instance of coredns detector.
func NewCorednsDetector(memberClusterClient kubernetes.Interface, karmadaClient karmada.Interface, informers informers.SharedInformerFactory,
	baselec componentbaseconfig.LeaderElectionConfiguration, cfg *Config, hostName, clusterName string) (*Detector, error) {
	broadcaster := record.NewBroadcaster()
	recorder := broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: name})

	var rl resourcelock.Interface
	var err error
	if baselec.LeaderElect {
		rl, err = resourcelock.New(
			baselec.ResourceLock,
			baselec.ResourceNamespace,
			baselec.ResourceName+"-"+name,
			memberClusterClient.CoreV1(),
			memberClusterClient.CoordinationV1(),
			resourcelock.ResourceLockConfig{Identity: hostName},
		)
		if err != nil {
			return nil, err
		}
	}

	nodeInformer := informers.Core().V1().Nodes()
	return &Detector{
		memberClusterClient: memberClusterClient,
		karmadaClient:       karmadaClient,
		nodeName:            hostName,
		clusterName:         clusterName,
		period:              cfg.Period,
		conditionCache:      store.NewConditionCache(cfg.SuccessThreshold, cfg.FailureThreshold),
		conditionStore:      store.NewNodeConditionStore(memberClusterClient.CoreV1().Nodes(), nodeInformer.Lister(), condType, cfg.StaleThreshold),
		cacheSynced:         []cache.InformerSynced{nodeInformer.Informer().HasSynced},
		eventBroadcaster:    broadcaster,
		eventRecorder:       recorder,
		queue:               workqueue.NewTypedRateLimitingQueueWithConfig(workqueue.DefaultTypedControllerRateLimiter[any](), workqueue.TypedRateLimitingQueueConfig[any]{Name: name}),
		lec: leaderelection.LeaderElectionConfig{
			Lock:          rl,
			LeaseDuration: baselec.LeaseDuration.Duration,
			RenewDeadline: baselec.RenewDeadline.Duration,
			RetryPeriod:   baselec.RetryPeriod.Duration,
		}}, nil
}

// Run starts the detector.
func (d *Detector) Run(ctx context.Context) {
	defer runtime.HandleCrash()

	d.eventBroadcaster.StartStructuredLogging(0)
	d.eventBroadcaster.StartRecordingToSink(&v1.EventSinkImpl{Interface: d.memberClusterClient.CoreV1().Events("")})
	defer d.eventBroadcaster.Shutdown()

	defer d.queue.ShutDown()

	logger := klog.FromContext(ctx)
	logger.Info("Starting coredns detector")
	defer logger.Info("Shutting down coredns detector")

	if !cache.WaitForCacheSync(ctx.Done(), d.cacheSynced...) {
		return
	}

	go func() {
		wait.Until(func() {
			defer runtime.HandleCrash()

			observed := lookupOnce(logger)
			curr, err := d.conditionStore.Load(d.nodeName)
			if err != nil {
				d.eventRecorder.Eventf(localReference, corev1.EventTypeWarning, "LoadCorednsConditionFailed", "failed to load condition: %v", err)
				return
			}

			cond := d.conditionCache.ThresholdAdjustedCondition(d.nodeName, curr, observed)
			if err = d.conditionStore.Store(d.nodeName, cond); err != nil {
				d.eventRecorder.Eventf(localReference, corev1.EventTypeWarning, "StoreCorednsConditionFailed", "failed to store condition: %v", err)
				return
			}
			d.queue.Add(0)
		}, d.period, ctx.Done())
	}()

	if d.lec.Lock != nil {
		d.lec.Callbacks = leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				wait.UntilWithContext(ctx, d.worker, time.Second)
			},
			OnStoppedLeading: func() {
				logger.Error(nil, "leader election lost")
				klog.FlushAndExit(klog.ExitFlushTimeout, 1)
			},
		}
		leaderelection.RunOrDie(ctx, d.lec)
	} else {
		wait.UntilWithContext(ctx, d.worker, time.Second)
	}
}

func (d *Detector) worker(ctx context.Context) {
	for d.processNextWorkItem(ctx) {
	}
}

func (d *Detector) processNextWorkItem(ctx context.Context) bool {
	key, quit := d.queue.Get()
	if quit {
		return false
	}
	defer d.queue.Done(key)

	if err := d.sync(ctx); err != nil {
		runtime.HandleError(fmt.Errorf("failed to sync corendns condition to control plane, requeuing: %v", err))
		d.queue.AddRateLimited(key)
	} else {
		d.queue.Forget(key)
	}
	return true
}

func lookupOnce(logger klog.Logger) *metav1.Condition {
	logger.Info("lookup service name once")
	observed := &metav1.Condition{Type: condType, LastTransitionTime: metav1.Now()}
	if _, err := net.LookupHost("kubernetes.default"); err != nil {
		logger.Error(err, "nslookup failed")
		observed.Status = metav1.ConditionFalse
		observed.Reason = serviceDomainNameResolutionFailed
		observed.Message = err.Error()
	} else {
		observed.Status = metav1.ConditionTrue
	}
	return observed
}

func (d *Detector) sync(ctx context.Context) error {
	logger := klog.FromContext(ctx)
	skip, alarm, err := d.shouldAlarm()
	if err != nil {
		return err
	}
	if skip {
		logger.Info("skip syncing to control plane since nodes with condition Unknown")
		return nil
	}

	cond, err := d.newClusterCondition(alarm)
	if err != nil {
		return err
	}

	cluster, err := d.karmadaClient.ClusterV1alpha1().Clusters().Get(ctx, d.clusterName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("cluster %s not found, skip sync coredns condition", d.clusterName))
			return nil
		}
		return err
	}
	if !cluster.DeletionTimestamp.IsZero() {
		logger.Info(fmt.Sprintf("cluster %s is deleting, skip sync coredns condition", d.clusterName))
		return nil
	}
	meta.SetStatusCondition(&cluster.Status.Conditions, *cond)
	_, err = d.karmadaClient.ClusterV1alpha1().Clusters().UpdateStatus(ctx, cluster, metav1.UpdateOptions{})
	return err
}

func (d *Detector) newClusterCondition(alarm bool) (*metav1.Condition, error) {
	cond := &metav1.Condition{Type: condType}
	if alarm {
		cond.Status = metav1.ConditionFalse
		cond.Reason = serviceDomainNameResolutionFailed
		cond.Message = "service domain name resolution is unready"
	} else {
		cond.Status = metav1.ConditionTrue
		cond.Reason = serviceDomainNameResolutionReady
		cond.Message = "service domain name resolution is ready"
	}
	return cond, nil
}

func (d *Detector) shouldAlarm() (skip bool, alarm bool, err error) {
	conditions, err := d.conditionStore.ListAll()
	if err != nil {
		return false, false, err
	}
	if len(conditions) == 0 {
		return true, false, nil
	}

	hasUnknown, allFalse := false, true
	for _, cond := range conditions {
		switch cond.Status {
		case metav1.ConditionUnknown:
			hasUnknown = true
		case metav1.ConditionFalse:
		default:
			allFalse = false
		}
	}
	return hasUnknown, allFalse, nil
}
