package karmadactl

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	cmdwait "k8s.io/kubectl/pkg/cmd/wait"
	"k8s.io/kubectl/pkg/rawhttp"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

var (
	deleteLong = templates.LongDesc(i18n.T(`
    Similar to kubectl delete, it will further delete the PropagationPolicy or ClusterPropagationPolicy associated with the resource

	IMPORTANT:

	If you use --raw, the corresponding PropagationPolicy or ClusterPropagationPolicy will not be deleted
`))
)

// DeleteOptions contains the input to the delete command.
type DeleteOptions struct {
	resource.FilenameOptions

	LabelSelector       string
	FieldSelector       string
	DeleteAll           bool
	DeleteAllNamespaces bool
	CascadingStrategy   metav1.DeletionPropagation
	IgnoreNotFound      bool
	DeleteNow           bool
	ForceDeletion       bool
	WaitForDeletion     bool
	Quiet               bool
	WarnClusterScope    bool
	Raw                 string
	Namespace           string
	KarmadaContext      string
	KubeConfig          string
	GracePeriod         int
	Timeout             time.Duration

	DryRunStrategy cmdutil.DryRunStrategy
	DryRunVerifier *resource.DryRunVerifier

	Output string

	DynamicClient     dynamic.Interface
	Mapper            meta.RESTMapper
	Result            *resource.Result
	karmadaMapping    map[schema.GroupKind]*meta.RESTMapping
	karmadaRESTClient map[schema.GroupKind]resource.RESTClient
	genericclioptions.IOStreams
	options.GlobalCommandOptions
}

// NewCmdDelete creates the `delete` command
// refer to  https://github.com/kubernetes/kubernetes/blob/v1.23.4/staging/src/k8s.io/kubectl/pkg/cmd/delete/delete.go#L131
func NewCmdDelete(karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	deleteFlags := NewDeleteCommandFlags("containing the resource to delete.")
	streams := genericclioptions.IOStreams{In: getIn, Out: getOut, ErrOut: getErr}
	var f cmdutil.Factory
	cmd := &cobra.Command{
		Use:                   "delete ([-f FILENAME] | [-k DIRECTORY] | TYPE [(NAME | -l label | --all)])",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Delete resources by file names, stdin, resources and names, or by resources and label selector"),
		Long:                  deleteLong,
		Example:               deleteExample(parentCommand),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// factory must be initialized before the command runs
			o, err := deleteFlags.toOptions(nil, streams)
			cmdutil.CheckErr(err)
			restConfig, err := karmadaConfig.GetRestConfig(o.KarmadaContext, o.KubeConfig)
			cmdutil.CheckErr(err)
			kubeConfigFlags := NewConfigFlags(true).WithDeprecatedPasswordFlag()
			kubeConfigFlags.WrapConfigFn = func(config *restclient.Config) *restclient.Config { return restConfig }
			f = cmdutil.NewFactory(kubeConfigFlags)
			return nil
		},
		//ValidArgsFunction: util.ResourceTypeAndNameCompletionFunc(f),
		Run: func(cmd *cobra.Command, args []string) {
			o, err := deleteFlags.toOptions(nil, streams)
			cmdutil.CheckErr(err)
			cmdutil.CheckErr(o.Complete(f, args, cmd))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunDelete(f))
		},
		SuggestFor: []string{"rm"},
	}

	deleteFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)

	return cmd
}

func deleteExample(parentCommand string) string {
	return templates.Examples(i18n.T(`
		# Delete a pod using the type and name specified in pod.json` + "\n" +
		fmt.Sprintf("%s delete -f ./pod.json", parentCommand) + `
		# Delete resources from a directory containing kustomization.yaml - e.g. dir/kustomization.yaml` + "\n" +
		fmt.Sprintf("%s delete -k dir", parentCommand) + `
		# Delete a pod based on the type and name in the JSON passed into stdin` + "\n" +
		fmt.Sprintf("cat pod.json | %s delete -f -", parentCommand) + `
		# Delete pods and services with same names "baz" and "foo"` + "\n" +
		fmt.Sprintf("%s delete pod,service baz foo", parentCommand) + `
		# Delete pods and services with label name=myLabel` + "\n" +
		fmt.Sprintf("%s delete pods,services -l name=myLabel", parentCommand) + `
		# Delete a pod with minimal delay` + "\n" +
		fmt.Sprintf("%s delete pod foo --now", parentCommand) + `
		# Force delete a pod on a dead node` + "\n" +
		fmt.Sprintf("%s delete pod foo --force", parentCommand) + `
		# Delete all pods` + "\n" +
		fmt.Sprintf("%s delete pods --all", parentCommand)))
}

