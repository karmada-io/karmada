package leaderelection

import (
	"context"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/cmd/karmada-scheduler/app/options"
)

// NewLeaderElector builds a leader elector.
func NewLeaderElector(opts *options.Options, fnStartScheduler func(*options.Options, <-chan struct{})) (*leaderelection.LeaderElector, error) {
	const component = "karmada-scheduler"
	rest.AddUserAgent(opts.KubeConfig, "leader-election")
	leaderElectionClient := kubeclient.NewForConfigOrDie(opts.KubeConfig)

	// Prepare event clients.
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: leaderElectionClient.CoreV1().Events(opts.HostNamespace)})
	eventRecorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: component})

	// add a uniquifier so that two processes on the same host don't accidentally both become active
	hostname, err := os.Hostname()
	if err != nil {
		klog.Infof("unable to get hostname: %v", err)
		return nil, err
	}
	id := hostname + "_" + string(uuid.NewUUID())

	rl, err := resourcelock.New(opts.LeaderElection.ResourceLock,
		opts.HostNamespace,
		component,
		leaderElectionClient.CoreV1(),
		leaderElectionClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: eventRecorder,
		})
	if err != nil {
		klog.Infof("couldn't create resource lock: %v", err)
		return nil, err
	}

	return leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: opts.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: opts.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   opts.LeaderElection.RetryPeriod.Duration,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Info("promoted as leader")
				stopChan := ctx.Done()
				fnStartScheduler(opts, stopChan)
				<-stopChan
			},
			OnStoppedLeading: func() {
				klog.Info("leader election lost")
			},
		},
	})
}
