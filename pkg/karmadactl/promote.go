package karmadactl

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
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/defaultinterpreter/prune"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/restmapper"
)

var (
	promoteShort   = `Promote resources from legacy clusters to karmada control plane`
	promoteLong    = `Promote resources from legacy clusters to karmada control plane. Requires the cluster be joined or registered.`
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
func NewCmdPromote(karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	opts := CommandPromoteOption{}
	opts.JSONYamlPrintFlags = genericclioptions.NewJSONYamlPrintFlags()

	cmd := &cobra.Command{
		Use:                   "promote <RESOURCE_TYPE> <RESOURCE_NAME> -n <NAME_SPACE> -C <CLUSTER_NAME>",
		Short:                 promoteShort,
		Long:                  promoteLong,
		Example:               fmt.Sprintf(promoteExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := RunPromote(karmadaConfig, opts, args); err != nil {
				return err
			}
			return nil
		},
	}

	flag := cmd.Flags()
	opts.AddFlags(flag)

	return cmd
}

// CommandPromoteOption holds all command options for promote
type CommandPromoteOption struct {
	options.GlobalCommandOptions

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
	o.GlobalCommandOptions.AddFlags(flags)

	flags.StringVarP(&o.OutputFormat, "output", "o", "", "Output format. One of: json|yaml")

	flags.StringVarP(&o.Namespace, "namespace", "n", "default", "-n=namespace or -n namespace")
	flags.StringVarP(&o.Cluster, "cluster", "C", "", "the name of legacy cluster (eg -C=member1)")
	flags.StringVar(&o.ClusterContext, "cluster-context", "",
		"Context name of legacy cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&o.ClusterKubeConfig, "cluster-kubeconfig", "",
		"Path of the legacy cluster's kubeconfig.")
	flags.BoolVar(&o.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandPromoteOption) Complete(args []string) error {
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

	// If '--cluster-context' not specified, take the cluster name as the context.
	if len(o.ClusterContext) == 0 {
		o.ClusterContext = o.Cluster
	}

	return nil
}

// RunPromote promote resources from legacy clusters
func RunPromote(karmadaConfig KarmadaConfig, opts CommandPromoteOption, args []string) error {
	// Get control plane karmada-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kubeconfig: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err)
	}

	clusterInfos := make(map[string]*ClusterInfo)

	if err := getClusterInKarmada(controlPlaneRestConfig, clusterInfos); err != nil {
		return fmt.Errorf("failed to get cluster info in karmada control plane. err: %v", err)
	}

	if _, exist := clusterInfos[opts.Cluster]; !exist {
		return fmt.Errorf("cluster(%s) does't exist in karmada control plane", opts.Cluster)
	}

	var f cmdutil.Factory

	if opts.ClusterKubeConfig != "" {
		kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
		kubeConfigFlags.KubeConfig = &opts.ClusterKubeConfig
		kubeConfigFlags.Context = &opts.ClusterContext

		f = cmdutil.NewFactory(kubeConfigFlags)
	} else {
		opts.setClusterProxyInfo(controlPlaneRestConfig, opts.Cluster, clusterInfos)
		f = getFactory(opts.Cluster, clusterInfos, "")
	}

	objInfo, err := opts.getObjInfo(f, opts.Cluster, args)
	if err != nil {
		return fmt.Errorf("failed to get resource in cluster(%s). err: %v", opts.Cluster, err)
	}

	obj := objInfo.Info.Object.(*unstructured.Unstructured)

	opts.gvk = obj.GetObjectKind().GroupVersionKind()

	mapper, err := restmapper.MapperProvider(controlPlaneRestConfig)
	if err != nil {
		return fmt.Errorf("failed to create restmapper: %v", err)
	}

	gvr, err := restmapper.GetGroupVersionResource(mapper, opts.gvk)
	if err != nil {
		return fmt.Errorf("failed to get gvr from %q: %v", opts.gvk, err)
	}

	return promote(controlPlaneRestConfig, obj, gvr, opts)
}

func promote(controlPlaneRestConfig *rest.Config, obj *unstructured.Unstructured, gvr schema.GroupVersionResource, opts CommandPromoteOption) error {
	if err := preprocessResource(obj); err != nil {
		return fmt.Errorf("failed to preprocess resource %q(%s/%s) in control plane: %v", gvr, opts.Namespace, opts.name, err)
	}

	if opts.OutputFormat != "" {
		// only print the resource template and Policy
		err := printObjectAndPolicy(obj, gvr, opts)

		return err
	}

	if opts.DryRun {
		return nil
	}

	controlPlaneDynamicClient := dynamic.NewForConfigOrDie(controlPlaneRestConfig)

	karmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)

	if len(obj.GetNamespace()) == 0 {
		_, err := controlPlaneDynamicClient.Resource(gvr).Get(context.TODO(), opts.name, metav1.GetOptions{})
		if err == nil {
			fmt.Printf("Resource %q(%s) already exist in karmada control plane, you can edit PropagationPolicy and OverridePolicy to propagate it\n",
				gvr, opts.name)
			return nil
		}

		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get resource %q(%s) in control plane: %v", gvr, opts.name, err)
		}

		_, err = controlPlaneDynamicClient.Resource(gvr).Create(context.TODO(), obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create resource %q(%s) in control plane: %v", gvr, opts.name, err)
		}

		err = createOrUpdateClusterPropagationPolicy(karmadaClient, gvr, opts)
		if err != nil {
			return err
		}

		fmt.Printf("Resource %q(%s) is promoted successfully\n", gvr, opts.name)
	} else {
		_, err := controlPlaneDynamicClient.Resource(gvr).Namespace(opts.Namespace).Get(context.TODO(), opts.name, metav1.GetOptions{})
		if err == nil {
			fmt.Printf("Resource %q(%s/%s) already exist in karmada control plane, you can edit PropagationPolicy and OverridePolicy to propagate it\n",
				gvr, opts.Namespace, opts.name)
			return nil
		}

		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get resource %q(%s/%s) in control plane: %v", gvr, opts.Namespace, opts.name, err)
		}

		_, err = controlPlaneDynamicClient.Resource(gvr).Namespace(opts.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create resource %q(%s/%s) in control plane: %v", gvr, opts.Namespace, opts.name, err)
		}

		err = createOrUpdatePropagationPolicy(karmadaClient, gvr, opts)
		if err != nil {
			return err
		}

		fmt.Printf("Resource %q(%s/%s) is promoted successfully\n", gvr, opts.Namespace, opts.name)
	}

	return nil
}