// Complete takes the command arguments and infers any remaining options.
// refer to https://github.com/kubernetes/kubernetes/blob/v1.23.4/staging/src/k8s.io/kubectl/pkg/cmd/delete/delete.go#L157
//gocyclo:ignore
func (o *DeleteOptions) Complete(f cmdutil.Factory, args []string, cmd *cobra.Command) (err error) {
	o.WarnClusterScope = o.Namespace != "" && !o.DeleteAllNamespaces

	if o.DeleteAll || len(o.LabelSelector) > 0 || len(o.FieldSelector) > 0 {
		if f := cmd.Flags().Lookup("ignore-not-found"); f != nil && !f.Changed {
			// If the user didn't explicitly set the option, default to ignoring NotFound errors when used with --all, -l, or --field-selector
			o.IgnoreNotFound = true
		}
	}
	if o.DeleteNow {
		if o.GracePeriod != -1 {
			return fmt.Errorf("--now and --grace-period cannot be specified together")
		}
		o.GracePeriod = 1
	}
	if o.GracePeriod == 0 && !o.ForceDeletion {
		// To preserve backwards compatibility, but prevent accidental data loss, we convert --grace-period=0
		// into --grace-period=1. Users may provide --force to bypass this conversion.
		o.GracePeriod = 1
	}
	if o.ForceDeletion && o.GracePeriod < 0 {
		o.GracePeriod = 0
	}

	o.DryRunStrategy, err = cmdutil.GetDryRunStrategy(cmd)
	if err != nil {
		return err
	}
	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return err
	}
	o.DryRunVerifier = resource.NewDryRunVerifier(dynamicClient, f.OpenAPIGetter())

	if len(o.Raw) == 0 {
		r := f.NewBuilder().
			Unstructured().
			ContinueOnError().
			NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.DeleteAllNamespaces).
			FilenameParam(o.Namespace != "", &o.FilenameOptions).
			LabelSelectorParam(o.LabelSelector).
			FieldSelectorParam(o.FieldSelector).
			SelectAllParam(o.DeleteAll).
			AllNamespaces(o.DeleteAllNamespaces).
			ResourceTypeOrNameArgs(false, args...).RequireObject(true).
			Flatten().
			Do()
		err = r.Err()
		if err != nil {
			return err
		}
		o.Result = r

		o.Mapper, err = f.ToRESTMapper()
		if err != nil {
			return err
		}

		o.DynamicClient, err = f.DynamicClient()
		if err != nil {
			return err
		}
		if o.karmadaMapping == nil {
			o.karmadaMapping = make(map[schema.GroupKind]*meta.RESTMapping)
		}
		if o.karmadaRESTClient == nil {
			o.karmadaRESTClient = make(map[schema.GroupKind]resource.RESTClient)
		}
		gk := schema.GroupKind{Kind: policyv1alpha1.ResourceKindPropagationPolicy, Group: policyv1alpha1.GroupName}
		mapping, err := o.Mapper.RESTMapping(gk, "v1alpha1")
		if err != nil {
			return fmt.Errorf("unable to recognize resource: %v", err)
		}
		o.karmadaMapping[gk] = mapping
		ppRESTClient, err := f.ClientForMapping(mapping)
		mapping.Scope = meta.RESTScopeNamespace
		if err != nil {
			return fmt.Errorf("unable to connect to a server to handle %q: %v", mapping.Resource, err)
		}
		o.karmadaRESTClient[gk] = ppRESTClient

		gk = schema.GroupKind{Kind: policyv1alpha1.ResourceKindClusterPropagationPolicy, Group: policyv1alpha1.GroupName}
		mapping, err = o.Mapper.RESTMapping(gk, "v1alpha1")
		mapping.Scope = meta.RESTScopeRoot
		if err != nil {
			return fmt.Errorf("unable to recognize resource: %v", err)
		}
		o.karmadaMapping[gk] = mapping
		cppRESTClient, err := f.ClientForMapping(mapping)
		if err != nil {
			return fmt.Errorf("unable to connect to a server to handle %q: %v", mapping.Resource, err)
		}
		o.karmadaRESTClient[gk] = cppRESTClient
	}

	return nil
}

