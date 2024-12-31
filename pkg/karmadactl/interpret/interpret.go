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

package interpret

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/cmd/util/editor"
	"k8s.io/kubectl/pkg/util/templates"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/genericresource"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/interpreter"
)

var (
	interpretLong = templates.LongDesc(`
        Validate, test and edit interpreter customization before applying it to the control plane.

		1. Validate the ResourceInterpreterCustomization configuration as per API schema
           and try to load the scripts for syntax check.

		2. Run the rules locally and test if the result is expected. Similar to the dry run.

		3. Edit customization. Similar to the kubectl edit.
	`)

	interpretExample = templates.Examples(`
        # Check the customizations in file
        %[1]s interpret -f customization.json --check

		# Execute the retention rule
        %[1]s interpret -f customization.yml --operation retain --desired-file desired.yml --observed-file observed.yml

		# Execute the replicaResource rule
        %[1]s interpret -f customization.yml --operation interpretReplica --observed-file observed.yml

		# Execute the replicaRevision rule
        %[1]s interpret -f customization.yml --operation reviseReplica --observed-file observed.yml --desired-replica 2

		# Execute the statusReflection rule
        %[1]s interpret -f customization.yml --operation interpretStatus --observed-file observed.yml

		# Execute the healthInterpretation rule
        %[1]s interpret -f customization.yml --operation interpretHealth --observed-file observed.yml

		# Execute the dependencyInterpretation rule 
        %[1]s interpret -f customization.yml --operation interpretDependency --observed-file observed.yml

		# Execute the statusAggregation rule
        %[1]s interpret -f customization.yml --operation aggregateStatus --observed-file observed.yml --status-file status.yml

		# Fetch observed object from url, and status items from stdin (specified with -)
        %[1]s interpret -f customization.yml --operation aggregateStatus --observed-file https://example.com/observed.yml --status-file -

		# Edit customization
		%[1]s interpret -f customization.yml --edit
	`)
)

const (
	customizationResourceName = "resourceinterpretercustomizations"
)

// NewCmdInterpret new interpret command.
func NewCmdInterpret(f util.Factory, parentCommand string, streams genericiooptions.IOStreams) *cobra.Command {
	editorFlags := editor.NewEditOptions(editor.NormalEditMode, streams)
	editorFlags.PrintFlags = editorFlags.PrintFlags.WithTypeSetter(gclient.NewSchema())

	o := &Options{
		EditOptions: editorFlags,
		IOStreams:   streams,
		Rules:       interpreter.AllResourceInterpreterCustomizationRules,
	}
	cmd := &cobra.Command{
		Use:                   "interpret (-f FILENAME) (--operation OPERATION) [--ARGS VALUE]... ",
		Short:                 "Validate, test and edit interpreter customization before applying it to the control plane",
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
	o.EditOptions.RecordFlags.AddFlags(cmd)
	o.EditOptions.PrintFlags.AddFlags(cmd)
	flags.StringVar(&o.Operation, "operation", o.Operation, "The interpret operation to use. One of: ("+strings.Join(o.Rules.Names(), ",")+")")
	flags.BoolVar(&o.Check, "check", false, "Validates the given ResourceInterpreterCustomization configuration(s)")
	flags.BoolVar(&o.Edit, "edit", false, "Edit customizations")
	flags.BoolVar(&o.ShowDoc, "show-doc", false, "Show document of rules when editing")
	flags.StringVar(&o.DesiredFile, "desired-file", o.DesiredFile, "Filename, directory, or URL to files identifying the resource to use as desiredObj argument in rule script.")
	flags.StringVar(&o.ObservedFile, "observed-file", o.ObservedFile, "Filename, directory, or URL to files identifying the resource to use as observedObj argument in rule script.")
	flags.StringVar(&o.StatusFile, "status-file", o.StatusFile, "Filename, directory, or URL to files identifying the resource to use as statusItems argument in rule script.")
	flags.Int32Var(&o.DesiredReplica, "desired-replica", o.DesiredReplica, "The desiredReplica argument in rule script.")
	cmdutil.AddJsonFilenameFlag(flags, &o.FilenameOptions.Filenames, "Filename, directory, or URL to files containing the customizations")
	flags.BoolVarP(&o.FilenameOptions.Recursive, "recursive", "R", false, "Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.")

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	return cmd
}

// Options contains the input to the interpret command.
type Options struct {
	resource.FilenameOptions
	*editor.EditOptions

	Operation string
	Check     bool
	Edit      bool
	ShowDoc   bool

	// args
	DesiredFile    string
	ObservedFile   string
	StatusFile     string
	DesiredReplica int32

	CustomizationResult *resource.Result
	DesiredResult       *resource.Result
	ObservedResult      *resource.Result
	StatusResult        *genericresource.Result

	Rules interpreter.Rules

	genericiooptions.IOStreams
}

// Complete ensures that options are valid and marshals them if necessary
func (o *Options) Complete(f util.Factory, _ *cobra.Command, args []string) error {
	if o.Check && o.Edit {
		return fmt.Errorf("you can't set both --check and --edit options")
	}

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
	errs = append(errs, o.completeExecute(f)...)
	errs = append(errs, o.completeEdit())
	return errors.NewAggregate(errs)
}

// Validate validates Options.
func (o *Options) Validate() error {
	if o.Operation != "" {
		r := o.Rules.GetByOperation(o.Operation)
		if r == nil {
			return fmt.Errorf("operation %s is not supported. Use one of: %s", o.Operation, strings.Join(o.Rules.Names(), ", "))
		}
	}
	if o.Edit {
		err := o.EditOptions.Validate()
		if err != nil {
			return err
		}
	}
	return nil
}

// Run describe information of resources
func (o *Options) Run() error {
	switch {
	case o.Check:
		return o.runCheck()
	case o.Edit:
		return o.runEdit()
	default:
		return o.runExecute()
	}
}

func (o *Options) getCustomizationObject() ([]*configv1alpha1.ResourceInterpreterCustomization, error) {
	infos, err := o.CustomizationResult.Infos()
	if err != nil {
		return nil, err
	}

	customizations := make([]*configv1alpha1.ResourceInterpreterCustomization, len(infos))
	for i, info := range infos {
		c, err := asResourceInterpreterCustomization(info.Object)
		if err != nil {
			return nil, err
		}
		customizations[i] = c
	}
	return customizations, nil
}

func (o *Options) getAggregatedStatusItems() ([]workv1alpha2.AggregatedStatusItem, error) {
	if o.StatusResult == nil {
		return nil, nil
	}

	objs, err := o.StatusResult.Objects()
	if err != nil {
		return nil, err
	}
	items := make([]workv1alpha2.AggregatedStatusItem, len(objs))
	for i, obj := range objs {
		items[i] = *(obj.(*workv1alpha2.AggregatedStatusItem))
	}
	return items, nil
}

func getUnstructuredObjectFromResult(result *resource.Result) (*unstructured.Unstructured, error) {
	if result == nil {
		return nil, nil
	}

	infos, err := result.Infos()
	if err != nil {
		return nil, err
	}

	if len(infos) > 1 {
		return nil, fmt.Errorf("get %v objects, expect one at most", len(infos))
	}

	return helper.ToUnstructured(infos[0].Object)
}

func asResourceInterpreterCustomization(o runtime.Object) (*configv1alpha1.ResourceInterpreterCustomization, error) {
	c, ok := o.(*configv1alpha1.ResourceInterpreterCustomization)
	if !ok {
		return nil, fmt.Errorf("not a ResourceInterpreterCustomization, got %v", o.GetObjectKind().GroupVersionKind())
	}
	return c, nil
}
