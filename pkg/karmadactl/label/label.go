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

package label

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectllabel "k8s.io/kubectl/pkg/cmd/label"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	labelExample = templates.Examples(`
		# Update deployment 'foo' with the label 'resourcetemplate.karmada.io/deletion-protected' and the value 'Always'
		[1]%s label deployment foo resourcetemplate.karmada.io/deletion-protected=Always

		# Update deployment 'foo' with the label 'resourcetemplate.karmada.io/deletion-protected' and the value '', overwriting any existing value
		[1]%s label --overwrite deployment foo resourcetemplate.karmada.io/deletion-protected=

		# Update all deployment in the namespace
		[1]%s label pp --all resourcetemplate.karmada.io/deletion-protected=

		# Update a deployment identified by the type and name in "deployment.json"
		[1]%s label -f deployment.json resourcetemplate.karmada.io/deletion-protected=

		# Update deployment 'foo' only if the resource is unchanged from version 1
		[1]%s label deployment resourcetemplate.karmada.io/deletion-protected=Always --resource-version=1

		# Update deployment 'foo' by removing a label named 'bar' if it exists
		# Does not require the --overwrite flag
		[1]%s label deployment foo resourcetemplate.karmada.io/deletion-protected-`)
)

// NewCmdLabel returns new initialized instance of label sub command
func NewCmdLabel(f util.Factory, parentCommand string, ioStreams genericiooptions.IOStreams) *cobra.Command {
	cmd := kubectllabel.NewCmdLabel(f, ioStreams)
	cmd.Example = fmt.Sprintf(labelExample, parentCommand)
	cmd.Annotations = map[string]string{
		util.TagCommandGroup: util.GroupSettingsCommands,
	}
	options.AddKubeConfigFlags(cmd.Flags())
	options.AddNamespaceFlag(cmd.Flags())

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	return cmd
}