// Validate checks the set of flags provided by the user.
// refer to https://github.com/kubernetes/kubernetes/blob/v1.23.4/staging/src/k8s.io/kubectl/pkg/cmd/delete/delete.go#L229
//gocyclo:ignore
func (o *DeleteOptions) Validate() error {
	if o.Output != "" && o.Output != "name" {
		return fmt.Errorf("unexpected -o output mode: %v. We only support '-o name'", o.Output)
	}

	if o.DeleteAll && len(o.LabelSelector) > 0 {
		return fmt.Errorf("cannot set --all and --selector at the same time")
	}
	if o.DeleteAll && len(o.FieldSelector) > 0 {
		return fmt.Errorf("cannot set --all and --field-selector at the same time")
	}

	switch {
	case o.GracePeriod == 0 && o.ForceDeletion:
		fmt.Fprintf(o.ErrOut, "warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.\n")
	case o.GracePeriod > 0 && o.ForceDeletion:
		return fmt.Errorf("--force and --grace-period greater than 0 cannot be specified together")
	}

	if len(o.Raw) > 0 {
		if len(o.FilenameOptions.Filenames) > 1 {
			return fmt.Errorf("--raw can only use a single local file or stdin")
		} else if len(o.FilenameOptions.Filenames) == 1 {
			if strings.Index(o.FilenameOptions.Filenames[0], "http://") == 0 || strings.Index(o.FilenameOptions.Filenames[0], "https://") == 0 {
				return fmt.Errorf("--raw cannot read from a url")
			}
		}

		if o.FilenameOptions.Recursive {
			return fmt.Errorf("--raw and --recursive are mutually exclusive")
		}
		if len(o.Output) > 0 {
			return fmt.Errorf("--raw and --output are mutually exclusive")
		}
		if _, err := url.ParseRequestURI(o.Raw); err != nil {
			return fmt.Errorf("--raw must be a valid URL path: %v", err)
		}
	}

	return nil
}

// RunDelete performs the delete operation.
// refer to https://github.com/kubernetes/kubernetes/blob/v1.23.4/staging/src/k8s.io/kubectl/pkg/cmd/delete/delete.go#L271
//gocyclo:ignore
func (o *DeleteOptions) RunDelete(f cmdutil.Factory) error {
	if len(o.Raw) > 0 {
		// No need to delete PropagationPolicy and ClusterPropagationPolicy
		restClient, err := f.RESTClient()
		if err != nil {
			return err
		}
		if len(o.Filenames) == 0 {
			return rawhttp.RawDelete(restClient, o.IOStreams, o.Raw, "")
		}
		return rawhttp.RawDelete(restClient, o.IOStreams, o.Raw, o.Filenames[0])
	}
	return o.DeleteResult(o.Result)
}