// getObjInfo get obj info in member cluster
func (o *CommandPromoteOption) getObjInfo(f cmdutil.Factory, cluster string, args []string) (*Obj, error) {
	chunkSize := int64(500)
	r := f.NewBuilder().
		Unstructured().
		NamespaceParam(o.Namespace).
		FilenameParam(false, &o.FilenameOptions).
		RequestChunksOf(chunkSize).
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

	obj := &Obj{
		Cluster: cluster,
		Info:    infos[0],
	}

	return obj, nil
}

// setClusterProxyInfo set proxy information of cluster
func (o *CommandPromoteOption) setClusterProxyInfo(karmadaRestConfig *rest.Config, name string, clusterInfos map[string]*ClusterInfo) {
	clusterInfos[name].APIEndpoint = karmadaRestConfig.Host + fmt.Sprintf(proxyURL, name)
	clusterInfos[name].KubeConfig = o.KubeConfig
	clusterInfos[name].Context = o.KarmadaContext
	if clusterInfos[name].KubeConfig == "" {
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			clusterInfos[name].KubeConfig = env
		} else {
			clusterInfos[name].KubeConfig = defaultKubeConfig
		}
	}
}

// printObjectAndPolicy print the converted resource
func printObjectAndPolicy(obj *unstructured.Unstructured, gvr schema.GroupVersionResource, opts CommandPromoteOption) error {
	printer, err := opts.Printer(nil, nil, false, false)
	if err != nil {
		return fmt.Errorf("failed to initialize k8s printer. err: %v", err)
	}

	if err = printer.PrintObj(obj, os.Stdout); err != nil {
		return fmt.Errorf("failed to print the resource template. err: %v", err)
	}

	if len(obj.GetNamespace()) == 0 {
		cpp := buildClusterPropagationPolicy(gvr, opts)
		if err = printer.PrintObj(cpp, os.Stdout); err != nil {
			return fmt.Errorf("failed to print the ClusterPropagationPolicy. err: %v", err)
		}
	} else {
		pp := buildPropagationPolicy(gvr, opts)
		if err = printer.PrintObj(pp, os.Stdout); err != nil {
			return fmt.Errorf("failed to print the PropagationPolicy. err: %v", err)
		}
	}

	return nil
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

// createOrUpdatePropagationPolicy create PropagationPolicy in karmada control plane
func createOrUpdatePropagationPolicy(karmadaClient *karmadaclientset.Clientset, gvr schema.GroupVersionResource, opts CommandPromoteOption) error {
	name := names.GeneratePolicyName(opts.Namespace, opts.name, opts.gvk.String())

	_, err := karmadaClient.PolicyV1alpha1().PropagationPolicies(opts.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		pp := buildPropagationPolicy(gvr, opts)
		_, err = karmadaClient.PolicyV1alpha1().PropagationPolicies(opts.Namespace).Create(context.TODO(), pp, metav1.CreateOptions{})

		return err
	}
	if err != nil {
		return fmt.Errorf("failed to get PropagationPolicy(%s/%s) in control plane: %v", opts.Namespace, name, err)
	}

	// PropagationPolicy already exists, not to create it
	return fmt.Errorf("the PropagationPolicy(%s/%s) already exist, please edit it to propagate resource", opts.Namespace, name)
}

// createOrUpdateClusterPropagationPolicy create ClusterPropagationPolicy in karmada control plane
func createOrUpdateClusterPropagationPolicy(karmadaClient *karmadaclientset.Clientset, gvr schema.GroupVersionResource, opts CommandPromoteOption) error {
	name := names.GeneratePolicyName("", opts.name, opts.gvk.String())

	_, err := karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		cpp := buildClusterPropagationPolicy(gvr, opts)
		_, err = karmadaClient.PolicyV1alpha1().ClusterPropagationPolicies().Create(context.TODO(), cpp, metav1.CreateOptions{})

		return err
	}
	if err != nil {
		return fmt.Errorf("failed to get ClusterPropagationPolicy(%s) in control plane: %v", name, err)
	}

	// ClusterPropagationPolicy already exists, not to create it
	return fmt.Errorf("the ClusterPropagationPolicy(%s) already exist, please edit it to propagate resource", name)
}

