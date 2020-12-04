package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime"

	"github.com/huawei-cloudnative/karmada/cmd/controller-manager/app/leaderelection"
	"github.com/huawei-cloudnative/karmada/cmd/controller-manager/app/options"
	memberclusterv1alpha1 "github.com/huawei-cloudnative/karmada/pkg/apis/membercluster/v1alpha1"
	propagationv1alpha1 "github.com/huawei-cloudnative/karmada/pkg/apis/propagationstrategy/v1alpha1"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/binding"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/execution"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/membercluster"
	"github.com/huawei-cloudnative/karmada/pkg/controllers/policy"
	karmadaclientset "github.com/huawei-cloudnative/karmada/pkg/generated/clientset/versioned"
)

// aggregatedScheme aggregates all Kubernetes and extended schemes used by controllers.
var aggregatedScheme = runtime.NewScheme()

func init() {
	var _ = scheme.AddToScheme(aggregatedScheme)                // add Kubernetes schemes
	var _ = propagationv1alpha1.AddToScheme(aggregatedScheme)   // add propagation schemes
	var _ = memberclusterv1alpha1.AddToScheme(aggregatedScheme) // add membercluster schemes
}

// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand(stopChan <-chan struct{}) *cobra.Command {
	verFlag := false
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "controller-manager",
		Long: `The controller manager runs a bunch of controllers`,
		Run: func(cmd *cobra.Command, args []string) {
			if verFlag {
				os.Exit(0)
			}

			opts.Complete()
			if err := Run(opts, stopChan); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cmd.Flags().BoolVar(&verFlag, "version", false, "Prints the version info of controller manager.")

	return cmd
}

// Run runs the controller-manager with options. This should never exit.
func Run(opts *options.Options, stopChan <-chan struct{}) error {
	logs.InitLogs()
	defer logs.FlushLogs()

	var err error
	// TODO(RainbowMango): need to change to shim kube-apiserver config.
	opts.KubeConfig, err = clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		panic(err)
	}

	controllerManager, err := controllerruntime.NewManager(opts.KubeConfig, controllerruntime.Options{
		Scheme:           aggregatedScheme,
		LeaderElection:   true, // TODO(RainbowMango): Add a flag '--enable-leader-election' for this option.
		LeaderElectionID: "41db11fa.karmada.io",
	})
	if err != nil {
		klog.Fatalf("failed to build controller manager: %v", err)
	}

	// TODO: To be removed.
	startControllers(opts, stopChan)

	setupControllers(controllerManager)

	if err := controllerManager.Start(stopChan); err != nil {
		klog.Fatalf("controller manager exits unexpectedly: %v", err)
	}

	if len(opts.HostNamespace) == 0 {
		// For in-cluster deployment set the namespace associated with the service account token
		data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			klog.Fatalf("An error occurred while attempting to discover the namespace from the service account: %v", err)
		}
		opts.HostNamespace = strings.TrimSpace(string(data))
	}

	// Validate if the namespace is configured
	if len(opts.HostNamespace) == 0 {
		klog.Fatalf("The namespace must be specified")
	}

	elector, err := leaderelection.NewLeaderElector(opts, startControllers)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-stopChan:
			cancel()
		case <-ctx.Done():
		}
	}()

	elector.Run(ctx)

	klog.Errorf("lost lease")
	return errors.New("lost lease")
}

func startControllers(opts *options.Options, stopChan <-chan struct{}) {

}

func setupControllers(mgr controllerruntime.Manager) {
	resetConfig := mgr.GetConfig()
	dynamicClientSet := dynamic.NewForConfigOrDie(resetConfig)
	karmadaClient := karmadaclientset.NewForConfigOrDie(resetConfig)
	kubeClientSet := kubernetes.NewForConfigOrDie(resetConfig)

	MemberClusterController := &membercluster.Controller{
		Client:        mgr.GetClient(),
		KubeClientSet: kubeClientSet,
		EventRecorder: mgr.GetEventRecorderFor(membercluster.ControllerName),
	}
	if err := MemberClusterController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup membercluster controller: %v", err)
	}

	policyController := &policy.PropagationPolicyController{
		Client:        mgr.GetClient(),
		DynamicClient: dynamicClientSet,
		KarmadaClient: karmadaClient,
		EventRecorder: mgr.GetEventRecorderFor(policy.ControllerName),
	}
	if err := policyController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup policy controller: %v", err)
	}

	bindingController := &binding.PropagationBindingController{
		Client:        mgr.GetClient(),
		DynamicClient: dynamicClientSet,
		KarmadaClient: karmadaClient,
		EventRecorder: mgr.GetEventRecorderFor(binding.ControllerName),
	}
	if err := bindingController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup binding controller: %v", err)
	}

	executionController := &execution.Controller{
		Client:        mgr.GetClient(),
		KubeClientSet: kubeClientSet,
		KarmadaClient: karmadaClient,
		EventRecorder: mgr.GetEventRecorderFor(execution.ControllerName),
	}
	if err := executionController.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Failed to setup execution controller: %v", err)
	}

}
