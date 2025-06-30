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

package rebalance

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/ptr"

	appsv1alpha1 "github.com/karmada-io/karmada/pkg/apis/apps/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	rebalanceLong = templates.LongDesc(`
	Actively triggering a fresh rescheduling, which disregards the previous assignment entirely 
    and seeks to establish an entirely new replica distribution across clusters.
	`)

	rebalanceExample = templates.Examples(`
		# Rebalance deployment(default/nginx)
		%[1]s rebalance deployment nginx -n default

		# Rebalance deployment(default/nginx) with gvk
		%[1]s rebalance deployment.v1.apps nginx -n default`)
)

const (
	rebalancerByCtl        = "rebalancer-by-ctl"
	checkInterval          = 1 * time.Second
	checkTimeout           = 3 * time.Minute
	rebalanceAutoDeleteTTL = 30 * 60 // default ttl 30 minutes
)

// NewCmdRebalance defines the `rebalance` command that rebalance resources to achieve a fresh rescheduling.
func NewCmdRebalance(f util.Factory, parentCommand string) *cobra.Command {
	opts := CommandRebalanceOption{}

	cmd := &cobra.Command{
		Use:                   "rebalance <RESOURCE_TYPE> <RESOURCE_NAME> -n <NAME_SPACE>",
		Short:                 "rebalance resources to achieve a fresh rescheduling",
		Long:                  rebalanceLong,
		Example:               fmt.Sprintf(rebalanceExample, parentCommand),
		SilenceUsage:          true,
		ValidArgsFunction:     utilcomp.ResourceTypeAndNameCompletionFunc(f),
		DisableFlagsInUseLine: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := opts.Complete(f, args); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Run(f, args); err != nil {
				return err
			}
			return nil
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupAdvancedCommands,
		},
	}

	flag := cmd.Flags()
	opts.AddFlags(flag)
	options.AddKubeConfigFlags(flag)
	options.AddNamespaceFlag(flag)

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)

	return cmd
}

// CommandRebalanceOption holds all command options for rebalance
type CommandRebalanceOption struct {
	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool

	// Namespace is the namespace of resource
	Namespace string
}

// AddFlags adds flags to the specified FlagSet.
func (o *CommandRebalanceOption) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandRebalanceOption) Complete(f util.Factory, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("incorrect command format, please use correct command format")
	}

	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return fmt.Errorf("failed to get namespace from Factory. error: %w", err)
	}

	return nil
}

// Validate checks to the RebalanceOptions to see if there is sufficient information run the command
func (o *CommandRebalanceOption) Validate() error {
	return nil
}

// Run rebalance resources to achieve a fresh rescheduling
func (o *CommandRebalanceOption) Run(f util.Factory, args []string) error {
	// 1. get target rebalance object
	obj, err := o.getObjInfo(f, args)
	if err != nil {
		return fmt.Errorf("failed to get resource in Karmada: %w", err)
	}

	// 2. get karmada-apiserver client
	karmadaClient, err := f.KarmadaClientSet()
	if err != nil {
		return fmt.Errorf("failed to get karmada-apiserver client. err: %w", err)
	}

	// 3. rebalance object
	return o.rebalanceObject(karmadaClient, obj)
}

// getObjInfo get obj info in member cluster
func (o *CommandRebalanceOption) getObjInfo(f cmdutil.Factory, args []string) (*unstructured.Unstructured, error) {
	r := f.NewBuilder().
		Unstructured().
		NamespaceParam(o.Namespace).
		RequestChunksOf(500).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()

	infos, err := r.Infos()
	if err != nil {
		return nil, err
	}

	return infos[0].Object.(*unstructured.Unstructured), nil
}

func (o *CommandRebalanceOption) rebalanceObject(cli karmadaclientset.Interface, obj *unstructured.Unstructured) error {
	if o.DryRun {
		fmt.Printf("try to rebalance resource %s %s %s\n", obj.GetKind(), obj.GetNamespace(), obj.GetName())
		return nil
	}

	rebalancer := &appsv1alpha1.WorkloadRebalancer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(uuid.NewUUID()),
			Namespace: obj.GetNamespace(),
			Labels:    map[string]string{rebalancerByCtl: "true"},
		},
		Spec: appsv1alpha1.WorkloadRebalancerSpec{
			Workloads: []appsv1alpha1.ObjectReference{{
				APIVersion: obj.GetAPIVersion(),
				Kind:       obj.GetKind(),
				Name:       obj.GetName(),
				Namespace:  obj.GetNamespace(),
			}},
			TTLSecondsAfterFinished: ptr.To[int32](rebalanceAutoDeleteTTL),
		},
	}

	_, err := cli.AppsV1alpha1().WorkloadRebalancers().Create(context.TODO(), rebalancer, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create WorkloadRebalancer when triggering rebalance: %w", err)
	}

	err = wait.PollUntilContextTimeout(context.TODO(), checkInterval, checkTimeout, false,
		func(ctx context.Context) (done bool, err error) {
			// 1. get WorkloadRebalancer
			rebalancerGet, err := cli.AppsV1alpha1().WorkloadRebalancers().Get(ctx, rebalancer.Name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return false, fmt.Errorf("unknown rebalance result, WorkloadRebalancer/%s seems to have been unexpected deleted", rebalancer.Name)
				}
				return false, err
			}
			fmt.Printf("WorkloadRebalancer/%s created, waiting for its result...\n", rebalancer.Name)

			// 2. wait for expect result: observedWorkload's result is successful
			if len(rebalancerGet.Status.ObservedWorkloads) == 0 {
				// haven't been reconciled, retry
				return false, nil
			}
			observedWorkload := rebalancerGet.Status.ObservedWorkloads[0]
			if observedWorkload.Result == "" {
				// haven't finished, retry
				return false, nil
			}
			if observedWorkload.Result == appsv1alpha1.RebalanceFailed {
				// already failed, return
				return true, fmt.Errorf("rebalance failed due to %s", observedWorkload.Reason)
			}

			return true, nil
		})

	if err != nil {
		return fmt.Errorf("failed to get rebalance result: %w", err)
	}

	if obj.GetNamespace() == "" {
		fmt.Printf("successfully rebalance %s %s\n", obj.GetKind(), obj.GetName())
	} else {
		fmt.Printf("successfully rebalance %s %s/%s\n", obj.GetKind(), obj.GetNamespace(), obj.GetName())
	}

	return nil
}