// DeleteResult performs the delete operation.
//gocyclo:ignore
func (o *DeleteOptions) DeleteResult(r *resource.Result) error {
	found := 0
	if o.IgnoreNotFound {
		r = r.IgnoreErrors(apierrors.IsNotFound)
	}
	warnClusterScope := o.WarnClusterScope
	deletedInfos := []*resource.Info{}
	uidMap := cmdwait.UIDMap{}
	responses := make(map[runtime.Object]*resource.Info)
	err := r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		deletedInfos = append(deletedInfos, info)
		found++

		opts := &metav1.DeleteOptions{}
		if o.GracePeriod >= 0 {
			opts = metav1.NewDeleteOptions(int64(o.GracePeriod))
		}
		opts.PropagationPolicy = &o.CascadingStrategy

		if warnClusterScope && info.Mapping.Scope.Name() == meta.RESTScopeNameRoot {
			fmt.Fprintf(o.ErrOut, "warning: deleting cluster-scoped resources, not scoped to the provided namespace\n")
			warnClusterScope = false
		}

		if o.DryRunStrategy == cmdutil.DryRunClient {
			if !o.Quiet {
				o.PrintObj(info)
			}
			return nil
		}
		if o.DryRunStrategy == cmdutil.DryRunServer {
			if err := o.DryRunVerifier.HasSupport(info.Mapping.GroupVersionKind); err != nil {
				return err
			}
		}
		response, err := o.deleteResource(info, opts)
		if err != nil {
			return err
		}
		responses[response] = info
		objectMeta, _ := meta.Accessor(info.Object)
		labels := objectMeta.GetLabels()
		var policyInfo *resource.Info
		if info.Namespaced() {
			policyName, ok := labels[policyv1alpha1.PropagationPolicyNameLabel]
			if ok {
				gk := schema.GroupKind{Kind: policyv1alpha1.ResourceKindPropagationPolicy, Group: policyv1alpha1.GroupName}
				policyInfo = &resource.Info{
					Namespace: info.Namespace,
					Name:      policyName,
					Object:    nil,
					Mapping:   o.karmadaMapping[gk],
					Client:    o.karmadaRESTClient[gk],
				}
			}
		} else {
			clusterPolicyName, ok := labels[policyv1alpha1.ClusterPropagationPolicyLabel]
			if ok {
				gk := schema.GroupKind{Kind: policyv1alpha1.ResourceKindClusterPropagationPolicy, Group: policyv1alpha1.GroupName}
				policyInfo = &resource.Info{
					Namespace: "",
					Name:      clusterPolicyName,
					Object:    nil,
					Mapping:   o.karmadaMapping[gk],
					Client:    o.karmadaRESTClient[gk],
				}
			}
		}
		if policyInfo == nil {
			return nil
		}
		deletedInfos = append(deletedInfos, policyInfo)
		found++
		response, err = o.deleteResource(policyInfo, opts)
		if err != nil {
			return err
		}
		responses[response] = policyInfo
		for k, v := range responses {
			resourceLocation := cmdwait.ResourceLocation{
				GroupResource: v.Mapping.Resource.GroupResource(),
				Namespace:     v.Namespace,
				Name:          v.Name,
			}
			if status, ok := k.(*metav1.Status); ok && status.Details != nil {
				uidMap[resourceLocation] = status.Details.UID
				continue
			}
			responseMetadata, err := meta.Accessor(k)
			if err != nil {
				// we don't have UID, but we didn't fail the delete, next best thing is just skipping the UID
				klog.V(1).Info(err)
				continue
			}
			uidMap[resourceLocation] = responseMetadata.GetUID()
		}

		return nil
	})
	if err != nil {
		return err
	}
	if found == 0 {
		fmt.Fprintf(o.Out, "No resources found\n")
		return nil
	}
	if !o.WaitForDeletion {
		return nil
	}
	// if we don't have a dynamic client, we don't want to wait.  Eventually when delete is cleaned up, this will likely
	// drop out.
	if o.DynamicClient == nil {
		return nil
	}

	// If we are dry-running, then we don't want to wait
	if o.DryRunStrategy != cmdutil.DryRunNone {
		return nil
	}

	effectiveTimeout := o.Timeout
	if effectiveTimeout == 0 {
		// if we requested to wait forever, set it to a week.
		effectiveTimeout = 168 * time.Hour
	}
	waitOptions := cmdwait.WaitOptions{
		ResourceFinder: genericclioptions.ResourceFinderForResult(resource.InfoListVisitor(deletedInfos)),
		UIDMap:         uidMap,
		DynamicClient:  o.DynamicClient,
		Timeout:        effectiveTimeout,

		Printer:     printers.NewDiscardingPrinter(),
		ConditionFn: cmdwait.IsDeleted,
		IOStreams:   o.IOStreams,
	}
	err = waitOptions.RunWait()
	if apierrors.IsForbidden(err) || apierrors.IsMethodNotSupported(err) {
		// if we're forbidden from waiting, we shouldn't fail.
		// if the resource doesn't support a verb we need, we shouldn't fail.
		klog.V(1).Info(err)
		return nil
	}
	return err
}

