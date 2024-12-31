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

package annotate

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectlannotate "k8s.io/kubectl/pkg/cmd/annotate"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	annotateExample = templates.Examples(`
    # Update deployment 'foo' with the annotation 'work.karmada.io/conflict-resolution' and the value 'overwrite'
    # If the same annotation is set multiple times, only the last value will be applied
    [1]%s annotate deployment foo work.karmada.io/conflict-resolution='overwrite'

    # Update a deployment identified by type and name in "deployment.json"
    [1]%s annotate -f deployment.json work.karmada.io/conflict-resolution='overwrite'

    # Update deployment 'foo' with the annotation 'work.karmada.io/conflict-resolution' and the value 'abort', overwriting any existing value
    [1]%s annotate --overwrite deployment foo work.karmada.io/conflict-resolution='abort'

    # Update all deployments in the namespace
    [1]%s annotate deployment --all work.karmada.io/conflict-resolution='abort'

    # Update deployment 'foo' only if the resource is unchanged from version 1
    [1]%s annotate deployment foo work.karmada.io/conflict-resolution='abort' --resource-version=1

    # Update deployment 'foo' by removing an annotation named 'work.karmada.io/conflict-resolution' if it exists
    # Does not require the --overwrite flag
    [1]%s annotate deployment foo work.karmada.io/conflict-resolution-`)
)

// NewCmdAnnotate returns new initialized instance of annotate sub command
func NewCmdAnnotate(f util.Factory, parentCommand string, ioStreams genericiooptions.IOStreams) *cobra.Command {
	cmd := kubectlannotate.NewCmdAnnotate(parentCommand, f, ioStreams)
	cmd.Example = fmt.Sprintf(annotateExample, parentCommand)
	cmd.Annotations = map[string]string{
		util.TagCommandGroup: util.GroupSettingsCommands,
	}
	options.AddKubeConfigFlags(cmd.Flags())
	options.AddNamespaceFlag(cmd.Flags())
	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	return cmd
}
