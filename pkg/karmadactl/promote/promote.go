package promote

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/get"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native/prune"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

var (
	promoteLong = templates.LongDesc(`
	Promote resources from legacy clusters to Karmada control plane. 
	Requires the cluster has been joined or registered.

	If the resource already exists in Karmada control plane, 
	please edit PropagationPolicy and OverridePolicy to propagate it.
	`)

	promoteExample = templates.Examples(`
		# Promote deployment(default/nginx) from cluster1 to Karmada
		%[1]s promote deployment nginx -n default -C cluster1

		# Promote deployment(default/nginx) with gvk from cluster1 to Karmada
		%[1]s promote deployment.v1.apps nginx -n default -C cluster1

		# Dumps the artifacts but does not deploy them to Karmada, same as 'dry run'
		%[1]s promote deployment nginx -n default -C cluster1 -o yaml|json

		# Promote secret(default/default-token) from cluster1 to Karmada
		%[1]s promote secret default-token -n default -C cluster1

		# Support to use '--cluster-kubeconfig' to specify the configuration of member cluster
		%[1]s promote deployment nginx -n default -C cluster1 --cluster-kubeconfig=<CLUSTER_KUBECONFIG_PATH>

		# Support to use '--cluster-kubeconfig' and '--cluster-context' to specify the configuration of member cluster
		%[1]s promote deployment nginx -n default -C cluster1 --cluster-kubeconfig=<CLUSTER_KUBECONFIG_PATH> --cluster-context=<CLUSTER_CONTEXT>`)
)

// NewCmdPromote defines the `promote` command that promote resources from legacy clusters
func NewCmdPromote(f util.Factory, parentCommand string) *cobra.Command {
	opts := CommandPromoteOption{}
	opts.JSONYamlPrintFlags = genericclioptions.NewJSONYamlPrintFlags()

	cmd := &cobra.Command{
		Use:                   "promote <RESOURCE_TYPE> <RESOURCE_NAME> -n <NAME_SPACE> -C <CLUSTER_NAME>",
		Short:                 "Promote resources from legacy clusters to Karmada control plane",
		Long:                  promoteLong,
		Example:               fmt.Sprintf(promoteExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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

	return cmd
}

// CommandPromoteOption holds all command options for promote
type CommandPromoteOption struct {
	// Cluster is the name of legacy cluster
	Cluster string

	// Namespace is the namespace of legacy resource
	Namespace string

	// ClusterContext is the cluster's context that we are going to join with.
	ClusterContext string

	// ClusterKubeConfig is the cluster's kubeconfig path.
	ClusterKubeConfig string

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool

	// AutoCreatePolicy tells if the promote command should create the
	// PropagationPolicy(or ClusterPropagationPolicy) by default.
	// It default to true.
	AutoCreatePolicy bool

	// PolicyName is the name of the PropagationPolicy(or ClusterPropagationPolicy),
	// It defaults to the promoting resource name with a random hash suffix.
	// It will be ingnored if AutoCreatePolicy is false.
	PolicyName string

	resource.FilenameOptions

	JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags
	OutputFormat       string
	Printer            func(*meta.RESTMapping, *bool, bool, bool) (printers.ResourcePrinterFunc, error)
	TemplateFlags      *genericclioptions.KubeTemplatePrintFlags

	name string
	gvk  schema.GroupVersionKind
}

// AddFlags adds flags to the specified FlagSet.
func (o *CommandPromoteOption) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.AutoCreatePolicy, "auto-create-policy", true,
		"Automatically create a PropagationPolicy for namespace-scoped resources or create a ClusterPropagationPolicy for cluster-scoped resources.")
	flags.StringVar(&o.PolicyName, "policy-name", "",
		"The name of the PropagationPolicy(or ClusterPropagationPolicy) that is automatically created after promotion. If not specified, the name will be the resource name with a hash suffix that is generated by resource metadata.")
	flags.StringVarP(&o.OutputFormat, "output", "o", "", "Output format. One of: json|yaml")

	flags.StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "If present, the namespace scope for this CLI request")
	flags.StringVarP(&o.Cluster, "cluster", "C", "", "the name of legacy cluster (eg -C=member1)")
	flags.StringVar(&o.ClusterContext, "cluster-context", "",
		"Context name of legacy cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&o.ClusterKubeConfig, "cluster-kubeconfig", "",
		"Path of the legacy cluster's kubeconfig.")
	flags.BoolVar(&o.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandPromoteOption) Complete(f util.Factory, args []string) error {
	var err error

	if len(args) != 2 {
		return fmt.Errorf("incorrect command format, please use correct command format")
	}

	o.name = args[1]

	if o.OutputFormat == "yaml" || o.OutputFormat == "json" {
		o.Printer = func(mapping *meta.RESTMapping, outputObjects *bool, withNamespace bool, withKind bool) (printers.ResourcePrinterFunc, error) {
			printer, err := o.JSONYamlPrintFlags.ToPrinter(o.OutputFormat)

			if genericclioptions.IsNoCompatiblePrinterError(err) {
				return nil, err
			}

			printer, err = printers.NewTypeSetter(gclient.NewSchema()).WrapToPrinter(printer, nil)
			if err != nil {
				return nil, err
			}

			return printer.PrintObj, nil
		}
	}

	if o.Namespace == "" {
		o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("failed to get namespace from Factory. error: %w", err)
		}
	}

	// If '--cluster-context' not specified, take the cluster name as the context.
	if len(o.ClusterContext) == 0 {
		o.ClusterContext = o.Cluster
	}

	return nil
}

