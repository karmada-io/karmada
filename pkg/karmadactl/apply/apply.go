/*
Copyright 2022 The Karmada Authors.

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

package apply

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"
	kubectlapply "k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/util/templates"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var metadataAccessor = meta.NewAccessor()

// CommandApplyOptions contains the input to the apply command.
type CommandApplyOptions struct {
	// apply flags
	KubectlApplyFlags *kubectlapply.ApplyFlags
	UtilFactory       util.Factory
	AllClusters       bool
	Clusters          []string

	kubectlApplyOptions *kubectlapply.ApplyOptions
	karmadaClient       karmadaclientset.Interface
}

var (
	applyLong = templates.LongDesc(`
		Apply a configuration to a resource by file name or stdin and propagate them into member clusters.
		The resource name must be specified. This resource will be created if it doesn't exist yet.
		To use 'apply', always create the resource initially with either 'apply' or 'create --save-config'.

		JSON and YAML formats are accepted.

		Alpha Disclaimer: the --prune functionality is not yet complete. Do not use unless you are aware of what the current state is. See https://issues.k8s.io/34274.

		Note: It implements the function of 'kubectl apply' by default.
		If you want to propagate them into member clusters, please use %[1]s apply --all-clusters'.`)

	applyExample = templates.Examples(`
		# Apply the configuration without propagation into member clusters. It acts as 'kubectl apply'.
		%[1]s apply -f manifest.yaml

		# Apply the configuration with propagation into specific member clusters.
		%[1]s apply -f manifest.yaml --cluster member1,member2

		# Apply resources from a directory and propagate them into all member clusters.
		%[1]s apply -f dir/ --all-clusters

		# Apply resources from a directory containing kustomization.yaml - e.g.
		# dir/kustomization.yaml, and propagate them into all member clusters
		%[1]s apply -k dir/ --all-clusters`)
)

// NewCmdApply creates the `apply` command
func NewCmdApply(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	o := &CommandApplyOptions{
		KubectlApplyFlags: kubectlapply.NewApplyFlags(streams),
		UtilFactory:       f,
	}
	cmd := &cobra.Command{
		Use:                   "apply (-f FILENAME | -k DIRECTORY)",
		Short:                 "Apply a configuration to a resource by file name or stdin and propagate them into member clusters",
		Long:                  applyLong,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		Example:               fmt.Sprintf(applyExample, parentCommand),
		ValidArgsFunction:     utilcomp.ResourceTypeAndNameCompletionFunc(f),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(f, cmd, parentCommand, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run()
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupAdvancedCommands,
		},
	}

	o.KubectlApplyFlags.AddFlags(cmd)
	flags := cmd.Flags()
	options.AddKubeConfigFlags(flags)
	options.AddNamespaceFlag(flags)
	flags.BoolVarP(&o.AllClusters, "all-clusters", "", o.AllClusters, "If present, propagates a group of resources to all member clusters.")
	flags.StringSliceVarP(&o.Clusters, "cluster", "C", o.Clusters, "If present, propagates a group of resources to specified clusters.")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	utilcomp.RegisterCompletionFuncForClusterFlag(cmd)

	return cmd
}

// Complete completes all the required options
func (o *CommandApplyOptions) Complete(f util.Factory, cmd *cobra.Command, parentCommand string, args []string) error {
	karmadaClient, err := f.KarmadaClientSet()
	if err != nil {
		return err
	}
	o.karmadaClient = karmadaClient
	kubectlApplyOptions, err := o.KubectlApplyFlags.ToOptions(f, cmd, parentCommand, args)
	if err != nil {
		return err
	}
	o.kubectlApplyOptions = kubectlApplyOptions
	return nil
}

// Validate verifies if CommandApplyOptions are valid and without conflicts.
func (o *CommandApplyOptions) Validate() error {
	if o.AllClusters && len(o.Clusters) > 0 {
		return errors.New("--all-clusters and --cluster cannot be used together")
	}
	if len(o.Clusters) > 0 {
		clusters, err := o.karmadaClient.ClusterV1alpha1().Clusters().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		clusterSet := sets.NewString()
		for _, cluster := range clusters.Items {
			clusterSet.Insert(cluster.Name)
		}
		var errs []error
		for _, cluster := range o.Clusters {
			if !clusterSet.Has(cluster) {
				errs = append(errs, fmt.Errorf("cluster %s does not exist", cluster))
			}
		}
		return utilerrors.NewAggregate(errs)
	}
	return o.kubectlApplyOptions.Validate()
}

// Run executes the `apply` command.
func (o *CommandApplyOptions) Run() error {
	if !o.AllClusters && len(o.Clusters) == 0 {
		return o.kubectlApplyOptions.Run()
	}

	if err := o.generateAndInjectPolicies(); err != nil {
		return err
	}

	return o.kubectlApplyOptions.Run()
}

// generateAndInjectPolicies generates and injects policies to the given resources.
// It returns an error if any of the policies cannot be generated.
func (o *CommandApplyOptions) generateAndInjectPolicies() error {
	// load the resources
	infos, err := o.kubectlApplyOptions.GetObjects()
	if err != nil {
		return err
	}

	// generate policies and append them to the resources
	var results []*resource.Info
	for _, info := range infos {
		results = append(results, info)
		obj := o.generatePropagationObject(info)
		gvk := obj.GetObjectKind().GroupVersionKind()
		mapping, err := o.kubectlApplyOptions.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("unable to recognize resource: %v", err)
		}
		client, err := o.UtilFactory.UnstructuredClientForMapping(mapping)
		if err != nil {
			return fmt.Errorf("unable to connect to a server to handle %q: %v", mapping.Resource, err)
		}
		policyName, _ := metadataAccessor.Name(obj)
		ret := &resource.Info{
			Namespace: info.Namespace,
			Name:      policyName,
			Object:    obj,
			Mapping:   mapping,
			Client:    client,
		}
		results = append(results, ret)
	}

	// store the results object to be sequentially applied
	o.kubectlApplyOptions.SetObjects(results)
	return nil
}

// generatePropagationObject generates a propagation object for the given resource info.
// It takes the resource namespace, name and GVK as input to generate policy name.
func (o *CommandApplyOptions) generatePropagationObject(info *resource.Info) runtime.Object {
	gvk := info.Mapping.GroupVersionKind
	spec := policyv1alpha1.PropagationSpec{
		ResourceSelectors: []policyv1alpha1.ResourceSelector{
			{
				APIVersion: gvk.GroupVersion().String(),
				Kind:       gvk.Kind,
				Name:       info.Name,
				Namespace:  info.Namespace,
			},
		},
	}

	if len(o.Clusters) > 0 {
		spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{
			ClusterNames: o.Clusters,
		}
	}

	if o.AllClusters {
		spec.Placement.ClusterAffinity = &policyv1alpha1.ClusterAffinity{}
	}

	// for a namespaced-scope resource, we need to generate a PropagationPolicy object.
	// for a cluster-scope resource, we need to generate a ClusterPropagationPolicy object.
	var obj runtime.Object
	policyName := names.GeneratePolicyName(info.Namespace, info.Name, gvk.String())
	if info.Namespaced() {
		obj = &policyv1alpha1.PropagationPolicy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "policy.karmada.io/v1alpha1",
				Kind:       "PropagationPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyName,
				Namespace: info.Namespace,
			},
			Spec: spec,
		}
	} else {
		obj = &policyv1alpha1.ClusterPropagationPolicy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "policy.karmada.io/v1alpha1",
				Kind:       "ClusterPropagationPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: policyName,
			},
			Spec: spec,
		}
	}
	return obj
}