// buildPropagationPolicy build PropagationPolicy according to resource and cluster
func buildPropagationPolicy(gvr schema.GroupVersionResource, opts CommandPromoteOption) *policyv1alpha1.PropagationPolicy {
	name := names.GeneratePolicyName(opts.Namespace, opts.name, opts.gvk.String())

	pp := &policyv1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: opts.Namespace,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: gvr.GroupVersion().String(),
					Kind:       opts.gvk.Kind,
					Name:       opts.name,
				},
			},
			Placement: policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{opts.Cluster},
				},
			},
		},
	}

	return pp
}

// buildClusterPropagationPolicy build ClusterPropagationPolicy according to resource and cluster
func buildClusterPropagationPolicy(gvr schema.GroupVersionResource, opts CommandPromoteOption) *policyv1alpha1.ClusterPropagationPolicy {
	name := names.GeneratePolicyName("", opts.name, opts.gvk.String())

	cpp := &policyv1alpha1.ClusterPropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: policyv1alpha1.PropagationSpec{
			ResourceSelectors: []policyv1alpha1.ResourceSelector{
				{
					APIVersion: gvr.GroupVersion().String(),
					Kind:       opts.gvk.Kind,
					Name:       opts.name,
				},
			},
			Placement: policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{opts.Cluster},
				},
			},
		},
	}
	return cpp
}
