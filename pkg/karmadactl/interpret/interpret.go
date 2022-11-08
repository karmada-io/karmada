package interpret

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

var (
	interpretLong = templates.LongDesc(`
        Validate and test interpreter customization before applying it to the control plane.

        1. Validate the ResourceInterpreterCustomization configuration as per API schema
           and try to load the scripts for syntax check.

        2. Run the rules locally and test if the result is expected. Similar to the dry run.

`)

	interpretExample = templates.Examples(`
        # Check the customizations in file
        %[1]s interpret -f customization.json --check
		# Execute the retention rule for
        %[1]s interpret -f customization.yml --operation retain --desired-file desired.yml --observed-file observed.yml
		# Execute the replicaRevision rule for
        %[1]s interpret -f customization.yml --operation reviseReplica --observed-file observed.yml --desired-replica 2
		# Execute the statusReflection rule for
        %[1]s interpret -f customization.yml --operation interpretStatus --observed-file observed.yml
		# Execute the healthInterpretation rule
        %[1]s interpret -f customization.yml --operation interpretHealth --observed-file observed.yml
		# Execute the dependencyInterpretation rule 
        %[1]s interpret -f customization.yml --operation interpretDependency --observed-file observed.yml
		# Execute the statusAggregation rule
        %[1]s interpret -f customization.yml --operation aggregateStatus --status-file status1.yml --status-file status2.yml

`)
)

const (
	customizationResourceName = "resourceinterpretercustomizations"
)

// NewCmdInterpret new interpret command.
func NewCmdInterpret(f util.Factory, parentCommand string, streams genericclioptions.IOStreams) *cobra.Command {
	o := &Options{
		IOStreams: streams,
		Rules:     allRules,
	}
	cmd := &cobra.Command{
		Use:                   "interpret (-f FILENAME) (--operation OPERATION) [--ARGS VALUE]... ",
		Short:                 "Validate and test interpreter customization before applying it to the control plane",
		Long:                  interpretLong,
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		Example:               fmt.Sprintf(interpretExample, parentCommand),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterTroubleshootingAndDebugging,
		},
	}

	flags := cmd.Flags()
	options.AddKubeConfigFlags(flags)
	flags.StringVar(&o.Operation, "operation", o.Operation, "The interpret operation to use. One of: ("+strings.Join(o.Rules.Names(), ",")+")")
	flags.BoolVar(&o.Check, "check", false, "Validates the given ResourceInterpreterCustomization configuration(s)")
	flags.StringVar(&o.DesiredFile, "desired-file", o.DesiredFile, "Filename, directory, or URL to files identifying the resource to use as desiredObj argument in rule script.")
	flags.StringVar(&o.ObservedFile, "observed-file", o.ObservedFile, "Filename, directory, or URL to files identifying the resource to use as observedObj argument in rule script.")
	flags.StringSliceVar(&o.StatusFile, "status-file", o.StatusFile, "Filename, directory, or URL to files identifying the resource to use as statusItems argument in rule script.")
	flags.Int32Var(&o.DesiredReplica, "desired-replica", o.DesiredReplica, "The desiredReplica argument in rule script.")
	cmdutil.AddJsonFilenameFlag(flags, &o.FilenameOptions.Filenames, "Filename, directory, or URL to files containing the customizations")
	flags.BoolVarP(&o.FilenameOptions.Recursive, "recursive", "R", false, "Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.")

	return cmd
}

// Options contains the input to the interpret command.
type Options struct {
	resource.FilenameOptions

	Operation string
	Check     bool

	// args
	DesiredFile    string
	ObservedFile   string
	StatusFile     []string
	DesiredReplica int32

	CustomizationResult *resource.Result

	Rules Rules

	genericclioptions.IOStreams
}

// Complete ensures that options are valid and marshals them if necessary
func (o *Options) Complete(f util.Factory, cmd *cobra.Command, args []string) error {
	scheme := gclient.NewSchema()
	o.CustomizationResult = f.NewBuilder().
		WithScheme(scheme, scheme.PrioritizedVersionsAllGroups()...).
		FilenameParam(false, &o.FilenameOptions).
		ResourceNames(customizationResourceName, args...).
		RequireObject(true).
		Local().
		Do()

	var errs []error
	errs = append(errs, o.CustomizationResult.Err())
	errs = append(errs, o.completeExecute(f, cmd, args)...)
	return errors.NewAggregate(errs)
}

// Validate checks the EditOptions to see if there is sufficient information to run the command.
func (o *Options) Validate() error {
	return nil
}

// Run describe information of resources
func (o *Options) Run() error {
	switch {
	case o.Check:
		return o.runCheck()
	default:
		return o.runExecute()
	}
}

func asResourceInterpreterCustomization(o runtime.Object) (*configv1alpha1.ResourceInterpreterCustomization, error) {
	c, ok := o.(*configv1alpha1.ResourceInterpreterCustomization)
	if !ok {
		return nil, fmt.Errorf("not a ResourceInterpreterCustomization: %#v", o)
	}
	return c, nil
}