// Validate checks to the PromoteOptions to see if there is sufficient information run the command
func (o *CommandPromoteOption) Validate() error {
	if o.Cluster == "" {
		return fmt.Errorf("the cluster cannot be empty")
	}

	if o.OutputFormat != "" && o.OutputFormat != "yaml" && o.OutputFormat != "json" {
		return fmt.Errorf("output format is only one of json and yaml")
	}

	return nil
}

// Run promote resources from legacy clusters
func (o *CommandPromoteOption) Run(f util.Factory, args []string) error {
	var memberClusterFactory cmdutil.Factory
	var err error

	if o.ClusterKubeConfig != "" {
		kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
		kubeConfigFlags.KubeConfig = &o.ClusterKubeConfig
		kubeConfigFlags.Context = &o.ClusterContext

		memberClusterFactory = cmdutil.NewFactory(kubeConfigFlags)
	} else {
		memberClusterFactory, err = f.FactoryForMemberCluster(o.Cluster)
		if err != nil {
			return fmt.Errorf("failed to get Factory of the member cluster. err: %w", err)
		}
	}

	objInfo, err := o.getObjInfo(memberClusterFactory, o.Cluster, args)
	if err != nil {
		return fmt.Errorf("failed to get resource in cluster(%s). err: %w", o.Cluster, err)
	}

	obj := objInfo.Info.Object.(*unstructured.Unstructured)

	o.gvk = obj.GetObjectKind().GroupVersionKind()

	controlPlaneRestConfig, err := f.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. err: %w", err)
	}

	mapper, err := restmapper.MapperProvider(controlPlaneRestConfig)
	if err != nil {
		return fmt.Errorf("failed to create restmapper: %v", err)
	}

	gvr, err := restmapper.GetGroupVersionResource(mapper, o.gvk)
	if err != nil {
		return fmt.Errorf("failed to get gvr from %q: %v", o.gvk, err)
	}

	return o.promote(controlPlaneRestConfig, obj, gvr)
}