func (o *DeleteOptions) deleteResource(info *resource.Info, deleteOptions *metav1.DeleteOptions) (runtime.Object, error) {
	deleteResponse, err := resource.
		NewHelper(info.Client, info.Mapping).
		DryRun(o.DryRunStrategy == cmdutil.DryRunServer).
		DeleteWithOptions(info.Namespace, info.Name, deleteOptions)
	if err != nil {
		return nil, cmdutil.AddSourceToErr("deleting", info.Source, err)
	}

	if !o.Quiet {
		o.PrintObj(info)
	}
	return deleteResponse, nil
}

// PrintObj for deleted objects is special because we do not have an object to print.
// This mirrors name printer behavior
func (o *DeleteOptions) PrintObj(info *resource.Info) {
	operation := "deleted"
	groupKind := info.Mapping.GroupVersionKind
	kindString := fmt.Sprintf("%s.%s", strings.ToLower(groupKind.Kind), groupKind.Group)
	if len(groupKind.Group) == 0 {
		kindString = strings.ToLower(groupKind.Kind)
	}

	if o.GracePeriod == 0 {
		operation = "force deleted"
	}

	switch o.DryRunStrategy {
	case cmdutil.DryRunClient:
		operation = fmt.Sprintf("%s (dry run)", operation)
	case cmdutil.DryRunServer:
		operation = fmt.Sprintf("%s (server dry run)", operation)
	}

	if o.Output == "name" {
		// -o name: prints resource/name
		fmt.Fprintf(o.Out, "%s/%s\n", kindString, info.Name)
		return
	}

	// understandable output by default
	fmt.Fprintf(o.Out, "%s \"%s\" %s\n", kindString, info.Name, operation)
}

// DeleteFlags composes common printer flag structs
// used for commands requiring deletion logic.
type DeleteFlags struct {
	FileNameFlags     *genericclioptions.FileNameFlags
	LabelSelector     *string
	FieldSelector     *string
	Namespace         *string
	All               *bool
	AllNamespaces     *bool
	CascadingStrategy *string
	Force             *bool
	GracePeriod       *int
	IgnoreNotFound    *bool
	Now               *bool
	Timeout           *time.Duration
	Wait              *bool
	Output            *string
	Raw               *string
	KubeConfig        *string
	KarmadaContext    *string
}

// refer to https://github.com/kubernetes/kubernetes/blob/v1.23.4/staging/src/k8s.io/kubectl/pkg/cmd/delete/delete_flags.go#L51
//gocyclo:ignore
func (f *DeleteFlags) toOptions(dynamicClient dynamic.Interface, streams genericclioptions.IOStreams) (*DeleteOptions, error) {
	o := &DeleteOptions{
		DynamicClient: dynamicClient,
		IOStreams:     streams,
	}

	// add filename o
	if f.FileNameFlags != nil {
		o.FilenameOptions = f.FileNameFlags.ToOptions()
	}
	if f.LabelSelector != nil {
		o.LabelSelector = *f.LabelSelector
	}
	if f.FieldSelector != nil {
		o.FieldSelector = *f.FieldSelector
	}

	// add output format
	if f.Output != nil {
		o.Output = *f.Output
	}

	if f.All != nil {
		o.DeleteAll = *f.All
	}
	if f.AllNamespaces != nil {
		o.DeleteAllNamespaces = *f.AllNamespaces
	}
	if f.CascadingStrategy != nil {
		var err error
		o.CascadingStrategy, err = parseCascadingFlag(streams, *f.CascadingStrategy)
		if err != nil {
			return nil, err
		}
	}
	if f.Force != nil {
		o.ForceDeletion = *f.Force
	}
	if f.GracePeriod != nil {
		o.GracePeriod = *f.GracePeriod
	}
	if f.IgnoreNotFound != nil {
		o.IgnoreNotFound = *f.IgnoreNotFound
	}
	if f.Now != nil {
		o.DeleteNow = *f.Now
	}
	if f.Timeout != nil {
		o.Timeout = *f.Timeout
	}
	if f.Wait != nil {
		o.WaitForDeletion = *f.Wait
	}
	if f.Raw != nil {
		o.Raw = *f.Raw
	}
	if f.KubeConfig != nil {
		o.KubeConfig = *f.KubeConfig
	}
	if f.KarmadaContext != nil {
		o.KarmadaContext = *f.KarmadaContext
	}
	if f.Namespace != nil {
		o.Namespace = *f.Namespace
	}
	return o, nil
}

