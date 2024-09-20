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

package patch

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectlpatch "k8s.io/kubectl/pkg/cmd/patch"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	patchExample = templates.Examples(`
		# Partially update a deployment using a strategic merge patch, specifying the patch as JSON
		[1]%s patch deployment nginx-deployment -p '{"spec":{"replicas":2}}'

		# Partially update a deployment using a strategic merge patch, specifying the patch as YAML
		[1]%s patch deployment nginx-deployment -p $'spec:\n replicas: 2'

		# Partially update a deployment identified by the type and name specified in "deployment.json" using strategic merge patch
		[1]%s patch -f deployment.json -p '{"spec":{"replicas":2}}'

		# Update a propagationpolicy's conflictResolution using a JSON patch with positional arrays
		[1]%s patch pp nginx-propagation --type='json' -p='[{"op": "replace", "path": "/spec/conflictResolution", "value":"Overwrite"}]'

		# Update a deployment's replicas through the 'scale' subresource using a merge patch
		[1]%s patch deployment nginx-deployment --subresource='scale' --type='merge' -p '{"spec":{"replicas":2}}'`)
)

// NewCmdPatch returns new initialized instance of patch sub command
func NewCmdPatch(f util.Factory, parentCommand string, ioStreams genericiooptions.IOStreams) *cobra.Command {
	cmd := kubectlpatch.NewCmdPatch(f, ioStreams)
	cmd.Example = fmt.Sprintf(patchExample, parentCommand)
	cmd.Annotations = map[string]string{
		util.TagCommandGroup: util.GroupAdvancedCommands,
	}
	options.AddKubeConfigFlags(cmd.Flags())
	options.AddNamespaceFlag(cmd.Flags())

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	return cmd
}
