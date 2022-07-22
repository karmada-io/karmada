package karmadactl

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

// NewCmdDescribe new describe command.
func NewCmdDescribe(karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	ioStreams := genericclioptions.IOStreams{In: getIn, Out: getOut, ErrOut: getErr}
	o := &CommandDescribeOptions{
		FilenameOptions: &resource.FilenameOptions{},
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
			ChunkSize:  cmdutil.DefaultChunkSize,
		},

		CmdParent: parentCommand,

		IOStreams: ioStreams,
	}

	cmd := &cobra.Command{
		Use:          "describe (-f FILENAME | TYPE [NAME_PREFIX | -l label] | TYPE/NAME) (-C CLUSTER)",
		Short:        "Show details of a specific resource or group of resources in a cluster",
		SilenceUsage: true,
		Example:      describeExample(parentCommand),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(karmadaConfig, args); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	o.GlobalCommandOptions.AddFlags(cmd.Flags())

	usage := "containing the resource to describe"
	cmdutil.AddFilenameOptionFlags(cmd, o.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.Cluster, "cluster", "C", "", "Specify a member cluster")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", o.Namespace, "If present, the namespace scope for this CLI request")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&o.DescriberSettings.ShowEvents, "show-events", o.DescriberSettings.ShowEvents, "If true, display events related to the described object.")
	cmdutil.AddChunkSizeFlag(cmd, &o.DescriberSettings.ChunkSize)

	return cmd
}

// CommandDescribeOptions contains the input to the describe command.
type CommandDescribeOptions struct {
	// global flags
	options.GlobalCommandOptions

	Cluster   string
	CmdParent string
	Selector  string
	Namespace string

	Describer  func(*meta.RESTMapping) (describe.ResourceDescriber, error)
	NewBuilder func() *resource.Builder

	BuilderArgs []string

	EnforceNamespace bool
	AllNamespaces    bool

	DescriberSettings *describe.DescriberSettings
	FilenameOptions   *resource.FilenameOptions

	genericclioptions.IOStreams
}

func describeExample(parentCommand string) string {
	example := `
# Describe a pod in cluster(member1)` + "\n" +
		fmt.Sprintf("%s describe pods/nginx -C=member1", parentCommand) + `

# Describe all pods in cluster(member1)` + "\n" +
		fmt.Sprintf("%s describe pods -C=member1", parentCommand) + `

# # Describe a pod identified by type and name in "pod.json" in cluster(member1)` + "\n" +
		fmt.Sprintf("%s describe -f pod.json -C=member1", parentCommand) + `

# Describe pods by label name=myLabel in cluster(member1)` + "\n" +
		fmt.Sprintf("%s describe po -l name=myLabel -C=member1", parentCommand) + `

# Describe all pods managed by the 'frontend' replication controller  in cluster(member1)
# (rc-created pods get the name of the rc as a prefix in the pod name)` + "\n" +
		fmt.Sprintf("%s describe pods frontend -C=member1", parentCommand)
	return example
}

// Complete ensures that options are valid and marshals them if necessary
func (o *CommandDescribeOptions) Complete(karmadaConfig KarmadaConfig, args []string) error {
	var err error

	if len(o.Cluster) == 0 {
		return fmt.Errorf("must specify a cluster")
	}

	karmadaRestConfig, err := karmadaConfig.GetRestConfig(o.KarmadaContext, o.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			o.KarmadaContext, o.KubeConfig, err)
	}

	clusterInfo, err := getClusterInfo(karmadaRestConfig, o.Cluster, o.KubeConfig, o.KarmadaContext)
	if err != nil {
		return err
	}

	f := getFactory(o.Cluster, clusterInfo, "")

	namespace, enforceNamespace, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.Namespace == "" {
		o.Namespace = namespace
	}
	o.EnforceNamespace = enforceNamespace

	if o.AllNamespaces {
		o.EnforceNamespace = false
	}

	if len(args) == 0 && cmdutil.IsFilenameSliceEmpty(o.FilenameOptions.Filenames, o.FilenameOptions.Kustomize) {
		return fmt.Errorf("must specify the type of resource to describe. %s", cmdutil.SuggestAPIResources(o.CmdParent))
	}

	o.BuilderArgs = args

	o.Describer = func(mapping *meta.RESTMapping) (describe.ResourceDescriber, error) {
		return describe.DescriberFn(f, mapping)
	}

	o.NewBuilder = f.NewBuilder

	return nil
}

// Run describe information of resources
func (o *CommandDescribeOptions) Run() error {
	r := o.NewBuilder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		FilenameParam(o.EnforceNamespace, o.FilenameOptions).
		LabelSelectorParam(o.Selector).
		ResourceTypeOrNameArgs(true, o.BuilderArgs...).
		RequestChunksOf(o.DescriberSettings.ChunkSize).
		Flatten().
		Do()
	err := r.Err()
	if err != nil {
		return err
	}

	allErrs := []error{}
	infos, err := r.Infos()
	if err != nil {
		if apierrors.IsNotFound(err) && len(o.BuilderArgs) == 2 {
			return o.describeMatchingResources(err, o.BuilderArgs[0], o.BuilderArgs[1])
		}
		allErrs = append(allErrs, err)
	}

	errs := sets.NewString()
	first := true
	for _, info := range infos {
		mapping := info.ResourceMapping()
		describer, err := o.Describer(mapping)
		if err != nil {
			if errs.Has(err.Error()) {
				continue
			}
			allErrs = append(allErrs, err)
			errs.Insert(err.Error())
			continue
		}
		s, err := describer.Describe(info.Namespace, info.Name, *o.DescriberSettings)
		if err != nil {
			if errs.Has(err.Error()) {
				continue
			}
			allErrs = append(allErrs, err)
			errs.Insert(err.Error())
			continue
		}
		if first {
			first = false
			fmt.Fprint(o.Out, s)
		} else {
			fmt.Fprintf(o.Out, "\n\n%s", s)
		}
	}

	if len(infos) == 0 && len(allErrs) == 0 {
		// if we wrote no output, and had no errors, be sure we output something.
		if o.AllNamespaces {
			fmt.Fprintln(o.ErrOut, "No resources found")
		} else {
			fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		}
	}

	return utilerrors.NewAggregate(allErrs)
}

func (o *CommandDescribeOptions) describeMatchingResources(originalError error, resource, prefix string) error {
	r := o.NewBuilder().
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().
		ResourceTypeOrNameArgs(true, resource).
		SingleResourceType().
		RequestChunksOf(o.DescriberSettings.ChunkSize).
		Flatten().
		Do()
	mapping, err := r.ResourceMapping()
	if err != nil {
		return err
	}
	describer, err := o.Describer(mapping)
	if err != nil {
		return err
	}
	infos, err := r.Infos()
	if err != nil {
		return err
	}
	isFound := false
	for ix := range infos {
		info := infos[ix]
		if strings.HasPrefix(info.Name, prefix) {
			isFound = true
			s, err := describer.Describe(info.Namespace, info.Name, *o.DescriberSettings)
			if err != nil {
				return err
			}
			fmt.Fprintf(o.Out, "%s\n", s)
		}
	}
	if !isFound {
		return originalError
	}
	return nil
}