func (o *CommandPromoteOption) promote(controlPlaneRestConfig *rest.Config, obj *unstructured.Unstructured, gvr schema.GroupVersionResource) error {
	if err := preprocessResource(obj); err != nil {
		return fmt.Errorf("failed to preprocess resource %q(%s/%s) in control plane: %v", gvr, o.Namespace, o.name, err)
	}

	if o.OutputFormat != "" {
		// only print the resource template and Policy
		err := o.printObjectAndPolicy(obj, gvr)

		return err
	}

	if o.DryRun {
		return nil
	}

	controlPlaneDynamicClient := dynamic.NewForConfigOrDie(controlPlaneRestConfig)

	karmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)

	if len(obj.GetNamespace()) == 0 {
		_, err := controlPlaneDynamicClient.Resource(gvr).Get(context.TODO(), o.name, metav1.GetOptions{})
		if err == nil {
			fmt.Printf("Resource %q(%s) already exist in karmada control plane, you can edit PropagationPolicy and OverridePolicy to propagate it\n",
				gvr, o.name)
			return nil
		}

		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get resource %q(%s) in control plane: %v", gvr, o.name, err)
		}

		_, err = controlPlaneDynamicClient.Resource(gvr).Create(context.TODO(), obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create resource %q(%s) in control plane: %v", gvr, o.name, err)
		}

		if o.AutoCreatePolicy {
			err = o.createClusterPropagationPolicy(karmadaClient, gvr)
			if err != nil {
				return err
			}
		}

		fmt.Printf("Resource %q(%s) is promoted successfully\n", gvr, o.name)
	} else {
		_, err := controlPlaneDynamicClient.Resource(gvr).Namespace(o.Namespace).Get(context.TODO(), o.name, metav1.GetOptions{})
		if err == nil {
			fmt.Printf("Resource %q(%s/%s) already exist in karmada control plane, you can edit PropagationPolicy and OverridePolicy to propagate it\n",
				gvr, o.Namespace, o.name)
			return nil
		}

		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get resource %q(%s/%s) in control plane: %v", gvr, o.Namespace, o.name, err)
		}

		_, err = controlPlaneDynamicClient.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create resource %q(%s/%s) in control plane: %v", gvr, o.Namespace, o.name, err)
		}

		if o.AutoCreatePolicy {
			err = o.createPropagationPolicy(karmadaClient, gvr)
			if err != nil {
				return err
			}
		}

		fmt.Printf("Resource %q(%s/%s) is promoted successfully\n", gvr, o.Namespace, o.name)
	}

	return nil
}

// getObjInfo get obj info in member cluster
func (o *CommandPromoteOption) getObjInfo(f cmdutil.Factory, cluster string, args []string) (*get.Obj, error) {
	r := f.NewBuilder().
		Unstructured().
		NamespaceParam(o.Namespace).
		FilenameParam(false, &o.FilenameOptions).
		RequestChunksOf(500).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()

	r.IgnoreErrors(apierrors.IsNotFound)

	infos, err := r.Infos()
	if err != nil {
		return nil, err
	}

	if len(infos) == 0 {
		return nil, fmt.Errorf("the %s or %s don't exist in cluster(%s)", args[0], args[1], o.Cluster)
	}

	obj := &get.Obj{
		Cluster: cluster,
		Info:    infos[0],
	}

	return obj, nil
}

// printObjectAndPolicy print the converted resource
func (o *CommandPromoteOption) printObjectAndPolicy(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) error {
	printer, err := o.Printer(nil, nil, false, false)
	if err != nil {
		return fmt.Errorf("failed to initialize k8s printer. err: %v", err)
	}

	if err = printer.PrintObj(obj, os.Stdout); err != nil {
		return fmt.Errorf("failed to print the resource template. err: %v", err)
	}
	if o.AutoCreatePolicy {
		var policyName string
		if o.PolicyName == "" {
			policyName = names.GeneratePolicyName(o.Namespace, o.name, o.gvk.String())
		} else {
			policyName = o.PolicyName
		}
		if len(obj.GetNamespace()) == 0 {
			cpp := buildClusterPropagationPolicy(o.name, policyName, o.Cluster, gvr, o.gvk)
			if err = printer.PrintObj(cpp, os.Stdout); err != nil {
				return fmt.Errorf("failed to print the ClusterPropagationPolicy. err: %v", err)
			}
		} else {
			pp := buildPropagationPolicy(o.name, policyName, o.Namespace, o.Cluster, gvr, o.gvk)
			if err = printer.PrintObj(pp, os.Stdout); err != nil {
				return fmt.Errorf("failed to print the PropagationPolicy. err: %v", err)
			}
		}
	}

	return nil
}