// AddFlags adds flags to the specified FlagSet.
//gocyclo:ignore
func (f *DeleteFlags) AddFlags(cmd *cobra.Command) {
	f.FileNameFlags.AddFlags(cmd.Flags())
	if f.LabelSelector != nil {
		cmd.Flags().StringVarP(f.LabelSelector, "selector", "l", *f.LabelSelector, "Selector (label query) to filter on.")
	}
	if f.FieldSelector != nil {
		cmd.Flags().StringVarP(f.FieldSelector, "field-selector", "", *f.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
	}
	if f.All != nil {
		cmd.Flags().BoolVar(f.All, "all", *f.All, "Delete all resources, in the namespace of the specified resource types.")
	}
	if f.AllNamespaces != nil {
		cmd.Flags().BoolVarP(f.AllNamespaces, "all-namespaces", "A", *f.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	}
	if f.Force != nil {
		cmd.Flags().BoolVar(f.Force, "force", *f.Force, "If true, immediately remove resources from API and bypass graceful deletion. Note that immediate deletion of some resources may result in inconsistency or data loss and requires confirmation.")
	}
	if f.CascadingStrategy != nil {
		cmd.Flags().StringVar(
			f.CascadingStrategy,
			"cascade",
			*f.CascadingStrategy,
			`Must be "background", "orphan", or "foreground". Selects the deletion cascading strategy for the dependents (e.g. Pods created by a ReplicationController). Defaults to background.`)
		cmd.Flags().Lookup("cascade").NoOptDefVal = "background"
	}
	if f.Now != nil {
		cmd.Flags().BoolVar(f.Now, "now", *f.Now, "If true, resources are signaled for immediate shutdown (same as --grace-period=1).")
	}
	if f.GracePeriod != nil {
		cmd.Flags().IntVar(f.GracePeriod, "grace-period", *f.GracePeriod, "Period of time in seconds given to the resource to terminate gracefully. Ignored if negative. Set to 1 for immediate shutdown. Can only be set to 0 when --force is true (force deletion).")
	}
	if f.Timeout != nil {
		cmd.Flags().DurationVar(f.Timeout, "timeout", *f.Timeout, "The length of time to wait before giving up on a delete, zero means determine a timeout from the size of the object")
	}
	if f.IgnoreNotFound != nil {
		cmd.Flags().BoolVar(f.IgnoreNotFound, "ignore-not-found", *f.IgnoreNotFound, "Treat \"resource not found\" as a successful delete. Defaults to \"true\" when --all is specified.")
	}
	if f.Wait != nil {
		cmd.Flags().BoolVar(f.Wait, "wait", *f.Wait, "If true, wait for resources to be gone before returning. This waits for finalizers.")
	}
	if f.Output != nil {
		cmd.Flags().StringVarP(f.Output, "output", "o", *f.Output, "Output mode. Use \"-o name\" for shorter output (resource/name).")
	}
	if f.Raw != nil {
		cmd.Flags().StringVar(f.Raw, "raw", *f.Raw, "Raw URI to DELETE to the server.  Uses the transport specified by the kubeconfig file.")
	}
	if f.KarmadaContext != nil {
		cmd.Flags().StringVar(f.KarmadaContext, "karmada-context", "", "Name of the cluster context in control plane kubeconfig file.")
	}
	if f.KubeConfig != nil {
		cmd.Flags().StringVar(f.KubeConfig, "kubeconfig", "", "Path to the control plane kubeconfig file.")
	}
	if f.Namespace != nil {
		cmd.Flags().StringVarP(f.Namespace, "namespace", "n", "default", "-n=namespace or -n namespace")
	}
}

// NewDeleteCommandFlags provides default flags and values for use with the "delete" command
func NewDeleteCommandFlags(usage string) *DeleteFlags {
	cascadingStrategy := "background"
	gracePeriod := -1

	// setup command defaults
	all := false
	allNamespaces := false
	force := false
	ignoreNotFound := false
	now := false
	output := ""
	labelSelector := ""
	fieldSelector := ""
	timeout := time.Duration(0)
	wait := true
	raw := ""

	filenames := []string{}
	recursive := false
	kustomize := ""
	karmadaContext := ""
	kubeConfig := ""
	namespace := ""
	return &DeleteFlags{
		// Not using helpers.go since it provides function to add '-k' for FileNameOptions, but not FileNameFlags
		FileNameFlags: &genericclioptions.FileNameFlags{Usage: usage, Filenames: &filenames, Kustomize: &kustomize, Recursive: &recursive},
		LabelSelector: &labelSelector,
		FieldSelector: &fieldSelector,

		CascadingStrategy: &cascadingStrategy,
		GracePeriod:       &gracePeriod,

		All:            &all,
		AllNamespaces:  &allNamespaces,
		Force:          &force,
		IgnoreNotFound: &ignoreNotFound,
		Now:            &now,
		Timeout:        &timeout,
		Wait:           &wait,
		Output:         &output,
		Raw:            &raw,
		KarmadaContext: &karmadaContext,
		KubeConfig:     &kubeConfig,
		Namespace:      &namespace,
	}
}

// NewDeleteFlags provides default flags and values for use in commands outside of "delete"
func NewDeleteFlags(usage string) *DeleteFlags {
	cascadingStrategy := "background"
	gracePeriod := -1

	force := false
	timeout := time.Duration(0)
	wait := false

	filenames := []string{}
	kustomize := ""
	recursive := false

	return &DeleteFlags{
		FileNameFlags: &genericclioptions.FileNameFlags{Usage: usage, Filenames: &filenames, Kustomize: &kustomize, Recursive: &recursive},

		CascadingStrategy: &cascadingStrategy,
		GracePeriod:       &gracePeriod,

		// add non-defaults
		Force:   &force,
		Timeout: &timeout,
		Wait:    &wait,
	}
}

func parseCascadingFlag(streams genericclioptions.IOStreams, cascadingFlag string) (metav1.DeletionPropagation, error) {
	boolValue, err := strconv.ParseBool(cascadingFlag)
	// The flag is not a boolean
	if err != nil {
		switch cascadingFlag {
		case "orphan":
			return metav1.DeletePropagationOrphan, nil
		case "foreground":
			return metav1.DeletePropagationForeground, nil
		case "background":
			return metav1.DeletePropagationBackground, nil
		default:
			return metav1.DeletePropagationBackground, fmt.Errorf(`invalid cascade value (%v). Must be "background", "foreground", or "orphan"`, cascadingFlag)
		}
	}
	// The flag was a boolean
	if boolValue {
		fmt.Fprintf(streams.ErrOut, "warning: --cascade=%v is deprecated (boolean value) and can be replaced with --cascade=%s.\n", cascadingFlag, "background")
		return metav1.DeletePropagationBackground, nil
	}
	fmt.Fprintf(streams.ErrOut, "warning: --cascade=%v is deprecated (boolean value) and can be replaced with --cascade=%s.\n", cascadingFlag, "orphan")
	return metav1.DeletePropagationOrphan, nil
}