// createPropagationPolicy create PropagationPolicy in karmada control plane
func (o *CommandPromoteOption) createPropagationPolicy(karmadaClient *karmadaclientset.Clientset, gvr schema.GroupVersionResource) error {
	var policyName string
	if o.PolicyName == "" {
		policyName = names.GeneratePolicyName(o.Namespace, o.name, o.gvk.String())
	} else {
		policyName = o.PolicyName
	}

	_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(o.Namespace).Get(context.TODO(), policyName, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		pp := buildPropagationPolicy(o.name, policyName, o.Namespace, o.Cluster, gvr, o.gvk)
		_, err = karmadaClient.PolicyV1alpha1().PropagationPolicies(o.Namespace).Create(context.TODO(), pp, metav1.CreateOptions{})

		return err
	}
	if err != nil {
		return fmt.Errorf("failed to get PropagationPolicy(%s/%s) in control plane: %v", o.Namespace, policyName, err)
	}

	// PropagationPolicy already exists, not to create it
	return fmt.Errorf("the PropagationPolicy(%s/%s) already exist, please edit it to propagate resource", o.Namespace, policyName)
}

// createClusterPropagationPolicy create ClusterPropagationPolicy in karmada control plane
func (o *CommandPromoteOption) createClusterPropagationPolicy(karmadaClient *karmadaclientset.Clientset, gvr schema.GroupVersionResource) error {
	var policyName string
	if o.PolicyName == "" {
		policyName = names.GeneratePolicyName("", o.name, o.gvk.String())
	} else {
		policyName = o.PolicyName
	}

	_, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Get(context.TODO(), policyName, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		cpp := buildClusterPropagationPolicy(o.name, policyName, o.Cluster, gvr, o.gvk)
		_, err = karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Create(context.TODO(), cpp, metav1.CreateOptions{})

		return err
	}
	if err != nil {
		return fmt.Errorf("failed to get ClusterPropagationPolicy(%s) in control plane: %v", policyName, err)
	}

	// ClusterPropagationPolicy already exists, not to create it
	return fmt.Errorf("the ClusterPropagationPolicy(%s) already exist, please edit it to propagate resource", policyName)
}

// preprocessResource delete redundant fields to convert resource as template
func preprocessResource(obj *unstructured.Unstructured) error {
	// remove fields that generated by kube-apiserver and no need(or can't) propagate to member clusters.
	if err := prune.RemoveIrrelevantField(obj); err != nil {
		return err
	}

	addOverwriteAnnotation(obj)

	return nil
}

// addOverwriteAnnotation add annotation "work.karmada.io/conflict-resolution" to the resource
func addOverwriteAnnotation(obj *unstructured.Unstructured) {
	annotations := obj.DeepCopy().GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	if _, exist := annotations[workv1alpha2.ResourceConflictResolutionAnnotation]; !exist {
		annotations[workv1alpha2.ResourceConflictResolutionAnnotation] = workv1alpha2.ResourceConflictResolutionOverwrite
		obj.SetAnnotations(annotations)
	}
}

// buildPropagationPolicy build PropagationPolicy according to resource and cluster
func buildPropagationPolicy(resourceName, policyName, namespace, cluster string, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) *policyv1alpha1.PropagationPolicy {
	pp := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: namespace,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: gvr.GroupVersion().String(),
					Kind:       gvk.Kind,
					Name:       resourceName,
				},
			},
			Placement: policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{cluster},
				},
			},
		},
	}
	return pp
}

// buildClusterPropagationPolicy build ClusterPropagationPolicy according to resource and cluster
func buildClusterPropagationPolicy(resourceName, policyName, cluster string, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) *policyv1alpha1.ClusterPropagationPolicy {
	cpp := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: gvr.GroupVersion().String(),
					Kind:       gvk.Kind,
					Name:       resourceName,
				},
			},
			Placement: policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{cluster},
				},
			},
		},
	}
	return cpp
}
